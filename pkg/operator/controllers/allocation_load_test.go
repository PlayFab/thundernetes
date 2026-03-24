package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newTestK8sClientForLoadTest creates a fake k8s client with the mpsv1alpha1 scheme registered.
// This is needed for standalone tests that don't run through the Ginkgo test suite.
func newTestK8sClientForLoadTest() client.Client {
	_ = mpsv1alpha1.AddToScheme(scheme.Scheme)
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme)
	return cb.WithStatusSubresource(&mpsv1alpha1.GameServer{}).WithIndex(&mpsv1alpha1.GameServer{}, statusSessionId, func(rawObj client.Object) []string {
		gs := rawObj.(*mpsv1alpha1.GameServer)
		return []string{gs.Status.SessionID}
	}).WithIndex(&mpsv1alpha1.GameServerBuild{}, specBuildId, func(rawObj client.Object) []string {
		gsb := rawObj.(*mpsv1alpha1.GameServerBuild)
		return []string{gsb.Spec.BuildID}
	}).Build()
}

// BenchmarkAllocationHandlerThroughput benchmarks the allocation request handler
// with a pre-populated queue of game servers
func BenchmarkAllocationHandlerThroughput(b *testing.B) {
	buildID := uuid.New().String()
	buildName := "bench-build"
	cl := newTestK8sClientForLoadTest()

	// Create a GameServerBuild
	gsb := mpsv1alpha1.GameServerBuild{}
	gsb.Name = buildName
	gsb.Namespace = "default"
	gsb.Spec.BuildID = buildID
	if err := cl.Create(nil, &gsb); err != nil {
		b.Fatalf("failed to create GameServerBuild: %v", err)
	}

	h := NewAllocationApiServer(nil, nil, cl, allocationApiSvcPort)
	h.gameServerQueue = NewGameServersQueue()

	// Pre-populate queue and create corresponding GameServer objects
	for i := 0; i < b.N; i++ {
		gsName := fmt.Sprintf("gs-%d", i)
		gs := &mpsv1alpha1.GameServer{}
		gs.Name = gsName
		gs.Namespace = "default"
		gs.Labels = map[string]string{
			LabelBuildID:   buildID,
			LabelBuildName: buildName,
		}
		gs.Status.State = mpsv1alpha1.GameServerStateStandingBy
		gs.Status.Health = mpsv1alpha1.GameServerHealthy
		if err := cl.Create(nil, gs); err != nil {
			b.Fatalf("failed to create GameServer: %v", err)
		}

		h.gameServerQueue.PushToQueue(&GameServerForQueue{
			Name:            gsName,
			Namespace:       "default",
			BuildID:         buildID,
			NodeAge:         i % 10,
			ResourceVersion: "1",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := uuid.New().String()
		args := AllocateArgs{
			SessionID: sessionID,
			BuildID:   buildID,
		}
		body, _ := json.Marshal(args)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.handleAllocationRequest(w, req)
	}
}

// TestAllocationThroughputLoadTest exercises the allocation handler with many
// concurrent allocation requests to verify correctness under load
func TestAllocationThroughputLoadTest(t *testing.T) {
	buildID := uuid.New().String()
	buildName := "load-test-build"
	cl := newTestK8sClientForLoadTest()

	// Create GameServerBuild
	gsb := mpsv1alpha1.GameServerBuild{}
	gsb.Name = buildName
	gsb.Namespace = "default"
	gsb.Spec.BuildID = buildID
	if err := cl.Create(nil, &gsb); err != nil {
		t.Fatalf("failed to create GameServerBuild: %v", err)
	}

	h := NewAllocationApiServer(nil, nil, cl, allocationApiSvcPort)
	h.gameServerQueue = NewGameServersQueue()

	// Create 200 game servers
	const totalGameServers = 200
	for i := 0; i < totalGameServers; i++ {
		gsName := fmt.Sprintf("load-gs-%d", i)
		gs := &mpsv1alpha1.GameServer{}
		gs.Name = gsName
		gs.Namespace = "default"
		gs.Labels = map[string]string{
			LabelBuildID:   buildID,
			LabelBuildName: buildName,
		}
		gs.Status.State = mpsv1alpha1.GameServerStateStandingBy
		gs.Status.Health = mpsv1alpha1.GameServerHealthy
		if err := cl.Create(nil, gs); err != nil {
			t.Fatalf("failed to create GameServer: %v", err)
		}
		h.gameServerQueue.PushToQueue(&GameServerForQueue{
			Name:            gsName,
			Namespace:       "default",
			BuildID:         buildID,
			NodeAge:         i % 10,
			ResourceVersion: "1",
		})
	}

	// Fire concurrent allocation requests
	const concurrency = 20
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var failCount atomic.Int64

	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < totalGameServers/concurrency; i++ {
				sessionID := uuid.New().String()
				args := AllocateArgs{
					SessionID: sessionID,
					BuildID:   buildID,
				}
				body, _ := json.Marshal(args)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewReader(body))
				w := httptest.NewRecorder()
				h.handleAllocationRequest(w, req)

				if w.Code == http.StatusOK {
					successCount.Add(1)
				} else {
					failCount.Add(1)
				}
			}
		}(c)
	}

	wg.Wait()

	t.Logf("Allocation load test results: %d successful, %d failed out of %d total requests",
		successCount.Load(), failCount.Load(), totalGameServers)

	// We should have at least some successful allocations
	if successCount.Load() == 0 {
		t.Error("expected at least some successful allocations, got 0")
	}
}

// TestAllocationConcurrentMultipleBuilds tests concurrent allocations across multiple builds
func TestAllocationConcurrentMultipleBuilds(t *testing.T) {
	const buildCount = 5
	const gsPerBuild = 20
	const concurrency = 10

	cl := newTestK8sClientForLoadTest()
	h := NewAllocationApiServer(nil, nil, cl, allocationApiSvcPort)
	h.gameServerQueue = NewGameServersQueue()

	buildIDs := make([]string, buildCount)
	for b := 0; b < buildCount; b++ {
		buildIDs[b] = uuid.New().String()
		buildName := fmt.Sprintf("multi-build-%d", b)

		gsb := mpsv1alpha1.GameServerBuild{}
		gsb.Name = buildName
		gsb.Namespace = "default"
		gsb.Spec.BuildID = buildIDs[b]
		if err := cl.Create(nil, &gsb); err != nil {
			t.Fatalf("failed to create GameServerBuild: %v", err)
		}

		for i := 0; i < gsPerBuild; i++ {
			gsName := fmt.Sprintf("multi-gs-%d-%d", b, i)
			gs := &mpsv1alpha1.GameServer{}
			gs.Name = gsName
			gs.Namespace = "default"
			gs.Labels = map[string]string{
				LabelBuildID:   buildIDs[b],
				LabelBuildName: buildName,
			}
			gs.Status.State = mpsv1alpha1.GameServerStateStandingBy
			gs.Status.Health = mpsv1alpha1.GameServerHealthy
			if err := cl.Create(nil, gs); err != nil {
				t.Fatalf("failed to create GameServer: %v", err)
			}
			h.gameServerQueue.PushToQueue(&GameServerForQueue{
				Name:            gsName,
				Namespace:       "default",
				BuildID:         buildIDs[b],
				NodeAge:         i,
				ResourceVersion: "1",
			})
		}
	}

	var wg sync.WaitGroup
	var totalSuccess atomic.Int64

	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < (buildCount * gsPerBuild / concurrency); i++ {
				buildID := buildIDs[i%buildCount]
				sessionID := uuid.New().String()
				args := AllocateArgs{
					SessionID: sessionID,
					BuildID:   buildID,
				}
				body, _ := json.Marshal(args)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewReader(body))
				w := httptest.NewRecorder()
				h.handleAllocationRequest(w, req)
				if w.Code == http.StatusOK {
					totalSuccess.Add(1)
				}
			}
		}(c)
	}

	wg.Wait()
	t.Logf("Multi-build allocation: %d successful allocations across %d builds", totalSuccess.Load(), buildCount)
}
