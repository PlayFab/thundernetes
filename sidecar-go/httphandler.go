package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"

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
	userSetSessionDetails = &SessionDetails{
		State: string(GameStateInvalid),
	}
	watchStopper              = make(chan struct{})
	mux                       = &sync.RWMutex{}
	connectedPlayersCount int = 0
)

const logEveryHeartbeat = false

type httpHandler struct {
	k8sClient           dynamic.Interface
	previousGameState   GameState
	previousGameHealth  string
	gameServerName      string
	gameServerNamespace string
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
		fmt.Printf("error getting old state %s\n", oldStateErr.Error())
		return
	}

	if newStateErr != nil {
		fmt.Printf("error getting new state %s\n", newStateErr.Error())
		return
	}

	if !oldStateExists || !newStateExists {
		fmt.Printf("state does not exist, oldStateExists:%t, newStateExists:%t\n", oldStateExists, newStateExists)
		return
	}

	fmt.Printf("GameServer CRD instance updated %s:%s,%s,%s\n", old.GetName(), oldState, new.GetName(), newState)

	// if the GameServer was allocated
	if oldState == string(GameStateStandingBy) && newState == string(GameStateActive) {
		sessionID, sessionCookie := h.getSessionDetails(new)

		fmt.Printf("Got values from allocation, sessionID:%s, sessionCookie:%s\n", sessionID, sessionCookie)

		initialPlayers := h.getInitialPlayersDetails()

		mux.Lock()
		userSetSessionDetails = &SessionDetails{
			SessionID:      sessionID,
			SessionCookie:  sessionCookie,
			InitialPlayers: initialPlayers,
			State:          string(GameStateActive),
		}
		connectedPlayersCount = len(initialPlayers)
		mux.Unlock()

		// closing the channel will cause the informer to stop
		// we don't expect any more state changes so we close the watch to decrease the pressue on Kubernetes API server
		close(watchStopper)
	}
}

func (h *httpHandler) getInitialPlayersDetails() []string {
	obj, err := h.k8sClient.Resource(gameserverDetailGVR).Namespace(h.gameServerNamespace).Get(context.Background(), h.gameServerName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("error getting initial players details %s\n", err.Error())
		return []string{}
	}

	initialPlayers, initialPlayersExist, err := unstructured.NestedStringSlice(obj.Object, "spec", "initialPlayers")
	if err != nil {
		fmt.Printf("error getting initial players details %s\n", err.Error())
		return []string{}
	}
	if !initialPlayersExist {
		fmt.Printf("initial players details does not exist\n")
		return []string{}
	}

	return initialPlayers
}

func (h *httpHandler) getSessionDetails(u *unstructured.Unstructured) (string, string) {
	sessionID, sessionIDExists, sessionIDErr := unstructured.NestedString(u.Object, "status", "sessionID")
	sessionCookie, sessionCookieExists, SessionCookieErr := unstructured.NestedString(u.Object, "status", "sessionCookie")

	if !sessionIDExists || !sessionCookieExists {
		fmt.Printf("sessionID or sessionCookie do not exist, sessionIDExists:%t, sessionCookieExists:%t\n", sessionIDExists, sessionCookieExists)
	}

	if sessionIDErr != nil {
		fmt.Printf("error getting sessionID %s\n", sessionIDErr.Error())
	}

	if SessionCookieErr != nil {
		fmt.Printf("error getting sessionCookie %s\n", SessionCookieErr.Error())
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
		fmt.Printf("heartbeat received from sessionHostId %s, data %#v\n", sessionHostId, hb)
	}

	if err := validateHeartbeatRequestArgs(&hb); err != nil {
		fmt.Printf("error validating heartbeat request %s\n", err.Error())
		badRequest(w, err, "invalid heartbeat request")
		return
	}

	if err := h.updateHealthIfNeeded(ctx, &hb); err != nil {
		fmt.Printf("error updating health %s\n", err.Error())
		internalServerError(w, err, "error updating health")
		return
	}

	if err := h.updateConnectedPlayersCountIfNeeded(ctx, &hb); err != nil {
		fmt.Printf("error updating connected players count %s\n", err.Error())
		internalServerError(w, err, "error updating connected players count")
		return
	}

	if h.previousGameState != hb.CurrentGameState && hb.CurrentGameState == GameStateStandingBy {
		if err := h.transitionStateToStandingBy(ctx, &hb); err != nil {
			fmt.Printf("error updating state %s\n", err.Error())
			internalServerError(w, err, "error updating state")
			return
		}
	}

	var op GameOperation = GameOperationContinue

	mux.RLock()
	sd := userSetSessionDetails
	mux.RUnlock()

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
		fmt.Printf("Health is different than before, updating. Old health %s, new health %s\n", h.previousGameHealth, hb.CurrentGameHealth)
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
	if connectedPlayersCount != len(hb.CurrentPlayers) {
		fmt.Printf("ConnectedPlayersCount is different than before, updating. Old connectedPlayersCount %d, new connectedPlayersCount %d\n", connectedPlayersCount, len(hb.CurrentPlayers))
		payload := fmt.Sprintf("{\"spec\":{\"connectedPlayersCount\":%d}}", len(hb.CurrentPlayers))
		payloadBytes := []byte(payload)
		_, err := h.k8sClient.Resource(gameserverDetailGVR).Namespace(h.gameServerNamespace).Patch(ctx, h.gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			return err
		}
		connectedPlayersCount = len(hb.CurrentPlayers)
	}
	return nil
}

func (h *httpHandler) transitionStateToStandingBy(ctx context.Context, hb *HeartbeatRequest) error {
	fmt.Printf("State is different than before, updating. Old state %s, new state StandingBy\n", h.previousGameState)
	payload := fmt.Sprintf("{\"status\":{\"state\":\"%s\"}}", hb.CurrentGameState)
	payloadBytes := []byte(payload)
	_, err := h.k8sClient.Resource(gameserverGVR).Namespace(h.gameServerNamespace).Patch(ctx, h.gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")

	if err != nil {
		return err
	}
	h.previousGameState = hb.CurrentGameState
	return nil
}
