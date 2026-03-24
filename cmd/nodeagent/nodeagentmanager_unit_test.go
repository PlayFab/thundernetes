package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	scheme2 "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// newDynamicInterfaceWithDetails creates a fake dynamic client that registers
// both gameserverGVR and gameserverDetailGVR so tests involving
// GameServerDetails work correctly.
func newDynamicInterfaceWithDetails() dynamic.Interface {
	SchemeBuilder := &scheme.Builder{GroupVersion: gameserverGVR.GroupVersion()}
	SchemeBuilder.AddToScheme(scheme2.Scheme)
	gvrMap := make(map[schema.GroupVersionResource]string)
	gvrMap[gameserverGVR] = "GameServerList"
	gvrMap[gameserverDetailGVR] = "GameServerDetailList"
	return fake.NewSimpleDynamicClientWithCustomListKinds(scheme2.Scheme, gvrMap)
}

// newTestNodeAgentManager creates a NodeAgentManager without starting a watch,
// which avoids informer timing issues in unit tests.
func newTestNodeAgentManager(dynamicClient dynamic.Interface, opts ...func(*NodeAgentManager)) *NodeAgentManager {
	n := &NodeAgentManager{
		dynamicClient:             dynamicClient,
		gameServerMap:             &sync.Map{},
		nodeName:                  testNodeName,
		logEveryHeartbeat:         false,
		ignoreHealthFromHeartbeat: false,
		nowFunc:                   time.Now,
		heartbeatTimeout:          5000,
		firstHeartbeatTimeout:     60000,
	}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// sendHeartbeat is a helper that sends a heartbeat HTTP request and returns the
// recorder, decoded HeartbeatResponse, and the raw response body.
func sendHeartbeat(t *testing.T, n *NodeAgentManager, gsName string, hb *HeartbeatRequest) (*httptest.ResponseRecorder, *HeartbeatResponse, []byte) {
	t.Helper()
	b, err := json.Marshal(hb)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s/heartbeats", gsName), bytes.NewReader(b))
	w := httptest.NewRecorder()
	n.heartbeatHandler(w, req)
	res := w.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var hbr HeartbeatResponse
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(body, &hbr))
	}
	return w, &hbr, body
}

// ---------- heartbeatHandler tests ----------

func TestUnitHeartbeatHandler_NonExistentGameServer(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateStandingBy,
		CurrentGameHealth: "Healthy",
	}
	w, _, _ := sendHeartbeat(t, n, "nonexistent-gs", hb)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUnitHeartbeatHandler_LogEveryHeartbeat(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient, func(n *NodeAgentManager) {
		n.logEveryHeartbeat = true
	})

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
	require.NoError(t, err)

	n.gameServerMap.Store(testGameServerName, &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	})

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateInitializing,
		CurrentGameHealth: "Healthy",
	}
	w, _, _ := sendHeartbeat(t, n, testGameServerName, hb)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUnitHeartbeatHandler_IgnoreHealthFromHeartbeat(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient, func(n *NodeAgentManager) {
		n.ignoreHealthFromHeartbeat = true
	})

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
	require.NoError(t, err)

	n.gameServerMap.Store(testGameServerName, &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	})

	// Send an "Unhealthy" heartbeat; with ignoreHealthFromHeartbeat the health
	// should be forced to Healthy.
	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateInitializing,
		CurrentGameHealth: "Unhealthy",
	}
	w, _, _ := sendHeartbeat(t, n, testGameServerName, hb)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify that the stored previous health is Healthy (overridden).
	gsdi, ok := n.gameServerMap.Load(testGameServerName)
	require.True(t, ok)
	gsd := gsdi.(*GameServerInfo)
	gsd.Mutex.RLock()
	defer gsd.Mutex.RUnlock()
	assert.Equal(t, string(mpsv1alpha1.GameServerHealthy), gsd.PreviousGameHealth)
}

func TestUnitHeartbeatHandler_ActiveServer_StandingByHeartbeat(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
	require.NoError(t, err)

	// Pre-populate the map with an Active server whose previous state is StandingBy.
	n.gameServerMap.Store(testGameServerName, &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		IsActive:            true,
		PreviousGameState:   GameStateStandingBy,
		PreviousGameHealth:  "Healthy",
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	})

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateStandingBy,
		CurrentGameHealth: "Healthy",
	}
	w, hbr, _ := sendHeartbeat(t, n, testGameServerName, hb)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, GameOperationActive, hbr.Operation)
}

func TestUnitHeartbeatHandler_ActiveServer_ActiveHeartbeat(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
	require.NoError(t, err)

	// Server is active and has already acknowledged the Active state.
	n.gameServerMap.Store(testGameServerName, &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		IsActive:            true,
		PreviousGameState:   GameStateActive,
		PreviousGameHealth:  "Healthy",
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	})

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateActive,
		CurrentGameHealth: "Healthy",
	}
	w, hbr, _ := sendHeartbeat(t, n, testGameServerName, hb)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, GameOperationContinue, hbr.Operation)
}

// ---------- updateHealthAndStateIfNeeded tests ----------

func TestUnitUpdateHealthAndState_NoChange(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	gsd := &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		PreviousGameHealth:  "Healthy",
		PreviousGameState:   GameStateStandingBy,
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateStandingBy,
		CurrentGameHealth: "Healthy",
	}
	err := n.updateHealthAndStateIfNeeded(context.Background(), hb, testGameServerName, gsd)
	assert.NoError(t, err)
}

func TestUnitUpdateHealthAndState_ValidTransitions(t *testing.T) {
	tests := []struct {
		name     string
		oldState GameState
		newState GameState
	}{
		{"empty to Initializing", "", GameStateInitializing},
		{"Initializing to StandingBy", GameStateInitializing, GameStateStandingBy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient := newDynamicInterface()
			n := newTestNodeAgentManager(dynamicClient)

			gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
			_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
			require.NoError(t, err)

			gsd := &GameServerInfo{
				GameServerNamespace: testGameServerNamespace,
				PreviousGameState:   tt.oldState,
				PreviousGameHealth:  "",
				Mutex:               &sync.RWMutex{},
				BuildName:           testBuildName,
			}

			hb := &HeartbeatRequest{
				CurrentGameState:  tt.newState,
				CurrentGameHealth: "Healthy",
			}
			err = n.updateHealthAndStateIfNeeded(context.Background(), hb, testGameServerName, gsd)
			assert.NoError(t, err)

			gsd.Mutex.RLock()
			assert.Equal(t, tt.newState, gsd.PreviousGameState)
			assert.Equal(t, "Healthy", gsd.PreviousGameHealth)
			gsd.Mutex.RUnlock()
		})
	}
}

func TestUnitUpdateHealthAndState_InvalidTransition(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	gsd := &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		PreviousGameState:   GameStateStandingBy,
		PreviousGameHealth:  "Healthy",
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateInitializing,
		CurrentGameHealth: "Healthy",
	}
	err := n.updateHealthAndStateIfNeeded(context.Background(), hb, testGameServerName, gsd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state transition")
}

func TestUnitUpdateHealthAndState_ActiveRecoveryAfterCrash(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
	require.NoError(t, err)

	// Simulate a NodeAgent crash scenario: PreviousGameState is empty while
	// the heartbeat reports Active.
	gsd := &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		PreviousGameState:   "",
		PreviousGameHealth:  "",
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateActive,
		CurrentGameHealth: "Healthy",
	}
	err = n.updateHealthAndStateIfNeeded(context.Background(), hb, testGameServerName, gsd)
	assert.NoError(t, err)

	gsd.Mutex.RLock()
	assert.Equal(t, GameStateActive, gsd.PreviousGameState)
	gsd.Mutex.RUnlock()
}

// ---------- updateConnectedPlayersIfNeeded tests ----------

func TestUnitUpdateConnectedPlayers_NotActive(t *testing.T) {
	dynamicClient := newDynamicInterfaceWithDetails()
	n := newTestNodeAgentManager(dynamicClient)

	gsd := &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateStandingBy,
		CurrentGameHealth: "Healthy",
		CurrentPlayers:    getTestConnectedPlayers(),
	}
	err := n.updateConnectedPlayersIfNeeded(context.Background(), hb, testGameServerName, gsd)
	assert.NoError(t, err)
	assert.Equal(t, 0, gsd.ConnectedPlayersCount)
}

func TestUnitUpdateConnectedPlayers_SameCount(t *testing.T) {
	dynamicClient := newDynamicInterfaceWithDetails()
	n := newTestNodeAgentManager(dynamicClient)

	gsd := &GameServerInfo{
		GameServerNamespace:   testGameServerNamespace,
		ConnectedPlayersCount: 2,
		Mutex:                 &sync.RWMutex{},
		BuildName:             testBuildName,
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateActive,
		CurrentGameHealth: "Healthy",
		CurrentPlayers:    getTestConnectedPlayers(), // 2 players
	}
	err := n.updateConnectedPlayersIfNeeded(context.Background(), hb, testGameServerName, gsd)
	assert.NoError(t, err)
	assert.Equal(t, 2, gsd.ConnectedPlayersCount)
}

func TestUnitUpdateConnectedPlayers_CountChanges(t *testing.T) {
	dynamicClient := newDynamicInterfaceWithDetails()
	n := newTestNodeAgentManager(dynamicClient)

	// First, create a GameServerDetail so the patch has a target.
	err := n.createGameServerDetails(context.Background(), "test-uid", testGameServerName, testGameServerNamespace, testBuildName, nil)
	require.NoError(t, err)

	gsd := &GameServerInfo{
		GameServerNamespace:   testGameServerNamespace,
		ConnectedPlayersCount: 0,
		Mutex:                 &sync.RWMutex{},
		BuildName:             testBuildName,
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateActive,
		CurrentGameHealth: "Healthy",
		CurrentPlayers:    getTestConnectedPlayers(), // 2 players
	}
	err = n.updateConnectedPlayersIfNeeded(context.Background(), hb, testGameServerName, gsd)
	assert.NoError(t, err)
	assert.Equal(t, 2, gsd.ConnectedPlayersCount)
}

// ---------- gameServerCreatedOrUpdated tests ----------

func TestUnitGameServerCreatedOrUpdated_NewServer(t *testing.T) {
	dynamicClient := newDynamicInterfaceWithDetails()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

	n.gameServerCreatedOrUpdated(gs)

	val, ok := n.gameServerMap.Load(testGameServerName)
	assert.True(t, ok)
	gsi := val.(*GameServerInfo)
	assert.Equal(t, testGameServerNamespace, gsi.GameServerNamespace)
	assert.NotEqual(t, int64(0), gsi.CreationTime)
}

func TestUnitGameServerCreatedOrUpdated_NonActiveDoesNotSetSessionDetails(t *testing.T) {
	dynamicClient := newDynamicInterfaceWithDetails()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	// State is StandingBy, health is Healthy — not Active, so session details should not be set.
	gs.Object["status"].(map[string]interface{})["state"] = "StandingBy"
	gs.Object["status"].(map[string]interface{})["health"] = "Healthy"

	n.gameServerCreatedOrUpdated(gs)

	val, ok := n.gameServerMap.Load(testGameServerName)
	assert.True(t, ok)
	gsi := val.(*GameServerInfo)
	gsi.Mutex.RLock()
	defer gsi.Mutex.RUnlock()
	assert.False(t, gsi.IsActive)
	assert.Empty(t, gsi.SessionID)
}

func TestUnitGameServerCreatedOrUpdated_ActiveHealthyTriggersSessionDetails(t *testing.T) {
	dynamicClient := newDynamicInterfaceWithDetails()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	gs.Object["status"].(map[string]interface{})["state"] = "Active"
	gs.Object["status"].(map[string]interface{})["health"] = "Healthy"
	gs.Object["status"].(map[string]interface{})["sessionID"] = "session-123"
	gs.Object["status"].(map[string]interface{})["sessionCookie"] = "cookie-456"
	gs.Object["status"].(map[string]interface{})["initialPlayers"] = []interface{}{"p1", "p2"}

	n.gameServerCreatedOrUpdated(gs)

	val, ok := n.gameServerMap.Load(testGameServerName)
	require.True(t, ok)
	gsi := val.(*GameServerInfo)
	gsi.Mutex.RLock()
	defer gsi.Mutex.RUnlock()
	assert.True(t, gsi.IsActive)
	assert.Equal(t, "session-123", gsi.SessionID)
	assert.Equal(t, "cookie-456", gsi.SessionCookie)
	assert.Equal(t, []string{"p1", "p2"}, gsi.InitialPlayers)

	// Verify that a GameServerDetail CR was created.
	u, err := dynamicClient.Resource(gameserverDetailGVR).Namespace(testGameServerNamespace).Get(context.Background(), testGameServerName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, testGameServerName, u.GetName())
}

// ---------- gameServerDeleted tests ----------

func TestUnitGameServerDeleted_RemovesFromMap(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	n.gameServerMap.Store(testGameServerName, &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	})

	n.gameServerDeleted(gs)
	_, ok := n.gameServerMap.Load(testGameServerName)
	assert.False(t, ok)
}

func TestUnitGameServerDeleted_NonExistent_NoOp(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer("nonexistent-gs", testGameServerNamespace)
	// Should not panic even though the GS is not in the map.
	assert.NotPanics(t, func() {
		n.gameServerDeleted(gs)
	})
}

func TestUnitGameServerDeleted_DeletedFinalStateUnknown(t *testing.T) {
	dynamicClient := newDynamicInterface()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	n.gameServerMap.Store(testGameServerName, &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	})

	wrapped := cache.DeletedFinalStateUnknown{
		Key: testGameServerNamespace + "/" + testGameServerName,
		Obj: gs,
	}

	assert.NotPanics(t, func() {
		n.gameServerDeleted(wrapped)
	})
	_, ok := n.gameServerMap.Load(testGameServerName)
	assert.False(t, ok)
}

// ---------- HeartbeatTimeChecker tests ----------

func TestUnitHeartbeatTimeChecker_RecentHeartbeat_NoMark(t *testing.T) {
	dynamicClient := newDynamicInterface()

	fixedTime := time.Date(2022, 4, 26, 10, 0, 0, 0, time.UTC)
	n := newTestNodeAgentManager(dynamicClient, func(n *NodeAgentManager) {
		n.nowFunc = func() time.Time { return fixedTime }
	})

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
	require.NoError(t, err)

	n.gameServerMap.Store(testGameServerName, &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		CreationTime:        fixedTime.UnixMilli(),
		LastHeartbeatTime:   fixedTime.UnixMilli(), // last heartbeat is now
		PreviousGameHealth:  "Healthy",
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	})

	n.HeartbeatTimeChecker()

	// Verify the game server health was NOT patched to Unhealthy.
	u, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Get(context.Background(), testGameServerName, metav1.GetOptions{})
	require.NoError(t, err)
	_, health, err := parseStateHealth(u)
	require.NoError(t, err)
	assert.NotEqual(t, "Unhealthy", health)
}

func TestUnitHeartbeatTimeChecker_AlreadyUnhealthy_NoMark(t *testing.T) {
	dynamicClient := newDynamicInterface()

	baseTime := time.Date(2022, 4, 26, 10, 0, 0, 0, time.UTC)
	laterTime := baseTime.Add(70 * time.Second) // well past firstHeartbeatTimeout
	n := newTestNodeAgentManager(dynamicClient, func(n *NodeAgentManager) {
		n.nowFunc = func() time.Time { return laterTime }
	})

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
	_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
	require.NoError(t, err)

	n.gameServerMap.Store(testGameServerName, &GameServerInfo{
		GameServerNamespace: testGameServerNamespace,
		CreationTime:        baseTime.UnixMilli(),
		LastHeartbeatTime:   0, // never sent a heartbeat
		PreviousGameHealth:  "Unhealthy",
		Mutex:               &sync.RWMutex{},
		BuildName:           testBuildName,
	})

	n.HeartbeatTimeChecker()

	// The server's PreviousGameHealth is already Unhealthy, so the checker
	// should skip it. The MarkedUnhealthy flag should remain false since no
	// patch was sent.
	gsdi, ok := n.gameServerMap.Load(testGameServerName)
	require.True(t, ok)
	gsi := gsdi.(*GameServerInfo)
	gsi.Mutex.RLock()
	defer gsi.Mutex.RUnlock()
	assert.False(t, gsi.MarkedUnhealthy)
}

// ---------- Additional edge-case tests ----------

func TestUnitUpdateConnectedPlayers_ZeroPlayers(t *testing.T) {
	dynamicClient := newDynamicInterfaceWithDetails()
	n := newTestNodeAgentManager(dynamicClient)

	// Create a GameServerDetail so the patch has a target.
	err := n.createGameServerDetails(context.Background(), "test-uid", testGameServerName, testGameServerNamespace, testBuildName, nil)
	require.NoError(t, err)

	gsd := &GameServerInfo{
		GameServerNamespace:   testGameServerNamespace,
		ConnectedPlayersCount: 2,
		Mutex:                 &sync.RWMutex{},
		BuildName:             testBuildName,
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateActive,
		CurrentGameHealth: "Healthy",
		CurrentPlayers:    []ConnectedPlayer{}, // 0 players
	}
	err = n.updateConnectedPlayersIfNeeded(context.Background(), hb, testGameServerName, gsd)
	assert.NoError(t, err)
	assert.Equal(t, 0, gsd.ConnectedPlayersCount)
}

func TestUnitGameServerCreatedOrUpdated_ExistingServerUpdate(t *testing.T) {
	dynamicClient := newDynamicInterfaceWithDetails()
	n := newTestNodeAgentManager(dynamicClient)

	gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

	// First call: create.
	n.gameServerCreatedOrUpdated(gs)
	val, ok := n.gameServerMap.Load(testGameServerName)
	require.True(t, ok)
	creationTime := val.(*GameServerInfo).CreationTime

	// Second call with same name: should not overwrite the existing entry.
	n.gameServerCreatedOrUpdated(gs)
	val2, ok2 := n.gameServerMap.Load(testGameServerName)
	require.True(t, ok2)
	assert.Equal(t, creationTime, val2.(*GameServerInfo).CreationTime)
}

// ---------- Table-driven heartbeatHandler tests ----------

func TestUnitHeartbeatHandler_TableDriven(t *testing.T) {
	tests := []struct {
		name              string
		gsExists          bool
		isActive          bool
		prevState         GameState
		prevHealth        string
		hbState           GameState
		hbHealth          string
		expectedStatus    int
		expectedOperation GameOperation
	}{
		{
			name:           "non-existent game server",
			gsExists:       false,
			hbState:        GameStateStandingBy,
			hbHealth:       "Healthy",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:              "standingBy heartbeat, not active",
			gsExists:          true,
			isActive:          false,
			prevState:         "",
			prevHealth:        "",
			hbState:           GameStateStandingBy,
			hbHealth:          "Healthy",
			expectedStatus:    http.StatusOK,
			expectedOperation: GameOperationContinue,
		},
		{
			name:              "standingBy heartbeat, server is active => Active op",
			gsExists:          true,
			isActive:          true,
			prevState:         GameStateStandingBy,
			prevHealth:        "Healthy",
			hbState:           GameStateStandingBy,
			hbHealth:          "Healthy",
			expectedStatus:    http.StatusOK,
			expectedOperation: GameOperationActive,
		},
		{
			name:              "active heartbeat, server is active => Continue",
			gsExists:          true,
			isActive:          true,
			prevState:         GameStateActive,
			prevHealth:        "Healthy",
			hbState:           GameStateActive,
			hbHealth:          "Healthy",
			expectedStatus:    http.StatusOK,
			expectedOperation: GameOperationContinue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient := newDynamicInterface()
			n := newTestNodeAgentManager(dynamicClient)

			gsName := testGameServerName

			if tt.gsExists {
				gs := createUnstructuredTestGameServer(gsName, testGameServerNamespace)
				_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
				require.NoError(t, err)

				n.gameServerMap.Store(gsName, &GameServerInfo{
					GameServerNamespace: testGameServerNamespace,
					IsActive:            tt.isActive,
					PreviousGameState:   tt.prevState,
					PreviousGameHealth:  tt.prevHealth,
					Mutex:               &sync.RWMutex{},
					BuildName:           testBuildName,
				})
			}

			hb := &HeartbeatRequest{
				CurrentGameState:  tt.hbState,
				CurrentGameHealth: tt.hbHealth,
			}
			w, hbr, _ := sendHeartbeat(t, n, gsName, hb)
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, tt.expectedOperation, hbr.Operation)
			}
		})
	}
}

// ---------- Table-driven updateHealthAndStateIfNeeded tests ----------

func TestUnitUpdateHealthAndState_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		prevState   GameState
		prevHealth  string
		newState    GameState
		newHealth   string
		expectError bool
	}{
		{
			name:       "no change",
			prevState:  GameStateStandingBy,
			prevHealth: "Healthy",
			newState:   GameStateStandingBy,
			newHealth:  "Healthy",
		},
		{
			name:      "empty to Initializing",
			prevState: "",
			newState:  GameStateInitializing,
			newHealth: "Healthy",
		},
		{
			name:       "empty to StandingBy",
			prevState:  "",
			prevHealth: "",
			newState:   GameStateStandingBy,
			newHealth:  "Healthy",
		},
		{
			name:       "Initializing to StandingBy",
			prevState:  GameStateInitializing,
			prevHealth: "Healthy",
			newState:   GameStateStandingBy,
			newHealth:  "Healthy",
		},
		{
			name:        "invalid: StandingBy to Initializing",
			prevState:   GameStateStandingBy,
			prevHealth:  "Healthy",
			newState:    GameStateInitializing,
			newHealth:   "Healthy",
			expectError: true,
		},
		{
			name:        "invalid: Active to Initializing",
			prevState:   GameStateActive,
			prevHealth:  "Healthy",
			newState:    GameStateInitializing,
			newHealth:   "Healthy",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient := newDynamicInterface()
			n := newTestNodeAgentManager(dynamicClient)

			gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
			_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
			require.NoError(t, err)

			gsd := &GameServerInfo{
				GameServerNamespace: testGameServerNamespace,
				PreviousGameState:   tt.prevState,
				PreviousGameHealth:  tt.prevHealth,
				Mutex:               &sync.RWMutex{},
				BuildName:           testBuildName,
			}

			hb := &HeartbeatRequest{
				CurrentGameState:  tt.newState,
				CurrentGameHealth: tt.newHealth,
			}
			err = n.updateHealthAndStateIfNeeded(context.Background(), hb, testGameServerName, gsd)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------- HeartbeatTimeChecker table-driven tests ----------

func TestUnitHeartbeatTimeChecker_TableDriven(t *testing.T) {
	baseTime := time.Date(2022, 4, 26, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		creationTime      int64
		lastHeartbeatTime int64
		prevHealth        string
		markedUnhealthy   bool
		currentTime       time.Time
		expectMarked      bool
	}{
		{
			name:              "recent heartbeat — no mark",
			creationTime:      baseTime.UnixMilli(),
			lastHeartbeatTime: baseTime.UnixMilli(),
			prevHealth:        "Healthy",
			currentTime:       baseTime.Add(2 * time.Second),
			expectMarked:      false,
		},
		{
			name:              "already unhealthy — no mark",
			creationTime:      baseTime.UnixMilli(),
			lastHeartbeatTime: 0,
			prevHealth:        "Unhealthy",
			currentTime:       baseTime.Add(70 * time.Second),
			expectMarked:      false,
		},
		{
			name:              "already marked unhealthy — no double mark",
			creationTime:      baseTime.UnixMilli(),
			lastHeartbeatTime: baseTime.UnixMilli(),
			prevHealth:        "Healthy",
			markedUnhealthy:   true,
			currentTime:       baseTime.Add(10 * time.Second),
			expectMarked:      true, // stays true, no additional patch
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient := newDynamicInterface()
			n := newTestNodeAgentManager(dynamicClient, func(n *NodeAgentManager) {
				n.nowFunc = func() time.Time { return tt.currentTime }
			})

			gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)
			_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
			require.NoError(t, err)

			n.gameServerMap.Store(testGameServerName, &GameServerInfo{
				GameServerNamespace: testGameServerNamespace,
				CreationTime:        tt.creationTime,
				LastHeartbeatTime:   tt.lastHeartbeatTime,
				PreviousGameHealth:  tt.prevHealth,
				MarkedUnhealthy:     tt.markedUnhealthy,
				Mutex:               &sync.RWMutex{},
				BuildName:           testBuildName,
			})

			n.HeartbeatTimeChecker()

			gsdi, ok := n.gameServerMap.Load(testGameServerName)
			require.True(t, ok)
			gsi := gsdi.(*GameServerInfo)
			gsi.Mutex.RLock()
			defer gsi.Mutex.RUnlock()
			assert.Equal(t, tt.expectMarked, gsi.MarkedUnhealthy)
		})
	}
}
