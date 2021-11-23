package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

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
)

var (
	sessionDetails = &SessionDetails{
		State: string(GameStateInvalid),
	}
	watchStopper        = make(chan struct{})
	sessionDetailsMutex = &sync.RWMutex{}
)

const logEveryHeartbeat = false

// sidecarManager is responsible for all the sidecar related operations
// mainly accepting heartbeats from the game server process and updating the GameServer CR by communicating with the Kubernetes API server
type sidecarManager struct {
	k8sClient             dynamic.Interface
	previousGameState     GameState
	previousGameHealth    string
	gameServerName        string
	gameServerNamespace   string
	connectedPlayersCount int
	logger                *log.Entry
}

// NewSidecarManager creates a new sidecarManager
func NewSidecarManager(k8sClient dynamic.Interface, gameServerName, gameServerNamespace string, logger *log.Entry) sidecarManager {
	sm := sidecarManager{
		previousGameState:   "",
		previousGameHealth:  "",
		k8sClient:           k8sClient,
		gameServerName:      gameServerName,
		gameServerNamespace: gameServerNamespace,
		logger:              logger,
	}

	sm.setupWatch()

	return sm
}

// setupWatch sets up the informer to watch the GameServer CRD
func (sm *sidecarManager) setupWatch() {
	// great article for reference https://firehydrant.io/blog/dynamic-kubernetes-informers/
	listOptions := dynamicinformer.TweakListOptionsFunc(func(options *metav1.ListOptions) {
		options.FieldSelector = fmt.Sprintf("metadata.name=%s", sm.gameServerName)
	})
	dynInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(sm.k8sClient, 0, sm.gameServerNamespace, listOptions)
	informer := dynInformer.ForResource(gameserverGVR).Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: sm.gameServerUpdated,
	})

	go informer.Run(watchStopper)
}

// gameServerUpdated runs when the GameServer CR has been updated
func (sm *sidecarManager) gameServerUpdated(oldObj, newObj interface{}) {
	// dynamic client returns an unstructured object
	old := oldObj.(*unstructured.Unstructured)
	new := newObj.(*unstructured.Unstructured)

	// get the old and the new state from .status.state
	oldState, oldStateExists, oldStateErr := unstructured.NestedString(old.Object, "status", "state")
	newState, newStateExists, newStateErr := unstructured.NestedString(new.Object, "status", "state")

	if oldStateErr != nil {
		sm.logger.Errorf("error getting old state %s", oldStateErr.Error())
		return
	}

	if newStateErr != nil {
		sm.logger.Errorf("error getting new state %s", newStateErr.Error())
		return
	}

	if !oldStateExists || !newStateExists {
		sm.logger.Warnf("state does not exist, oldStateExists:%t, newStateExists:%t", oldStateExists, newStateExists)
		return
	}

	sm.logger.Infof("GameServer CR updated %s:%s,%s,%s", old.GetName(), oldState, new.GetName(), newState)

	// if the GameServer was allocated
	if oldState == string(GameStateStandingBy) && newState == string(GameStateActive) {
		sessionID, sessionCookie := sm.parseSessionDetails(new)
		sm.logger.Infof("Got values from allocation, sessionID:%s, sessionCookie:%s", sessionID, sessionCookie)

		initialPlayers := sm.getInitialPlayers()
		sm.logger.Infof("Got values from allocation, initialPlayers:%#v", initialPlayers)

		sessionDetailsMutex.Lock()
		sessionDetails = &SessionDetails{
			SessionID:      sessionID,
			SessionCookie:  sessionCookie,
			InitialPlayers: initialPlayers,
			State:          string(GameStateActive),
		}
		sessionDetailsMutex.Unlock()

		// closing the channel will cause the informer to stop
		// we don't expect any more state changes so we close the watch to decrease the pressue on Kubernetes API server
		close(watchStopper)
	}
}

// getInitialPlayers returns the initial players from the unstructured GameServerDetail CR
func (sm *sidecarManager) getInitialPlayers() []string {
	obj, err := sm.k8sClient.Resource(gameserverDetailGVR).Namespace(sm.gameServerNamespace).Get(context.Background(), sm.gameServerName, metav1.GetOptions{})
	if err != nil {
		sm.logger.Warnf("error getting initial players details %s", err.Error())
		return []string{}
	}

	initialPlayers, initialPlayersExist, err := unstructured.NestedStringSlice(obj.Object, "spec", "initialPlayers")
	if err != nil {
		sm.logger.Warnf("error getting initial players %s", err.Error())
		return []string{}
	}
	if !initialPlayersExist {
		sm.logger.Warnf("initial players does not exist")
		return []string{}
	}

	return initialPlayers
}

// parseSessionDetails returns the sessionID and sessionCookie from the unstructured GameServer CR
func (sm *sidecarManager) parseSessionDetails(u *unstructured.Unstructured) (string, string) {
	sessionID, sessionIDExists, sessionIDErr := unstructured.NestedString(u.Object, "status", "sessionID")
	sessionCookie, sessionCookieExists, SessionCookieErr := unstructured.NestedString(u.Object, "status", "sessionCookie")

	if !sessionIDExists || !sessionCookieExists {
		sm.logger.Warnf("sessionID or sessionCookie do not exist, sessionIDExists:%t, sessionCookieExists:%t", sessionIDExists, sessionCookieExists)
	}

	if sessionIDErr != nil {
		sm.logger.Warnf("error getting sessionID %s", sessionIDErr.Error())
	}

	if SessionCookieErr != nil {
		sm.logger.Warnf("error getting sessionCookie %s", SessionCookieErr.Error())
	}

	return sessionID, sessionCookie
}

// heartbeathandler is the http handler for the heartbeats coming from the game server process, facilitate through GSDK
func (sm *sidecarManager) heartbeatHandler(w http.ResponseWriter, req *http.Request) {
	ctx := context.Background()
	re := regexp.MustCompile(`.*/v1/sessionHosts\/(.*?)(/heartbeats|$)`)
	match := re.FindStringSubmatch(req.RequestURI)

	sessionHostId := match[1]

	var hb HeartbeatRequest
	err := json.NewDecoder(req.Body).Decode(&hb)
	if err != nil {
		badRequest(w, err, "cannot deserialize json")
		return
	}

	if logEveryHeartbeat {
		sm.logger.Debugf("heartbeat received from sessionHostId %s, data %#v", sessionHostId, hb)
	}

	if err := validateHeartbeatRequestArgs(&hb); err != nil {
		sm.logger.Warnf("error validating heartbeat request %s", err.Error())
		badRequest(w, err, "invalid heartbeat request")
		return
	}

	if err := sm.updateHealthAndStateIfNeeded(ctx, &hb); err != nil {
		sm.logger.Errorf("error updating health %s", err.Error())
		internalServerError(w, err, "error updating health")
		return
	}

	if err := sm.updateConnectedPlayersIfNeeded(ctx, &hb); err != nil {
		sm.logger.Errorf("error updating connected players count %s", err.Error())
		internalServerError(w, err, "error updating connected players count")
		return
	}

	var op GameOperation = GameOperationContinue

	sessionDetailsMutex.RLock()
	sd := sessionDetails
	sessionDetailsMutex.RUnlock()

	if sd.State == string(GameStateInvalid) { // user has not set the status yet
		op = GameOperationContinue
	} else if sd.State == string(GameStateInitializing) {
		op = GameOperationContinue
	} else if sd.State == string(GameStateStandingBy) {
		op = GameOperationContinue
	} else if sd.State == string(GameStateActive) {
		op = GameOperationActive
	} else if sd.State == string(GameStateTerminated) || sd.State == string(GameStateTerminating) {
		op = GameOperationTerminate
	}

	sc := &SessionConfig{}
	sc.SessionId = sd.SessionID
	sc.SessionCookie = sd.SessionCookie
	sc.InitialPlayers = sd.InitialPlayers

	hr := &HeartbeatResponse{
		Operation:     op,
		SessionConfig: *sc,
	}

	json, _ := json.Marshal(hr)
	w.WriteHeader(http.StatusOK)
	w.Write(json)
}

// updateHealthAndStateIfNeeded updates both the health and state of the GameServer if any one of them has changed
func (sm *sidecarManager) updateHealthAndStateIfNeeded(ctx context.Context, hb *HeartbeatRequest) error {
	if sm.previousGameHealth != hb.CurrentGameHealth || sm.previousGameState != hb.CurrentGameState {
		sm.logger.Infof("Health or state is different than before, updating. Old health %s, new health %s, old state %s, new state %s", sm.previousGameHealth, hb.CurrentGameHealth, sm.previousGameState, hb.CurrentGameState)
		payload := fmt.Sprintf("{\"status\":{\"health\":\"%s\",\"state\":\"%s\"}}", hb.CurrentGameHealth, hb.CurrentGameState)
		payloadBytes := []byte(payload)
		_, err := sm.k8sClient.Resource(gameserverGVR).Namespace(sm.gameServerNamespace).Patch(ctx, sm.gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")

		if err != nil {
			return err
		}
		sm.previousGameHealth = hb.CurrentGameHealth
		sm.previousGameState = hb.CurrentGameState
	}
	return nil
}

// updateConnectedPlayersIfNeeded updates the connected players count of the GameServer CRD if the connected players count has changed
func (sm *sidecarManager) updateConnectedPlayersIfNeeded(ctx context.Context, hb *HeartbeatRequest) error {
	// we're not interested in updating the connected players count if the game is not active
	if hb.CurrentGameState == GameStateActive && sm.connectedPlayersCount != len(hb.CurrentPlayers) {
		currentPlayerIDs := make([]string, len(hb.CurrentPlayers))
		for i := 0; i < len(hb.CurrentPlayers); i++ {
			currentPlayerIDs[i] = hb.CurrentPlayers[i].PlayerId
		}
		sm.logger.Infof("ConnectedPlayersCount is different than before, updating. Old connectedPlayersCount %d, new connectedPlayersCount %d", sm.connectedPlayersCount, len(hb.CurrentPlayers))
		var payload string
		if len(hb.CurrentPlayers) == 0 {
			payload = "{\"spec\":{\"connectedPlayersCount\":0,\"connectedPlayers\":[]}}"
		} else {
			payload = fmt.Sprintf("{\"spec\":{\"connectedPlayersCount\":%d,\"connectedPlayers\":[\"%s\"]}}", len(hb.CurrentPlayers), strings.Join(currentPlayerIDs, "\",\""))
		}
		payloadBytes := []byte(payload)
		_, err := sm.k8sClient.Resource(gameserverDetailGVR).Namespace(sm.gameServerNamespace).Patch(ctx, sm.gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			return err
		}
		// storing the current number in memory
		sm.connectedPlayersCount = len(hb.CurrentPlayers)
	}
	return nil
}
