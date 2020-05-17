package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

var (
	gameserverGVR = schema.GroupVersionResource{
		Group:    "mps.playfab.com",
		Version:  "v1alpha1",
		Resource: "gameservers",
	}
)

type httpHandler struct {
	k8sClient             dynamic.Interface
	previousGameState     GameState
	previousGameHealth    string
	gameServerName        string
	gameServerNamespace   string
	userSetSessionDetails SessionDetails
	mux                   *sync.RWMutex
}

func NewHttpHandler(k8sClient dynamic.Interface, gameServerName, gameServerNamespace string) httpHandler {
	return httpHandler{
		previousGameState:   GameStateInitializing,
		previousGameHealth:  "N/A",
		k8sClient:           k8sClient,
		gameServerName:      gameServerName,
		gameServerNamespace: gameServerNamespace,
		userSetSessionDetails: SessionDetails{
			State: string(GameStateInvalid),
		},
		mux: &sync.RWMutex{},
	}
}

func (h *httpHandler) changeStateHandler(w http.ResponseWriter, req *http.Request) {
	s := SessionDetails{}
	err := json.NewDecoder(req.Body).Decode(&s)
	if err != nil {
		badRequest(w, err, "cannot deserialize json")
		return
	}

	fmt.Printf("sessiondetails request received for sessionId %s, data %#v\n", s.SessionId, s)

	if err := validateSessionDetailsArgs(&s); err != nil {
		fmt.Printf("error validating change state request %s\n", err.Error())
		badRequest(w, err, "invalid change state request")
		return
	}

	h.mux.Lock()
	h.userSetSessionDetails = s
	defer h.mux.Unlock()

	w.WriteHeader(http.StatusOK)
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

	fmt.Printf("heartbeat received from sessionHostId %s, data %#v\n", sessionHostId, hb)

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

	if h.previousGameState != hb.CurrentGameState && hb.CurrentGameState == GameStateStandingBy {
		if err := h.transitionStateToStandingBy(ctx, &hb); err != nil {
			fmt.Printf("error updating state %s\n", err.Error())
			internalServerError(w, err, "error updating state")
			return
		}
	}

	var op GameOperation = GameOperationContinue

	h.mux.RLock()
	sd := h.userSetSessionDetails
	h.mux.RUnlock()

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
	if sd.SessionId != "" {
		sc.SessionId = sd.SessionId
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

func (h *httpHandler) transitionStateToStandingBy(ctx context.Context, hb *HeartbeatRequest) error {
	fmt.Printf("State is different than before, updating. Old state %s, new state StandingBy", h.previousGameState)
	payload := fmt.Sprintf("{\"status\":{\"state\":\"%s\"}}", hb.CurrentGameState)
	payloadBytes := []byte(payload)
	_, err := h.k8sClient.Resource(gameserverGVR).Namespace(h.gameServerNamespace).Patch(ctx, h.gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")

	if err != nil {
		return err
	}
	h.previousGameState = hb.CurrentGameState
	return nil
}

func (h *httpHandler) getGameServerStateFromK8s(ctx context.Context) (*SessionDetails, error) {
	sd := &SessionDetails{}
	gs, err := h.k8sClient.Resource(gameserverGVR).Namespace(h.gameServerNamespace).Get(ctx, h.gameServerName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	stu, exists := gs.Object["status"]
	if !exists {
		return nil, errors.New("status subresource does not exist")
	}
	st := stu.(map[string]interface{})
	state, exists := st["state"]
	if exists {
		sd.State = state.(string)
	}
	sessionCookie, exists := st["sessionCookie"]
	if exists {
		sd.SessionCookie = sessionCookie.(string)
	}
	sessionId, exists := st["sessionId"]
	if exists {
		sd.SessionId = sessionId.(string)
	}
	initialPlayers, exists := st["initialPlayers"]
	if exists {
		temp := initialPlayers.([]interface{})
		sd.InitialPlayers = make([]string, len(temp))
		for i := 0; i < len(temp); i++ {
			sd.InitialPlayers = append(sd.InitialPlayers, temp[i].(string))
		}
	}
	return sd, nil
}
