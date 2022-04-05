package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

const (
	GameServerName      = "GameServerName"
	GameServerNamespace = "GameServerNamespace"
	defaultTimeout      = 4
	LabelNodeName       = "NodeName"
	ErrStateNotExists   = "state does not exist"
	ErrHealthNotExists  = "health does not exist"
)

var (
	GameServerStates = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "thundernetes",
		Name: "game_server_states",
		Help: "Game server states",
	}, []string{"name", "state"})

	ConnectedPlayersGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "thundernetes",
		Name: "connected_players",
		Help: "Number of connected players per GameServer",
	}, []string{"namespace", "name"})
)

// NodeAgentManager manages the GameServer CRs that reside on this Node
// these game server process heartbeat to the NodeAgent process
// There is a two way communication between the game server and the NodeAgent
// The game server process tells the NodeAgent about its state (if it's Initializing or StandingBy)
// and NodeAgent tells the game server if it has been allocated (its state having been converted to Active)
type NodeAgentManager struct {
	gameServerMap     *sync.Map // we use a sync map instead of a regular map since this will be updated by multiple goroutines
	dynamicClient     dynamic.Interface
	watchStopper      chan struct{}
	nodeName          string
	logEveryHeartbeat bool
}

func NewNodeAgentManager(dynamicClient dynamic.Interface, nodeName string, logEveryHeartbeat bool) *NodeAgentManager {
	n := &NodeAgentManager{
		dynamicClient:     dynamicClient,
		watchStopper:      make(chan struct{}),
		gameServerMap:     &sync.Map{},
		nodeName:          nodeName,
		logEveryHeartbeat: logEveryHeartbeat,
	}
	n.startWatch()
	return n
}

// startWatch starts a watch on the GameServer CRs that reside on this Node
func (n *NodeAgentManager) startWatch() {
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

	// check if the details for this GameServer exist in the map
	gsdi, exists := n.gameServerMap.Load(gameServerName)

	if !exists {
		// details for this GameServer do not exist in the map
		// this means that either the GameServer was just created
		// or that the NodeAgent crashed and we're having a new instance
		// in any case, we're adding the details to the map
		logger.Infof("GameServer %s/%s does not exist in map, we're creating it", gameServerNamespace, gameServerName)
		gsdi = &GameServerDetails{
			GameServerNamespace: gameServerNamespace,
			Mutex:               &sync.RWMutex{},
			// we're not adding details about health/state since the NodeAgent might have crashed
			// and the health/state might have changed during the crash
		}
		n.gameServerMap.Store(gameServerName, gsdi)
	}

	gameServerState, _, err := n.parseStateHealth(obj)
	if err != nil {
		logger.Warnf("parsing state/health: %s. This is probably OK if the GameServer was just created", err.Error())
	}

	// we only care to continue if the state is Active
	if gameServerState != string(GameStateActive) {
		return
	}

	// server is Active, so get session details as well initial players details
	sessionID, sessionCookie := n.parseSessionDetails(obj, gameServerName, gameServerNamespace)
	logger.Infof("getting values from allocation - GameServer CR, sessionID:%s, sessionCookie:%s", sessionID, sessionCookie)
	initialPlayers := n.getInitialPlayers(ctx, gameServerName, gameServerNamespace)
	logger.Infof("getting values from allocation GameServerDetail CR, initialPlayers:%#v", initialPlayers)

	// get a reference to the GameServerDetails instance for this GameServer
	gsd := gsdi.(*GameServerDetails)

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

	gsd := gsdi.(*GameServerDetails)
	logger := getLogger(gameServerName, gsd.GameServerNamespace)

	if n.logEveryHeartbeat {
		heartbeatString := fmt.Sprintf("%v", hb) // from CodeQL analysis: If unsanitized user input is written to a log entry, a malicious user may be able to forge new log entries.
		heartbeatString = sanitize(heartbeatString)
		logger.Infof("heartbeat received from sessionHostId %s, data %s", gameServerName, heartbeatString)
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
func (n *NodeAgentManager) updateHealthAndStateIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerDetails) error {
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

		// the reason we're using unstructured to serialize the GameServerStatus instead of the entire GameServer object
		// is that we don't want extra fields (.Spec, .ObjectMeta) to be serialized
		u := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": mpsv1alpha1.GameServerStatus{
					State:  mpsv1alpha1.GameServerState(hb.CurrentGameState),
					Health: mpsv1alpha1.GameServerHealth(hb.CurrentGameHealth),
				},
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

// getInitialPlayers returns the initial players from the unstructured GameServerDetail CR
func (n *NodeAgentManager) getInitialPlayers(ctx context.Context, gameServerName, gameServerNamespace string) []string {
	logger := getLogger(gameServerName, gameServerNamespace)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*defaultTimeout)
	defer cancel()
	obj, err := n.dynamicClient.Resource(gameserverDetailGVR).Namespace(gameServerNamespace).Get(ctxWithTimeout, gameServerName, metav1.GetOptions{})
	if err != nil {
		logger.Warnf("error getting initial players details %s", err.Error())
		return []string{}
	}

	initialPlayers, initialPlayersExist, err := unstructured.NestedStringSlice(obj.Object, "spec", "initialPlayers")
	if err != nil {
		logger.Warnf("error getting initial players %s", err.Error())
		return []string{}
	}
	if !initialPlayersExist {
		logger.Warnf("initial players does not exist")
		return []string{}
	}

	return initialPlayers
}

// updateConnectedPlayersIfNeeded updates the connected players of the GameServerDetail CR if it has changed
func (n *NodeAgentManager) updateConnectedPlayersIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerDetails) error {
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
		return err
	}

	// storing the current number in memory
	gsd.Mutex.Lock()
	defer gsd.Mutex.Unlock()
	gsd.ConnectedPlayersCount = connectedPlayersCount

	return nil
}

// parseSessionDetails returns the sessionID and sessionCookie from the unstructured GameServer CR
func (n *NodeAgentManager) parseSessionDetails(u *unstructured.Unstructured, gameServerName, gameServerNamespace string) (string, string) {
	logger := getLogger(gameServerName, gameServerNamespace)
	sessionID, sessionIDExists, sessionIDErr := unstructured.NestedString(u.Object, "status", "sessionID")
	sessionCookie, sessionCookieExists, SessionCookieErr := unstructured.NestedString(u.Object, "status", "sessionCookie")

	if !sessionIDExists || !sessionCookieExists {
		logger.Debugf("sessionID or sessionCookie do not exist, sessionIDExists: %t, sessionCookieExists: %t", sessionIDExists, sessionCookieExists)
	}

	if sessionIDErr != nil {
		logger.Debugf("error getting sessionID: %s", sessionIDErr.Error())
	}

	if SessionCookieErr != nil {
		logger.Debugf("error getting sessionCookie: %s", SessionCookieErr.Error())
	}

	return sessionID, sessionCookie
}

// parseState parses the GameServer state from the unstructured GameServer CR
func (n *NodeAgentManager) parseStateHealth(u *unstructured.Unstructured) (string, string, error) {
	state, stateExists, stateErr := unstructured.NestedString(u.Object, "status", "state")
	health, healthExists, healthErr := unstructured.NestedString(u.Object, "status", "health")

	if stateErr != nil {
		return "", "", stateErr
	}
	if !stateExists {
		return "", "", errors.New(ErrStateNotExists)
	}

	if healthErr != nil {
		return "", "", stateErr
	}
	if !healthExists {
		return "", "", errors.New(ErrHealthNotExists)
	}
	return state, health, nil
}
