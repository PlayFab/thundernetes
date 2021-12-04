package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"net/http"
	"os"
	"regexp"

	hm "github.com/cornelk/hashmap"
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

	gameServerMap = &hm.HashMap{}
	dynamicClient dynamic.Interface
	watchStopper  = make(chan struct{})
)

const (
	logEveryHeartbeat   = true
	GameServerName      = "GameServerName"
	GameServerNamespace = "GameServerNamespace"
)

func main() {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatalf("NODE_NAME environment variable must be set")
	}

	nodeInternalIp := os.Getenv("NODE_INTERNAL_IP")
	if nodeInternalIp == "" {
		log.Fatalf("NODE_INTERNAL_IP environment variable must be set")
	}

	var err error
	dynamicClient, err = initializeKubernetesClient()
	if err != nil {
		log.Fatal(err)
	}

	listOptions := dynamicinformer.TweakListOptionsFunc(func(options *metav1.ListOptions) {
		options.LabelSelector = fmt.Sprintf("NodeName=%s", nodeName)
	})

	dynInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, 0, v1.NamespaceAll, listOptions)
	informer := dynInformer.ForResource(gameserverGVR).Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    gameServerCreated,
		UpdateFunc: gameServerUpdated,
		DeleteFunc: gameServerDeleted,
	})

	go informer.Run(watchStopper)

	log.Debug("Starting HTTP server")
	http.HandleFunc("/v1/sessionHosts/", heartbeatHandler)
	err = http.ListenAndServe(fmt.Sprintf(":%d", 56001), nil)
	if err != nil {
		log.Fatal(err)
	}
	close(watchStopper)
}

func gameServerCreated(objUnstructured interface{}) {
	obj := objUnstructured.(*unstructured.Unstructured)
	gameServerName := obj.GetName()
	gameServerNamespace := obj.GetNamespace()

	ok := gameServerMap.Insert(gameServerName, &GameServerDetails{
		GameServerNamespace: gameServerNamespace,
		Mutex:               &sync.RWMutex{},
	})
	if !ok {
		logger := getLogger(gameServerName, gameServerNamespace)
		logger.Errorf("GameServer %s/%s already exists in map", gameServerNamespace, gameServerName)
	}
}

func gameServerUpdated(oldObj, newObj interface{}) {
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
		logger.Errorf("error getting old state %s", oldStateErr.Error())
		return
	}

	if newStateErr != nil {
		logger.Errorf("error getting new state %s", newStateErr.Error())
		return
	}

	if !oldStateExists || !newStateExists {
		logger.Warnf("state does not exist, oldStateExists:%t, newStateExists:%t", oldStateExists, newStateExists)
		return
	}

	logger.Infof("GameServer CR updated %s:%s,%s,%s", old.GetName(), oldState, new.GetName(), newState)

	gsdi, exists := gameServerMap.Get(gameServerName)

	if !exists {
		logger.Errorf("GameServer %s/%s does not exist in map", gameServerNamespace, gameServerName)
		return
	}

	gsd := gsdi.(*GameServerDetails)

	if gsd.CurrentState == GameStateStandingBy && newState == string(GameStateActive) {
		sessionID, sessionCookie := parseSessionDetails(new, gameServerName, gameServerNamespace)
		logger.Infof("Got values from allocation, sessionID:%s, sessionCookie:%s", sessionID, sessionCookie)

		initialPlayers := getInitialPlayers(gameServerName, gameServerNamespace)
		logger.Infof("Got values from allocation, initialPlayers:%#v", initialPlayers)

		gsd.Mutex.Lock()
		gsd.CurrentState = GameStateActive
		gsd.SessionCookie = sessionCookie
		gsd.SessionID = sessionID
		gsd.InitialPlayers = initialPlayers
		gsd.Mutex.Unlock()
	}
}

func gameServerDeleted(objUnstructured interface{}) {
	obj := objUnstructured.(*unstructured.Unstructured)
	gameServerName := obj.GetName()
	gameServerNamespace := obj.GetNamespace()

	log.WithFields(log.Fields{
		GameServerName:      gameServerName,
		GameServerNamespace: gameServerNamespace,
	}).Infof("GameServer %s/%s deleted", gameServerNamespace, gameServerName)

	gameServerMap.Del(gameServerName)
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
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

	gsdi, exists := gameServerMap.Get(gameServerName)
	if !exists {
		internalServerError(w, fmt.Errorf("game server %s not found", gameServerName), "gameserver not found")
		return
	}
	gsd := gsdi.(*GameServerDetails)

	logger := getLogger(gameServerName, gsd.GameServerNamespace)

	if logEveryHeartbeat {
		logger.Infof("heartbeat received from sessionHostId %s, data %#v", gameServerName, hb)
	}

	if err := updateHealthAndStateIfNeeded(ctx, &hb, gameServerName, gsd); err != nil {
		logger.Errorf("error updating health %s", err.Error())
		internalServerError(w, err, "error updating health")
		return
	}

	if err := updateConnectedPlayersIfNeeded(ctx, &hb, gameServerName, gsd); err != nil {
		logger.Errorf("error updating connected players count %s", err.Error())
		internalServerError(w, err, "error updating connected players count")
		return
	}

	var op GameOperation = GameOperationContinue
	gsd.Mutex.RLock()
	defer gsd.Mutex.RUnlock()

	if gsd.CurrentState == GameStateInvalid { // user has not set the status yet
		op = GameOperationContinue
	} else if gsd.CurrentState == GameStateInitializing {
		op = GameOperationContinue
	} else if gsd.CurrentState == GameStateStandingBy {
		op = GameOperationContinue
	} else if gsd.CurrentState == GameStateActive {
		op = GameOperationActive
	} else if gsd.CurrentState == GameStateTerminated || gsd.CurrentState == GameStateTerminating {
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
func updateHealthAndStateIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerDetails) error {
	logger := getLogger(gameServerName, gsd.GameServerNamespace)
	ok := isValidStateTransition(gsd.CurrentState, hb.CurrentGameState)
	if !ok {
		logger.Warnf("invalid state transition from %s to %s", gsd.CurrentState, hb.CurrentGameState)
		return nil
	}

	if gsd.CurrentHealth != hb.CurrentGameHealth || gsd.CurrentState != hb.CurrentGameState {
		logger.Infof("Health or state is different than before, updating. Old health %s, new health %s, old state %s, new state %s", gsd.CurrentHealth, hb.CurrentGameHealth, gsd.CurrentState, hb.CurrentGameState)
		payload := fmt.Sprintf("{\"status\":{\"health\":\"%s\",\"state\":\"%s\"}}", hb.CurrentGameHealth, hb.CurrentGameState)
		payloadBytes := []byte(payload)
		_, err := dynamicClient.Resource(gameserverGVR).Namespace(gsd.GameServerNamespace).Patch(ctx, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")

		if err != nil {
			return err
		}
		gsd.CurrentHealth = hb.CurrentGameHealth
		gsd.CurrentState = hb.CurrentGameState
	}
	return nil
}

// getInitialPlayers returns the initial players from the unstructured GameServerDetail CR
func getInitialPlayers(gameServerName, gameServerNamespace string) []string {
	logger := getLogger(gameServerName, gameServerNamespace)
	obj, err := dynamicClient.Resource(gameserverDetailGVR).Namespace(gameServerNamespace).Get(context.Background(), gameServerName, metav1.GetOptions{})
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

func updateConnectedPlayersIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerDetails) error {
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
		_, err := dynamicClient.Resource(gameserverDetailGVR).Namespace(gsd.GameServerNamespace).Patch(ctx, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			return err
		}
		// storing the current number in memory
		gsd.ConnectedPlayersCount = len(hb.CurrentPlayers)
	}
	return nil
}

// parseSessionDetails returns the sessionID and sessionCookie from the unstructured GameServer CR
func parseSessionDetails(u *unstructured.Unstructured, gameServerName, gameServerNamespace string) (string, string) {
	logger := getLogger(gameServerName, gameServerNamespace)
	sessionID, sessionIDExists, sessionIDErr := unstructured.NestedString(u.Object, "status", "sessionID")
	sessionCookie, sessionCookieExists, SessionCookieErr := unstructured.NestedString(u.Object, "status", "sessionCookie")

	if !sessionIDExists || !sessionCookieExists {
		logger.Warnf("sessionID or sessionCookie do not exist, sessionIDExists:%t, sessionCookieExists:%t", sessionIDExists, sessionCookieExists)
	}

	if sessionIDErr != nil {
		logger.Warnf("error getting sessionID %s", sessionIDErr.Error())
	}

	if SessionCookieErr != nil {
		logger.Warnf("error getting sessionCookie %s", SessionCookieErr.Error())
	}

	return sessionID, sessionCookie
}

func getLogger(gameServerName, gameServerNamespace string) *log.Entry {
	return log.WithFields(log.Fields{"GameServerName": gameServerName, "GameServerNamespace": gameServerNamespace})
}
