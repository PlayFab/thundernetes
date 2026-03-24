package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// benchNewNodeAgentManager creates a NodeAgentManager for benchmarking without the heartbeat time checker loop
func benchNewNodeAgentManager(b *testing.B) (*NodeAgentManager, func(name string)) {
	b.Helper()
	dynamicClient := newDynamicInterface()
	n := NewNodeAgentManager(dynamicClient, testNodeName, false, false, time.Now, false)

	registerGS := func(name string) {
		gs := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "mps.playfab.com/v1alpha1",
			"kind":       "GameServer",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": testGameServerNamespace,
				"labels": map[string]interface{}{
					"NodeName":  testNodeName,
					"BuildName": testBuildName,
				},
			},
			"spec": map[string]interface{}{
				"titleID": "testTitleID",
				"buildID": "testBuildID",
				"portsToExpose": []interface{}{
					"80",
				},
			},
			"status": map[string]interface{}{
				"health": "",
				"state":  "",
			},
		}}

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(
			nil, gs, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create game server: %v", err)
		}

		// Wait for the watch to pick it up
		for attempts := 0; attempts < 1000; attempts++ {
			if _, ok := n.gameServerMap.Load(name); ok {
				return
			}
			time.Sleep(time.Millisecond)
		}
		b.Fatalf("game server %s not found in map after creation", name)
	}

	return n, registerGS
}

func BenchmarkHeartbeatHandler(b *testing.B) {
	n, registerGS := benchNewNodeAgentManager(b)
	gsName := "bench-gs"
	registerGS(gsName)

	// Pre-set state so heartbeat processing doesn't trigger K8s patches
	gsi, _ := n.gameServerMap.Load(gsName)
	info := gsi.(*GameServerInfo)
	info.PreviousGameState = GameStateStandingBy
	info.PreviousGameHealth = "Healthy"

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateStandingBy,
		CurrentGameHealth: "Healthy",
	}
	payload, _ := json.Marshal(hb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", gsName), bytes.NewReader(payload))
		w := httptest.NewRecorder()
		n.heartbeatHandler(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status code: %d", w.Code)
		}
	}
}

func BenchmarkHeartbeatHandlerWithStateChange(b *testing.B) {
	n, registerGS := benchNewNodeAgentManager(b)
	gsName := "bench-gs-state"
	registerGS(gsName)

	gsi, _ := n.gameServerMap.Load(gsName)
	info := gsi.(*GameServerInfo)
	info.PreviousGameState = ""
	info.PreviousGameHealth = ""

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateStandingBy,
		CurrentGameHealth: "Healthy",
	}
	payload, _ := json.Marshal(hb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset state each iteration to trigger the update path
		info.PreviousGameState = ""
		info.PreviousGameHealth = ""

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", gsName), bytes.NewReader(payload))
		w := httptest.NewRecorder()
		n.heartbeatHandler(w, req)
	}
}

func BenchmarkHeartbeatHandlerConcurrent(b *testing.B) {
	n, registerGS := benchNewNodeAgentManager(b)

	// Register multiple game servers
	const gsCount = 10
	gsNames := make([]string, gsCount)
	for i := 0; i < gsCount; i++ {
		gsNames[i] = fmt.Sprintf("bench-gs-concurrent-%d", i)
		registerGS(gsNames[i])
		gsi, _ := n.gameServerMap.Load(gsNames[i])
		info := gsi.(*GameServerInfo)
		info.PreviousGameState = GameStateStandingBy
		info.PreviousGameHealth = "Healthy"
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateStandingBy,
		CurrentGameHealth: "Healthy",
	}
	payload, _ := json.Marshal(hb)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			gsName := gsNames[i%gsCount]
			i++
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", gsName), bytes.NewReader(payload))
			w := httptest.NewRecorder()
			n.heartbeatHandler(w, req)
		}
	})
}

func BenchmarkHeartbeatTimeChecker(b *testing.B) {
	dynamicClient := newDynamicInterface()
	n := NewNodeAgentManager(dynamicClient, testNodeName, false, false, time.Now, false)

	// Register 100 game servers
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("bench-gs-checker-%d", i)
		gs := createUnstructuredTestGameServer(name, testGameServerNamespace)
		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(
			nil, gs, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create game server: %v", err)
		}
		// Wait for watch
		for attempts := 0; attempts < 1000; attempts++ {
			if _, ok := n.gameServerMap.Load(name); ok {
				break
			}
			time.Sleep(time.Millisecond)
		}
		// Set heartbeat time so they're not marked unhealthy
		gsi, _ := n.gameServerMap.Load(name)
		info := gsi.(*GameServerInfo)
		info.Mutex.Lock()
		info.LastHeartbeatTime = time.Now().UnixMilli()
		info.GsUid = types.UID(fmt.Sprintf("uid-%d", i))
		info.Mutex.Unlock()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.HeartbeatTimeChecker()
	}
}

func BenchmarkHeartbeatHandlerManyGameServers(b *testing.B) {
	n, registerGS := benchNewNodeAgentManager(b)

	// Register 100 game servers
	const gsCount = 100
	gsNames := make([]string, gsCount)
	for i := 0; i < gsCount; i++ {
		gsNames[i] = fmt.Sprintf("bench-gs-many-%d", i)
		registerGS(gsNames[i])
		gsi, _ := n.gameServerMap.Load(gsNames[i])
		info := gsi.(*GameServerInfo)
		info.PreviousGameState = GameStateStandingBy
		info.PreviousGameHealth = "Healthy"
	}

	hb := &HeartbeatRequest{
		CurrentGameState:  GameStateStandingBy,
		CurrentGameHealth: "Healthy",
	}
	payload, _ := json.Marshal(hb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gsName := gsNames[i%gsCount]
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", gsName), bytes.NewReader(payload))
		w := httptest.NewRecorder()
		n.heartbeatHandler(w, req)
	}
}

// TestHeartbeatConcurrencyStress tests for race conditions in heartbeat processing
// under high concurrency with many distinct game servers (one goroutine per server).
// The nodeagent design assumes heartbeats for a single game server are serial
// (from a single game process), so this test uses unique game servers per goroutine.
func TestHeartbeatConcurrencyStress(t *testing.T) {
	dynamicClient := newDynamicInterface()
	// Disable heartbeat time checker and watch - we'll populate the map directly
	n := &NodeAgentManager{
		dynamicClient:             dynamicClient,
		watchStopper:              make(chan struct{}),
		gameServerMap:             &sync.Map{},
		nodeName:                  testNodeName,
		logEveryHeartbeat:         false,
		ignoreHealthFromHeartbeat: false,
		nowFunc:                   time.Now,
		heartbeatTimeout:          5000,
		firstHeartbeatTimeout:     60000,
	}

	const gsCount = 50

	// Directly populate the gameServerMap (bypassing the watch)
	for i := 0; i < gsCount; i++ {
		name := fmt.Sprintf("stress-gs-%d", i)
		n.gameServerMap.Store(name, &GameServerInfo{
			GameServerNamespace: testGameServerNamespace,
			Mutex:               &sync.RWMutex{},
			CreationTime:        time.Now().UnixMilli(),
			BuildName:           testBuildName,
			PreviousGameState:   GameStateStandingBy,
			PreviousGameHealth:  "Healthy",
		})
	}

	// Fire concurrent heartbeats - each goroutine owns its own game server
	var wg sync.WaitGroup
	for i := 0; i < gsCount; i++ {
		wg.Add(1)
		go func(gsName string) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				hb := &HeartbeatRequest{
					CurrentGameState:  GameStateStandingBy,
					CurrentGameHealth: "Healthy",
				}
				payload, _ := json.Marshal(hb)
				req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", gsName), bytes.NewReader(payload))
				w := httptest.NewRecorder()
				n.heartbeatHandler(w, req)
				if w.Code != http.StatusOK {
					t.Errorf("unexpected status code %d for %s", w.Code, gsName)
				}
			}
		}(fmt.Sprintf("stress-gs-%d", i))
	}

	wg.Wait()
}
