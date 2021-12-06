package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	gameserverGVR = schema.GroupVersionResource{
		Group:    "mps.playfab.com",
		Version:  "v1alpha1",
		Resource: "gameservers",
	}

	gameserverDetailGVR = schema.GroupVersionResource{
		Group:    "mps.playfab.com",
		Version:  "v1alpha1",
		Resource: "gameserverdetails",
	}
	logEveryHeartbeat = false
)

const (
	GameServerName      = "GameServerName"
	GameServerNamespace = "GameServerNamespace"
	timeout             = 4
	LabelNodeName       = "NodeName"
)

type NodeAgentManager struct {
	gameServerMap *sync.Map // we use a sync map since this will be updated by multiple goroutines
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

func (n *NodeAgentManager) startWatch() {
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

func (n *NodeAgentManager) gameServerCreated(objUnstructured interface{}) {
	obj := objUnstructured.(*unstructured.Unstructured)
	gameServerName := obj.GetName()
	gameServerNamespace := obj.GetNamespace()

	n.gameServerMap.Store(gameServerName, &GameServerDetails{
		GameServerNamespace: gameServerNamespace,
		Mutex:               &sync.RWMutex{},
	})
}

func (n *NodeAgentManager) gameServerUpdated(oldObj, newObj interface{}) {
	ctx := context.Background()
	// dynamic client returns an unstructured object
	old := oldObj.(*unstructured.Unstructured)
	new := newObj.(*unstructured.Unstructured)

	gameServerName := old.GetName()
	gameServerNamespace := old.GetNamespace()

	logger := getLogger(gameServerName, gameServerNamespace)

	// get the old and the new state from .status.state
	oldState, oldStateExists, oldStateErr := unstructured.NestedString(old.Object, "status", "state")
	newState, newStateExists, newStateErr := unstructured.NestedString(new.Object, "status", "state")

	if oldStateErr != nil {
		logger.Errorf("error getting old state: %s", oldStateErr.Error())
		return
	}

	if newStateErr != nil {
		logger.Errorf("error getting new state: %s", newStateErr.Error())
		return
	}

	if !oldStateExists || !newStateExists {
		logger.Warnf("state does not exist, oldStateExists:%t, newStateExists:%t", oldStateExists, newStateExists)
		return
	}

	logger.Infof("GameServer CR updated %s:%s,%s,%s", old.GetName(), oldState, new.GetName(), newState)

	gsdi, exists := n.gameServerMap.Load(gameServerName)

	if !exists {
		logger.Errorf("GameServer %s/%s does not exist in map", gameServerNamespace, gameServerName)
		return
	}

	gsd := gsdi.(*GameServerDetails)

	if gsd.CurrentGameState == GameStateStandingBy && newState == string(GameStateActive) {
		sessionID, sessionCookie := n.parseSessionDetails(new, gameServerName, gameServerNamespace)
		logger.Infof("Got values from allocation, sessionID:%s, sessionCookie:%s", sessionID, sessionCookie)

		initialPlayers := n.getInitialPlayers(ctx, gameServerName, gameServerNamespace)
		logger.Infof("Got values from allocation, initialPlayers:%#v", initialPlayers)

		gsd.Mutex.Lock()
		defer gsd.Mutex.Unlock()
		gsd.CurrentGameState = GameStateActive
		gsd.SessionCookie = sessionCookie
		gsd.SessionID = sessionID
		gsd.InitialPlayers = initialPlayers
	}
}

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
		logger.Infof("heartbeat received from sessionHostId %s, data %#v", gameServerName, hb)
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

	var op GameOperation = GameOperationContinue

	gsd.Mutex.RLock()
	currentState := gsd.CurrentGameState
	gsd.Mutex.RUnlock()

	if currentState == GameStateInvalid { // user has not set the status yet
		op = GameOperationContinue
	} else if currentState == GameStateInitializing {
		op = GameOperationContinue
	} else if currentState == GameStateStandingBy {
		op = GameOperationContinue
	} else if currentState == GameStateActive {
		op = GameOperationActive
	} else if currentState == GameStateTerminated || currentState == GameStateTerminating {
		op = GameOperationTerminate
	}

	sc := &SessionConfig{
		SessionId:      gsd.SessionID,
		SessionCookie:  gsd.SessionCookie,
		InitialPlayers: gsd.InitialPlayers,
	}

	hr := &HeartbeatResponse{
		Operation:     op,
		SessionConfig: *sc,
	}

	json, _ := json.Marshal(hr)

	w.WriteHeader(http.StatusOK)
	w.Write(json)
}

// updateHealthAndStateIfNeeded updates both the health and state of the GameServer if any one of them has changed
func (n *NodeAgentManager) updateHealthAndStateIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerDetails) error {
	logger := getLogger(gameServerName, gsd.GameServerNamespace)

	if gsd.CurrentGameHealth != hb.CurrentGameHealth || gsd.CurrentGameState != hb.CurrentGameState {
		ok := isValidStateTransition(gsd.CurrentGameState, hb.CurrentGameState)
		if !ok {
			logger.Warnf("invalid state transition from %s to %s", gsd.CurrentGameState, hb.CurrentGameState)
			return nil
		}

		logger.Debugf("Health or state is different than before, updating. Old health %s, new health %s, old state %s, new state %s", gsd.CurrentGameHealth, hb.CurrentGameHealth, gsd.CurrentGameState, hb.CurrentGameState)
		payload := fmt.Sprintf("{\"status\":{\"health\":\"%s\",\"state\":\"%s\"}}", hb.CurrentGameHealth, hb.CurrentGameState)
		payloadBytes := []byte(payload)
		ctxWithTimeout, _ := context.WithTimeout(ctx, time.Second*timeout)
		_, err := n.dynamicClient.Resource(gameserverGVR).Namespace(gsd.GameServerNamespace).Patch(ctxWithTimeout, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")

		if err != nil {
			return err
		}
		gsd.Mutex.Lock()
		defer gsd.Mutex.Unlock()
		gsd.CurrentGameHealth = hb.CurrentGameHealth
		gsd.CurrentGameState = hb.CurrentGameState
	}
	return nil
}

// getInitialPlayers returns the initial players from the unstructured GameServerDetail CR
func (n *NodeAgentManager) getInitialPlayers(ctx context.Context, gameServerName, gameServerNamespace string) []string {
	logger := getLogger(gameServerName, gameServerNamespace)
	ctxWithTimeout, _ := context.WithTimeout(ctx, time.Second*timeout)
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

func (n *NodeAgentManager) updateConnectedPlayersIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerDetails) error {
	logger := getLogger(gameServerName, gsd.GameServerNamespace)
	// we're not interested in updating the connected players count if the game is not active
	if hb.CurrentGameState == GameStateActive && gsd.ConnectedPlayersCount != len(hb.CurrentPlayers) {
		currentPlayerIDs := make([]string, len(hb.CurrentPlayers))
		for i := 0; i < len(hb.CurrentPlayers); i++ {
			currentPlayerIDs[i] = hb.CurrentPlayers[i].PlayerId
		}
		logger.Infof("ConnectedPlayersCount is different than before, updating. Old connectedPlayersCount %d, new connectedPlayersCount %d", gsd.ConnectedPlayersCount, len(hb.CurrentPlayers))
		var payload string
		if len(hb.CurrentPlayers) == 0 {
			payload = "{\"spec\":{\"connectedPlayersCount\":0,\"connectedPlayers\":[]}}"
		} else {
			payload = fmt.Sprintf("{\"spec\":{\"connectedPlayersCount\":%d,\"connectedPlayers\":[\"%s\"]}}", len(hb.CurrentPlayers), strings.Join(currentPlayerIDs, "\",\""))
		}
		payloadBytes := []byte(payload)
		ctxWithTimeout, _ := context.WithTimeout(ctx, time.Second*timeout)
		_, err := n.dynamicClient.Resource(gameserverDetailGVR).Namespace(gsd.GameServerNamespace).Patch(ctxWithTimeout, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			return err
		}
		// storing the current number in memory
		gsd.Mutex.Lock()
		defer gsd.Mutex.Unlock()
		gsd.ConnectedPlayersCount = len(hb.CurrentPlayers)
	}
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
		logger.Debugf("error getting sessionID %s", sessionIDErr.Error())
	}

	if SessionCookieErr != nil {
		logger.Debugf("error getting sessionCookie %s", SessionCookieErr.Error())
	}

	return sessionID, sessionCookie
}
