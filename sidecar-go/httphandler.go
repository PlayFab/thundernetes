package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
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

type httpHandler struct {
	k8sClient             dynamic.Interface
	previousGameState     GameState
	previousGameHealth    string
	gameServerName        string
	gameServerNamespace   string
	connectedPlayersCount int
}

func NewHttpHandler(k8sClient dynamic.Interface, gameServerName, gameServerNamespace string) httpHandler {
	hh := httpHandler{
		previousGameState:   GameStateInitializing,
		previousGameHealth:  "N/A",
		k8sClient:           k8sClient,
		gameServerName:      gameServerName,
		gameServerNamespace: gameServerNamespace,
	}

	hh.setupWatch()

	return hh
}

func (h *httpHandler) setupWatch() {
	// great article for reference https://firehydrant.io/blog/dynamic-kubernetes-informers/
	listOptions := dynamicinformer.TweakListOptionsFunc(func(options *metav1.ListOptions) {
		options.FieldSelector = fmt.Sprintf("metadata.name=%s", h.gameServerName)
	})
	dynInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(h.k8sClient, 0, h.gameServerNamespace, listOptions)
	informer := dynInformer.ForResource(gameserverGVR).Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: h.gameServerUpdated,
	})

	go informer.Run(watchStopper)
}

func (h *httpHandler) gameServerUpdated(oldObj, newObj interface{}) {
	// dynamic client returns an unstructured object
	old := oldObj.(*unstructured.Unstructured)
	new := newObj.(*unstructured.Unstructured)

	// get the old and the new state from .status.state
	oldState, oldStateExists, oldStateErr := unstructured.NestedString(old.Object, "status", "state")
	newState, newStateExists, newStateErr := unstructured.NestedString(new.Object, "status", "state")

	if oldStateErr != nil {
		log.Errorf("error getting old state %s", oldStateErr.Error())
		return
	}

	if newStateErr != nil {
		log.Errorf("error getting new state %s", newStateErr.Error())
		return
	}

	if !oldStateExists || !newStateExists {
		log.Errorf("state does not exist, oldStateExists:%t, newStateExists:%t", oldStateExists, newStateExists)
		return
	}

	log.Infof("GameServer CRD instance updated %s:%s,%s,%s", old.GetName(), oldState, new.GetName(), newState)

	// if the GameServer was allocated
	if oldState == string(GameStateStandingBy) && newState == string(GameStateActive) {
		sessionID, sessionCookie := h.parseSessionDetails(new)
		log.Infof("Got values from allocation, sessionID:%s, sessionCookie:%s", sessionID, sessionCookie)

		initialPlayers := h.getInitialPlayersDetails()
		log.Infof("Got values from allocation, initialPlayers:%#v", initialPlayers)

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

func (h *httpHandler) getInitialPlayersDetails() []string {
	obj, err := h.k8sClient.Resource(gameserverDetailGVR).Namespace(h.gameServerNamespace).Get(context.Background(), h.gameServerName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("error getting initial players details %s", err.Error())
		return nil
	}

	initialPlayers, initialPlayersExist, err := unstructured.NestedStringSlice(obj.Object, "spec", "initialPlayers")
	if err != nil {
		log.Warnf("error getting initial players %s", err.Error())
		return nil
	}
	if !initialPlayersExist {
		log.Warnf("initial players does not exist")
		return nil
	}

	return initialPlayers
}

func (h *httpHandler) parseSessionDetails(u *unstructured.Unstructured) (string, string) {
	sessionID, sessionIDExists, sessionIDErr := unstructured.NestedString(u.Object, "status", "sessionID")
	sessionCookie, sessionCookieExists, SessionCookieErr := unstructured.NestedString(u.Object, "status", "sessionCookie")

	if !sessionIDExists || !sessionCookieExists {
		log.Warnf("sessionID or sessionCookie do not exist, sessionIDExists:%t, sessionCookieExists:%t", sessionIDExists, sessionCookieExists)
	}

	if sessionIDErr != nil {
		log.Warnf("error getting sessionID %s", sessionIDErr.Error())
	}

	if SessionCookieErr != nil {
		log.Warnf("error getting sessionCookie %s", SessionCookieErr.Error())
	}

	return sessionID, sessionCookie
}

func (h *httpHandler) heartbeatHandler(w http.ResponseWriter, req *http.Request) {
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
		log.Debugf("heartbeat received from sessionHostId %s, data %#v", sessionHostId, hb)
	}

	if err := validateHeartbeatRequestArgs(&hb); err != nil {
		log.Warnf("error validating heartbeat request %s", err.Error())
		badRequest(w, err, "invalid heartbeat request")
		return
	}

	if err := h.updateHealthIfNeeded(ctx, &hb); err != nil {
		log.Errorf("error updating health %s", err.Error())
		internalServerError(w, err, "error updating health")
		return
	}

	// game has reached the standingBy state (GSDK ReadyForPlayers has been called)
	if h.previousGameState != hb.CurrentGameState && hb.CurrentGameState == GameStateStandingBy {
		if err := h.transitionStateToStandingBy(ctx, &hb); err != nil {
			log.Errorf("error updating state %s", err.Error())
			internalServerError(w, err, "error updating state")
			return
		}
	}

	if err := h.updateConnectedPlayersCountIfNeeded(ctx, &hb); err != nil {
		log.Errorf("error updating connected players count %s", err.Error())
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
	if sd.SessionID != "" {
		sc.SessionId = sd.SessionID
	}
	if sd.SessionCookie != "" {
		sc.SessionCookie = sd.SessionCookie
	}

	if sd.InitialPlayers != nil {
		sc.InitialPlayers = sd.InitialPlayers
	}

	hr := &HeartbeatResponse{
		Operation:     op,
		SessionConfig: *sc,
	}

	json, _ := json.Marshal(hr)
	w.WriteHeader(http.StatusOK)
	w.Write(json)
}

func (h *httpHandler) updateHealthIfNeeded(ctx context.Context, hb *HeartbeatRequest) error {
	if h.previousGameHealth != hb.CurrentGameHealth {
		log.Infof("Health is different than before, updating. Old health %s, new health %s", h.previousGameHealth, hb.CurrentGameHealth)
		payload := fmt.Sprintf("{\"status\":{\"health\":\"%s\"}}", hb.CurrentGameHealth)
		payloadBytes := []byte(payload)
		_, err := h.k8sClient.Resource(gameserverGVR).Namespace(h.gameServerNamespace).Patch(ctx, h.gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")

		if err != nil {
			return err
		}
		h.previousGameHealth = hb.CurrentGameHealth
	}
	return nil
}

func (h *httpHandler) updateConnectedPlayersCountIfNeeded(ctx context.Context, hb *HeartbeatRequest) error {
	// we're not interested in the connected players count if the game is not active
	if hb.CurrentGameState == GameStateActive && h.connectedPlayersCount != len(hb.CurrentPlayers) {
		log.Infof("ConnectedPlayersCount is different than before, updating. Old connectedPlayersCount %d, new connectedPlayersCount %d", h.connectedPlayersCount, len(hb.CurrentPlayers))
		payload := fmt.Sprintf("{\"spec\":{\"connectedPlayersCount\":%d}}", len(hb.CurrentPlayers))
		payloadBytes := []byte(payload)
		_, err := h.k8sClient.Resource(gameserverDetailGVR).Namespace(h.gameServerNamespace).Patch(ctx, h.gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			return err
		}
		h.connectedPlayersCount = len(hb.CurrentPlayers)
	}
	return nil
}

func (h *httpHandler) transitionStateToStandingBy(ctx context.Context, hb *HeartbeatRequest) error {
	log.Infof("State is different than before, updating. Old state %s, new state StandingBy", h.previousGameState)
	payload := fmt.Sprintf("{\"status\":{\"state\":\"%s\"}}", hb.CurrentGameState)
	payloadBytes := []byte(payload)
	_, err := h.k8sClient.Resource(gameserverGVR).Namespace(h.gameServerNamespace).Patch(ctx, h.gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return err
	}
	h.previousGameState = hb.CurrentGameState
	return nil
}
