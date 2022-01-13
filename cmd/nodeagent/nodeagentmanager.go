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

var (
	logEveryHeartbeat = false
)

const (
	GameServerName      = "GameServerName"
	GameServerNamespace = "GameServerNamespace"
	timeout             = 4
	LabelNodeName       = "NodeName"
)

var (
	GameServerStates = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "game_server_states",
		Help: "Game server states",
	}, []string{"name", "state"})

	ConnectedPlayersGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "connected_players",
		Help: "Number of connected players per GameServer",
	}, []string{"namespace", "name"})
)

type NodeAgentManager struct {
	gameServerMap *sync.Map // we use a sync map instead of a regular map since this will be updated by multiple goroutines
	dynamicClient dynamic.Interface
	watchStopper  chan struct{}
	nodeName      string
}

func NewNodeAgentManager(dynamicClient dynamic.Interface, nodeName string) *NodeAgentManager {
	n := &NodeAgentManager{
		dynamicClient: dynamicClient,
		watchStopper:  make(chan struct{}),
		gameServerMap: &sync.Map{},
		nodeName:      nodeName,
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
	gameServerName := obj.GetName()
	gameServerNamespace := obj.GetNamespace()

	logger := getLogger(gameServerName, gameServerNamespace)

	// we should check if the object already has its .status set with state/health
	// this can happen if NodeAgent crashes and starts again
	// in this case, only the Created event will trigger
	state, health, err := n.parseStateHealth(obj)
	if err != nil {
		logger.Warnf("parsing state/health: %s", err.Error())
	}

	var sessionID, sessionCookie string
	var wasActivated bool = false

	if state == string(GameStateActive) {
		wasActivated = true
		//if the saved state is active, this means that NodeAgent crashed and restarted while the GameServer was active
		sessionID, sessionCookie = n.parseSessionDetails(obj, gameServerName, gameServerNamespace)
		// we don't do current players since they will be parsed/updated in a subsequent heartbeat
	}

	n.gameServerMap.Store(gameServerName, &GameServerDetails{
		GameServerNamespace: gameServerNamespace,
		Mutex:               &sync.RWMutex{},
		PreviousGameState:   GameState(state),
		PreviousGameHealth:  health,
		SessionID:           sessionID,
		SessionCookie:       sessionCookie,
		WasActivated:        wasActivated,
	})

	logger.Infof("GameServer %s/%s created", gameServerNamespace, gameServerName)
}

// gameServerUpdated is called when a GameServer CR is updated
func (n *NodeAgentManager) gameServerUpdated(oldObj, newObj interface{}) {
	ctx := context.Background()

	old := oldObj.(*unstructured.Unstructured)
	new := newObj.(*unstructured.Unstructured)

	gameServerName := old.GetName()
	gameServerNamespace := old.GetNamespace()

	logger := getLogger(gameServerName, gameServerNamespace)

	oldState, oldHealth, oldErr := n.parseStateHealth(old)
	if oldErr != nil {
		logger.Warnf("error parsing old state/health: %s", oldErr.Error())
		return
	}

	newState, newHealth, newErr := n.parseStateHealth(new)
	if newErr != nil {
		logger.Warnf("error parsing new state: %s", newErr.Error())
		return
	}

	logger.Infof("GameServer CR updated, name: %s, old state:%s, new state:%s, old health:%s, new health: %s", old.GetName(), oldState, newState, oldHealth, newHealth)

	gsdi, exists := n.gameServerMap.Load(gameServerName)

	if !exists {
		logger.Errorf("GameServer %s/%s does not exist in map", gameServerNamespace, gameServerName)
		return
	}

	gsd := gsdi.(*GameServerDetails)

	GameServerStates.WithLabelValues(gameServerName, newState).Set(1)
	if gsd.PreviousGameState != "" {
		GameServerStates.WithLabelValues(gameServerName, string(gsd.PreviousGameState)).Set(0)
	}

	// we're only interested if the game server was allocated
	if gsd.PreviousGameState == GameStateStandingBy && newState == string(GameStateActive) {
		sessionID, sessionCookie := n.parseSessionDetails(new, gameServerName, gameServerNamespace)
		logger.Infof("setting values from allocation - GameServer CR, sessionID:%s, sessionCookie:%s", sessionID, sessionCookie)

		initialPlayers := n.getInitialPlayers(ctx, gameServerName, gameServerNamespace)
		logger.Infof("setting values from allocation GameServerDetail CR, initialPlayers:%#v", initialPlayers)

		gsd.Mutex.Lock()
		defer gsd.Mutex.Unlock()
		// we don't modify the state (should be StandingBy)
		// we mark it as allocated plus add session details
		gsd.WasActivated = true
		gsd.SessionCookie = sessionCookie
		gsd.SessionID = sessionID
		gsd.InitialPlayers = initialPlayers
	}
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

	n.gameServerMap.Delete(gameServerName)
}

// heartbeatHandler is the http handler handling heartbeats from the GameServer Pods running on this Node
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

	if err := validateHeartbeatRequestArgs(&hb); err != nil {
		badRequest(w, err, "invalid heartbeat request")
		return
	}

	gsdi, exists := n.gameServerMap.Load(gameServerName)
	if !exists {
		// this will probably happen when the GameServer CR is deleted. Pod will probably send some heartbeats before its deleted
		internalServerError(w, fmt.Errorf("game server %s not found", gameServerName), "gameserver not found")
		return
	}

	gsd := gsdi.(*GameServerDetails)
	logger := getLogger(gameServerName, gsd.GameServerNamespace)

	if logEveryHeartbeat {
		heartbeatString := fmt.Sprintf("%v", hb) // from CodeQL analysis: If unsanitized user input is written to a log entry, a malicious user may be able to forge new log entries.
		heartbeatString = sanitize(heartbeatString)
		logger.Infof("heartbeat received from sessionHostId %s, data %s", gameServerName, heartbeatString)
	}

	if err := n.updateHealthAndStateIfNeeded(ctx, &hb, gameServerName, gsd); err != nil {
		logger.Errorf("error updating health %s", err.Error())
		internalServerError(w, err, "error updating health")
		return
	}

	if err := n.updateConnectedPlayersIfNeeded(ctx, &hb, gameServerName, gsd); err != nil {
		logger.Errorf("error updating connected players count %s", err.Error())
		internalServerError(w, err, "error updating connected players count")
		return
	}

	// read the previous game state
	// in most cases we're gonna return continue
	gsd.Mutex.RLock()
	heartbeatResponseState := gsd.PreviousGameState
	wasActivated := gsd.WasActivated
	gsd.Mutex.RUnlock()

	// game server was just allocated, so we have to signal this to the game server (return GameStateActive instead of GameStateContinue)
	if heartbeatResponseState == GameStateStandingBy && wasActivated {
		gsd.Mutex.Lock()
		gsd.PreviousGameState = GameStateActive
		heartbeatResponseState = GameStateActive
		gsd.Mutex.Unlock()
	}

	sc := &SessionConfig{
		SessionId:      gsd.SessionID,
		SessionCookie:  gsd.SessionCookie,
		InitialPlayers: gsd.InitialPlayers,
	}

	hr := &HeartbeatResponse{
		Operation:     n.getOperation(heartbeatResponseState),
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

	ok := isValidStateTransition(gsd.PreviousGameState, hb.CurrentGameState)
	if !ok {
		return fmt.Errorf("invalid state transition from %s to %s", gsd.PreviousGameState, hb.CurrentGameState)
	}

	logger.Debugf("Health or state is different than before, updating. Old health %s, new health %s, old state %s, new state %s", sanitize(gsd.PreviousGameHealth), sanitize(hb.CurrentGameHealth), sanitize(string(gsd.PreviousGameState)), sanitize(string(hb.CurrentGameState)))

	// the reason we're using unstructured to serialize the GameServerStatus is that we don't want extra fields (.Spec, .ObjectMeta) to be serialized
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

	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*timeout)
	defer cancel()
	_, err = n.dynamicClient.Resource(gameserverGVR).Namespace(gsd.GameServerNamespace).Patch(ctxWithTimeout, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return err
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
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*timeout)
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
	logger.Infof("ConnectedPlayersCount is different than before, updating. Old connectedPlayersCount %d, new connectedPlayersCount %d", gsd.ConnectedPlayersCount, len(hb.CurrentPlayers))

	gsdPatchSpec := mpsv1alpha1.GameServerDetailSpec{}
	if connectedPlayersCount == 0 {
		gsdPatchSpec.ConnectedPlayersCount = 0
		gsdPatchSpec.ConnectedPlayers = make([]string, 0)
	} else {
		gsdPatchSpec.ConnectedPlayersCount = connectedPlayersCount
		gsdPatchSpec.ConnectedPlayers = currentPlayerIDs
	}

	// the reason we're using unstructured to serialize the GameServerDetailSpec is that we don't want extra fields (.Status, .ObjectMeta) to be serialized
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

	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*timeout)
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
		logger.Debugf("sessionID or sessionCookie do not exist, sessionIDExists:%t, sessionCookieExists:%t", sessionIDExists, sessionCookieExists)
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
		return "", "", errors.New("state does not exist")
	}

	if healthErr != nil {
		return "", "", stateErr
	}
	if !healthExists {
		return "", "", errors.New("health does not exist")
	}
	return state, health, nil
}

// getOperation returns the operation for the heartbeat response
func (n *NodeAgentManager) getOperation(heartbeatResponseState GameState) GameOperation {
	var op GameOperation = GameOperationContinue
	if heartbeatResponseState == GameStateInvalid { // status has not been set yet
		op = GameOperationContinue
	} else if heartbeatResponseState == GameStateInitializing {
		op = GameOperationContinue
	} else if heartbeatResponseState == GameStateStandingBy {
		op = GameOperationContinue
	} else if heartbeatResponseState == GameStateActive {
		op = GameOperationActive
	} else if heartbeatResponseState == GameStateTerminated || heartbeatResponseState == GameStateTerminating {
		op = GameOperationTerminate
	}
	return op
}
