package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

const (
	GameServerName        = "GameServerName"
	GameServerNamespace   = "GameServerNamespace"
	defaultTimeout        = 4
	LabelNodeName         = "NodeName"
	ErrBuildNameNotExists = "build name does not exist"
	ErrStateNotExists     = "state does not exist"
	ErrHealthNotExists    = "health does not exist"
	unhealthyStatus       = "Unhealthy"
	healthyStatus         = "Healthy"
)

// HeartbeatState is a status of a gameserver, it represents if it has sent heartbeats
type HeartbeatState int

const (
	gotHeartbeat     HeartbeatState = iota // it has sent a hearbeat in the corresponding time window
	noHeartbeatEver                        // it has never sent a heartbeat
	noHeartbeatSince                       // it hasn't sent a heartbeat in the corresponding time window
)

// NodeAgentManager manages the GameServer CRs that reside on this Node
// these game server process heartbeat to the NodeAgent process
// There is a two way communication between the game server and the NodeAgent
// The game server process tells the NodeAgent about its state (if it's Initializing or StandingBy)
// and NodeAgent tells the game server if it has been allocated (its state having been converted to Active)
type NodeAgentManager struct {
	gameServerMap             *sync.Map // we use a sync map instead of a regular map since this will be updated by multiple goroutines
	dynamicClient             dynamic.Interface
	watchStopper              chan struct{}
	nodeName                  string
	logEveryHeartbeat         bool
	ignoreHealthFromHeartbeat bool // treat every heartbeat's health state as Healthy. This is to handle GSDK clients that may send invalid heartbeat
	nowFunc                   func() time.Time
	heartbeatTimeout          int64 // timeouts for not receiving a heartbeat in milliseconds
	firstHeartbeatTimeout     int64 // the first heartbeat gets a longer window considering initialization time
}

func NewNodeAgentManager(dynamicClient dynamic.Interface, nodeName string, logEveryHeartbeat bool, ignoreHealthFromHeartbeat bool, now func() time.Time) *NodeAgentManager {
	n := &NodeAgentManager{
		dynamicClient:             dynamicClient,
		watchStopper:              make(chan struct{}),
		gameServerMap:             &sync.Map{},
		nodeName:                  nodeName,
		logEveryHeartbeat:         logEveryHeartbeat,
		ignoreHealthFromHeartbeat: ignoreHealthFromHeartbeat,
		nowFunc:                   now,
	}
	n.runWatch()
	n.runHeartbeatTimeCheckerLoop()
	return n
}

// runWatch starts a watch on the GameServer CRs that reside on this Node
func (n *NodeAgentManager) runWatch() {
	// we watch for GameServers which Pods have been scheduled to the same Node as this NodeAgent DaemonSet Pod
	listOptions := dynamicinformer.TweakListOptionsFunc(func(options *metav1.ListOptions) {
		options.LabelSelector = fmt.Sprintf("%s=%s", LabelNodeName, n.nodeName)
	})

	dynInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(n.dynamicClient, 0, v1.NamespaceAll, listOptions)
	informer := dynInformer.ForResource(gameserverGVR).Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    n.gameServerCreated,
		UpdateFunc: n.gameServerUpdated,
		DeleteFunc: n.gameServerDeleted,
	})

	go informer.Run(n.watchStopper)
}

// runHeartbeatTimeCheckerLoop runs HeartbeatTimeChecker on an infinite loop
func (n *NodeAgentManager) runHeartbeatTimeCheckerLoop() {
	n.firstHeartbeatTimeout = ParseInt64FromEnv("FIRST_HEARTBEAT_TIMEOUT", 60000)
	n.heartbeatTimeout = ParseInt64FromEnv("HEARTBEAT_TIMEOUT", 5000)
	go func() {
		for {
			n.HeartbeatTimeChecker()
			time.Sleep(time.Duration(n.heartbeatTimeout) * time.Millisecond)
		}
	}()
}

// HeartbeatTimeChecker checks that heartbeats are still being sent for each GameServerInfo
// in the local gameServerMap, if not it will send a patch to mark those GameServers as unhealthy,
// it follows these two rules:
// 1. if the server hasn't sent its first heartbeat, it has FirstHeartbeatTimeout
//    milliseconds since its creation before being marked as unhealthy
// 2. if the server has sent its first heartbeat, it has HeartbeatTimeout milliseconds
//    since its last heartbeat before being marked as unhealthy
func (n *NodeAgentManager) HeartbeatTimeChecker() {
	n.gameServerMap.Range(func(key interface{}, value interface{}) bool {
		currentTime := n.nowFunc().UnixMilli()
		gsi := value.(*GameServerInfo)
		state := gotHeartbeat
		gsi.Mutex.RLock()
		gameServerName := key.(string)
		gameServerNamespace := gsi.GameServerNamespace
		if gsi.LastHeartbeatTime == 0 && gsi.CreationTime != 0 &&
			(currentTime-gsi.CreationTime) > n.firstHeartbeatTimeout &&
			gsi.PreviousGameHealth != unhealthyStatus {
			state = noHeartbeatEver
		} else if gsi.LastHeartbeatTime != 0 &&
			(currentTime-gsi.LastHeartbeatTime) > n.heartbeatTimeout &&
			gsi.PreviousGameHealth != unhealthyStatus {
			state = noHeartbeatSince
		}
		markedUnhealthy := gsi.MarkedUnhealthy
		gsi.Mutex.RUnlock()
		// the first part of this if is to avoid sending a patch more than once
		if !markedUnhealthy && state != gotHeartbeat {
			err := n.markGameServerUnhealthy(gameServerName, gameServerNamespace, state)
			if err == nil {
				gsi.Mutex.Lock()
				gsi.MarkedUnhealthy = true
				gsi.Mutex.Unlock()
			}
		}
		return true
	})
}

// markGameServerUnhealthy sends a patch to mark the GameServer, described by its name
// and namespace, as Unhealthy
func (n *NodeAgentManager) markGameServerUnhealthy(gameServerName, gameServerNamespace string, state HeartbeatState) error {
	logger := getLogger(gameServerName, gameServerNamespace)
	if state == noHeartbeatEver {
		logger.Infof("GameServer has not sent any heartbeats in %d seconds since creation, marking Unhealthy", n.firstHeartbeatTimeout/1000)
	} else if state == noHeartbeatSince {
		logger.Infof("GameServer has not sent any heartbeats in %d seconds since last, marking Unhealthy", n.heartbeatTimeout/1000)
	}
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": mpsv1alpha1.GameServerStatus{
				Health: mpsv1alpha1.GameServerHealth(unhealthyStatus),
			},
		},
	}
	// this will be marshaled as payload := fmt.Sprintf("{\"status\":{\"health\":\"%s\"}}", "Unhealthy")
	payloadBytes, err := json.Marshal(u)
	if err != nil {
		logger.Errorf("marshaling %s", err.Error())
		return err
	}
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), time.Second*defaultTimeout)
	defer cancel()
	_, err = n.dynamicClient.Resource(gameserverGVR).Namespace(gameServerNamespace).Patch(ctxWithTimeout, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		logger.Errorf("updating health %s", err.Error())
		return err
	}
	return nil
}

// gameServerCreated is called when a GameServer CR is created
func (n *NodeAgentManager) gameServerCreated(objUnstructured interface{}) {
	obj := objUnstructured.(*unstructured.Unstructured)
	if obj == nil {
		return
	}
	n.gameServerCreatedOrUpdated(obj)
}

// gameServerUpdated is called when a GameServer CR is updated
func (n *NodeAgentManager) gameServerUpdated(_, newObj interface{}) {
	obj := newObj.(*unstructured.Unstructured)
	if obj == nil {
		return
	}
	n.gameServerCreatedOrUpdated(obj)
}

// gameServerCreatedOrUpdated is called when a GameServer CR is created or updated
func (n *NodeAgentManager) gameServerCreatedOrUpdated(obj *unstructured.Unstructured) {
	gameServerName := obj.GetName()
	gameServerNamespace := obj.GetNamespace()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout*time.Second)
	defer cancel()

	logger := getLogger(gameServerName, gameServerNamespace)

	gameServerBuildName, err := parseBuildName(obj)
	if err != nil {
		logger.Errorf("parsing buildID: %s", err.Error())
	}

	// check if the details for this GameServer exist in the map
	gsdi, exists := n.gameServerMap.Load(gameServerName)

	if !exists {
		// details for this GameServer do not exist in the map
		// this means that either the GameServer was just created
		// or that the NodeAgent crashed and we're having a new instance
		// in any case, we're adding the details to the map
		logger.Infof("GameServer %s/%s does not exist in cache, we're creating it", gameServerNamespace, gameServerName)
		gsdi = &GameServerInfo{
			GameServerNamespace: gameServerNamespace,
			Mutex:               &sync.RWMutex{},
			GsUid:               obj.GetUID(),
			CreationTime:        n.nowFunc().UnixMilli(),
			BuildName:           gameServerBuildName,
			MarkedUnhealthy:     false,
			// we're not adding details about health/state since the NodeAgent might have crashed
			// and the health/state might have changed during the crash
		}
		n.gameServerMap.Store(gameServerName, gsdi)
	}

	gameServerState, gameServerHealth, err := parseStateHealth(obj)
	if err != nil {
		if err.Error() == ErrHealthNotExists || err.Error() == ErrStateNotExists {
			logger.Debugf("parsing state/health: %s. This is OK since the server was probably just created", err.Error())
		} else {
			logger.Errorf("parsing state/health: %s", err.Error())
		}

	}

	// we only care to continue if the state is Active and the GameServer is healthy
	if gameServerState != string(GameStateActive) || gameServerHealth != string(mpsv1alpha1.GameServerHealthy) {
		logger.Debugf("skipping create/update handler since GameServer %s/%s has state %s and health %s", gameServerNamespace, gameServerName, gameServerState, gameServerHealth)
		return
	}

	// server is Active, so get session details as well initial players details
	sessionID, sessionCookie, initialPlayers := parseSessionDetails(obj, gameServerName, gameServerNamespace)
	// sessionCookie:<valueOfCookie> string is looked for in the e2e tests, be careful not to modify it!
	logger.Infof("getting values from allocation - GameServer CR, sessionID:%s, sessionCookie:%s, initialPlayers: %v", sessionID, sessionCookie, initialPlayers)

	// create the GameServerDetails CR
	err = n.createGameServerDetails(ctx, obj.GetUID(), gameServerName, gameServerNamespace, gameServerBuildName, nil)
	if err != nil {
		logger.Errorf("error creating GameServerDetails: %s", err.Error())
	}

	// get a reference to the GameServerDetails instance for this GameServer
	gsd := gsdi.(*GameServerInfo)

	// we are setting the current state to 1 and the previous to zero (if exists)
	// in this way, we can add all the "1"s together to get a total number of GameServers in a specific state
	GameServerStates.WithLabelValues(gameServerName, gameServerState).Set(1)
	if gsd.PreviousGameState != "" {
		GameServerStates.WithLabelValues(gameServerName, string(gsd.PreviousGameState)).Set(0)
	}

	gsd.Mutex.Lock()
	defer gsd.Mutex.Unlock()
	// we mark the server as allocated plus add session details
	// we're locking the mutex so the heartbeat handler method won't read this data at the same time
	gsd.IsActive = true
	gsd.SessionCookie = sessionCookie
	gsd.SessionID = sessionID
	gsd.InitialPlayers = initialPlayers
}

// gameServerDeleted is called when a GameServer CR is deleted
func (n *NodeAgentManager) gameServerDeleted(objUnstructured interface{}) {
	obj := objUnstructured.(*unstructured.Unstructured)
	gameServerName := obj.GetName()
	gameServerNamespace := obj.GetNamespace()

	log.WithFields(log.Fields{
		GameServerName:      gameServerName,
		GameServerNamespace: gameServerNamespace,
	}).Infof("GameServer %s/%s deleted", gameServerNamespace, gameServerName)

	// Delete is a no-op if the GameServer is not in the map
	n.gameServerMap.Delete(gameServerName)
}

// heartbeatHandler is the http handler handling heartbeats from the GameServer Pods running on this Node
// it responds by sending intructions/signal for the next operation
// on Thundernetes, the only operation that NodeAgent can signal to the GameServer is that the GameServer has been allocated (its state has transitioned to Active)
// when it's allocated, it will return an "Active" operation
// in all other cases, it will return "Continue" (which basically means continue doing what you are already doing)
func (n *NodeAgentManager) heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	re := regexp.MustCompile(`.*/v1/sessionHosts\/(.*?)(/heartbeats|$)`)
	match := re.FindStringSubmatch(r.RequestURI)

	gameServerName := match[1]

	var hb HeartbeatRequest
	err := json.NewDecoder(r.Body).Decode(&hb)
	if err != nil {
		badRequest(w, err, "cannot deserialize json")
		return
	}

	if err := validateHeartbeatRequest(&hb); err != nil {
		badRequest(w, err, "invalid heartbeat request")
		return
	}

	gsdi, exists := n.gameServerMap.Load(gameServerName)
	if !exists {
		// this will probably happen when the GameServer CR is deleted. Pod may send some heartbeats before it's deleted
		internalServerError(w, fmt.Errorf("game server %s not found", gameServerName), "gameserver not found")
		return
	}

	gsd := gsdi.(*GameServerInfo)
	logger := getLogger(gameServerName, gsd.GameServerNamespace)

	gsd.Mutex.Lock()
	gsd.LastHeartbeatTime = n.nowFunc().UnixMilli()
	gsd.Mutex.Unlock()

	if n.logEveryHeartbeat {
		heartbeatString := fmt.Sprintf("%v", hb) // from CodeQL analysis: If unsanitized user input is written to a log entry, a malicious user may be able to forge new log entries.
		heartbeatString = sanitize(heartbeatString)
		logger.Infof("heartbeat received from sessionHostId %s, data %s", gameServerName, heartbeatString)
	}

	if n.ignoreHealthFromHeartbeat {
		hb.CurrentGameHealth = string(mpsv1alpha1.GameServerHealthy)
	}

	if err := n.updateHealthAndStateIfNeeded(ctx, &hb, gameServerName, gsd); err != nil {
		logger.Errorf("updating health/state %s", err.Error())
		internalServerError(w, err, "error updating health/state")
		return
	}

	if err := n.updateConnectedPlayersIfNeeded(ctx, &hb, gameServerName, gsd); err != nil {
		logger.Errorf("updating connected players count %s", err.Error())
		internalServerError(w, err, "error updating connected players count")
		return
	}

	gsd.Mutex.RLock()
	// check if the game server is active
	isActive := gsd.IsActive
	// get the session details (if any)
	sc := &SessionConfig{
		SessionId:      gsd.SessionID,
		SessionCookie:  gsd.SessionCookie,
		InitialPlayers: gsd.InitialPlayers,
	}
	gsd.Mutex.RUnlock()

	operation := GameOperationContinue
	// game server process internal state is StandingBy
	// but the GameServer CR state is Active, so the server was just allocated
	// in this case, we have to signal the transition to Active to the game server process
	if hb.CurrentGameState == GameStateStandingBy && isActive {
		logger.Debugf("GameServer %s is transitioning to Active", gameServerName)
		operation = GameOperationActive
	}

	// prepare the heartbeat response
	// this includes the current designated operation as well as any session configuration
	hr := &HeartbeatResponse{
		Operation:     operation,
		SessionConfig: *sc,
	}

	json, err := json.Marshal(hr)
	if err != nil {
		internalServerError(w, err, "error marshalling heartbeat response")
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(json)
}

// updateHealthAndStateIfNeeded updates both the health and state of the GameServer if any one of them has changed
func (n *NodeAgentManager) updateHealthAndStateIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerInfo) error {
	logger := getLogger(gameServerName, gsd.GameServerNamespace)

	// if neither state or health changed, we don't need to do anything
	if gsd.PreviousGameHealth == hb.CurrentGameHealth && gsd.PreviousGameState == hb.CurrentGameState {
		return nil
	}

	// the following can happen if the NodeAgent crashes while the game server is Active, so the cache will be empty
	// in this case, we should update the cache with the Active state
	// so that the update methods below can work appropriately
	if hb.CurrentGameState == GameStateActive && gsd.PreviousGameState == "" {
		gsd.Mutex.Lock()
		logger.Info("GameServer is Active and previous state is empty, NodeAgent probably crashed and restarted. Manually setting previous state to Active")
		gsd.PreviousGameState = GameStateActive
		gsd.Mutex.Unlock()
	}

	ok := isValidStateTransition(gsd.PreviousGameState, hb.CurrentGameState)
	if !ok {
		return fmt.Errorf("invalid state transition from %s to %s", gsd.PreviousGameState, hb.CurrentGameState)
	}

	// if the previous cached state is StandingBy and the current state is Active,
	// this means that the GameServer was allocated and we are in the process of handling the first heartbeat
	// in this case, there is no need to update the GameServer CR status with the new state
	// since it's already set to Active, otherwise the game server would not have been allocated
	if !(gsd.PreviousGameState == GameStateStandingBy && hb.CurrentGameState == GameStateActive && gsd.PreviousGameHealth == hb.CurrentGameHealth) {
		logger.Debugf("Health or state is different than before, updating. Old health: %s, new health: %s, old state: %s, new state: %s", sanitize(gsd.PreviousGameHealth), sanitize(hb.CurrentGameHealth), sanitize(string(gsd.PreviousGameState)), sanitize(string(hb.CurrentGameState)))

		status := mpsv1alpha1.GameServerStatus{
			State:  mpsv1alpha1.GameServerState(hb.CurrentGameState),
			Health: mpsv1alpha1.GameServerHealth(hb.CurrentGameHealth),
		}
		// if the GameServer is Healthy, update the timestamps of the corresponding state transition
		if hb.CurrentGameHealth == string(mpsv1alpha1.GameServerHealthy) {
			now := metav1.Time{Time: n.nowFunc()}
			if hb.CurrentGameState == GameStateInitializing {
				status.ReachedInitializingOn = &now
			} else if hb.CurrentGameState == GameStateStandingBy {
				status.ReachedStandingByOn = &now
			}
		}

		// the reason we're using unstructured to serialize the GameServerStatus instead of the entire GameServer object
		// is that we don't want extra fields (.Spec, .ObjectMeta) to be serialized
		u := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": status,
			},
		}

		// this will be marshaled as payload := fmt.Sprintf("{\"status\":{\"health\":\"%s\",\"state\":\"%s\"}}", hb.CurrentGameHealth, hb.CurrentGameState)
		payloadBytes, err := json.Marshal(u)
		if err != nil {
			return err
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*defaultTimeout)
		defer cancel()
		_, err = n.dynamicClient.Resource(gameserverGVR).Namespace(gsd.GameServerNamespace).Patch(ctxWithTimeout, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")
		if err != nil {
			return err
		}
	}

	gsd.Mutex.Lock()
	defer gsd.Mutex.Unlock()
	gsd.PreviousGameHealth = hb.CurrentGameHealth
	gsd.PreviousGameState = hb.CurrentGameState

	return nil
}

// updateConnectedPlayersIfNeeded updates the connected players of the GameServerDetail CR if it has changed
func (n *NodeAgentManager) updateConnectedPlayersIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerInfo) error {
	logger := getLogger(gameServerName, gsd.GameServerNamespace)
	// we're not interested in updating the connected players count if the game is not active or if the player population has not changed
	if hb.CurrentGameState != GameStateActive || gsd.ConnectedPlayersCount == len(hb.CurrentPlayers) {
		return nil
	}

	connectedPlayersCount := len(hb.CurrentPlayers)

	// set the prometheus gauge
	ConnectedPlayersGauge.WithLabelValues(gsd.GameServerNamespace, gameServerName).Set(float64(connectedPlayersCount))

	currentPlayerIDs := make([]string, connectedPlayersCount)
	for i := 0; i < len(hb.CurrentPlayers); i++ {
		currentPlayerIDs[i] = hb.CurrentPlayers[i].PlayerId
	}
	logger.Infof("ConnectedPlayersCount is different than before, updating. Old connectedPlayersCount: %d, new connectedPlayersCount: %d", gsd.ConnectedPlayersCount, len(hb.CurrentPlayers))

	gsdPatchSpec := mpsv1alpha1.GameServerDetailSpec{}
	if connectedPlayersCount == 0 {
		gsdPatchSpec.ConnectedPlayersCount = 0
		gsdPatchSpec.ConnectedPlayers = make([]string, 0)
	} else {
		gsdPatchSpec.ConnectedPlayersCount = connectedPlayersCount
		gsdPatchSpec.ConnectedPlayers = currentPlayerIDs
	}

	// the reason we're using unstructured to serialize the GameServerDetailSpec instead of the GameServerDetail object
	// is that we don't want extra fields (.Status, .ObjectMeta) to be serialized
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": gsdPatchSpec,
		},
	}

	// this will be marshaled as fmt.Sprintf("{\"spec\":{\"connectedPlayersCount\":%d,\"connectedPlayers\":[\"%s\"]}}", len(hb.CurrentPlayers), strings.Join(currentPlayerIDs, "\",\""))
	payloadBytes, err := json.Marshal(u)
	if err != nil {
		return err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*defaultTimeout)
	defer cancel()

	_, err = n.dynamicClient.Resource(gameserverDetailGVR).Namespace(gsd.GameServerNamespace).Patch(ctxWithTimeout, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// GameServerDetails CR not found, there was an error when it was created
			logger.Warnf("GameServerDetail CR not found, will create it")
			errCreate := n.createGameServerDetails(ctx, gsd.GsUid, gameServerName, gsd.GameServerNamespace, gsd.BuildName, currentPlayerIDs)
			if errCreate != nil {
				return errCreate
			}
			// at this point, we have successfully recovered and created the missing GameServerDetail CR
			// we will return the original error so the client knows that the update has failed
			// however, the code will try to update the connectedPlayers soon, on the next heartbeat
		}
		return err
	}

	// storing the current number in memory
	gsd.Mutex.Lock()
	defer gsd.Mutex.Unlock()
	gsd.ConnectedPlayersCount = connectedPlayersCount

	return nil
}

// createGameServerDetails creates a GameServerDetails CR with the specified name and namespace
func (n *NodeAgentManager) createGameServerDetails(ctx context.Context, gsuid types.UID, gsname, gsnamespace string, gsbuildname string, connectedPlayers []string) error {
	gs := &mpsv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gsname,
			Namespace: gsnamespace,
			UID:       gsuid,
		},
	}
	gsd := mpsv1alpha1.GameServerDetail{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gsname, // same name and namespace as the GameServer
			Namespace: gsnamespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(gs, schema.GroupVersionKind{
					Group:   gameserverGVR.Group,
					Version: gameserverGVR.Version,
					Kind:    "GameServer",
				}),
			},
			Labels: map[string]string{"BuildName": gsbuildname},
		},
		Spec: mpsv1alpha1.GameServerDetailSpec{
			ConnectedPlayers: connectedPlayers,
		},
	}

	// connectedPlayers only comes != nil when NodeAgent failed to create the GameServerDetails CR after allocation
	// and NodeAgent updates the GameServerDetails CR on the next heartbeat
	if connectedPlayers != nil {
		gsd.Spec.ConnectedPlayersCount = len(connectedPlayers)
	}

	metadata, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&gsd.ObjectMeta)
	if err != nil {
		return err
	}

	spec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&gsd.Spec)
	if err != nil {
		return err
	}

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": gameserverDetailGVR.GroupVersion().String(),
			"kind":       "GameServerDetail",
			"metadata":   metadata,
			"spec":       spec,
		},
	}

	_, err = n.dynamicClient.Resource(gameserverDetailGVR).Namespace(gsnamespace).Create(ctx, u, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
