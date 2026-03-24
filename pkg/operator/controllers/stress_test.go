package controllers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// ============================================================================
// Port Registry Concurrency Stress Tests
// ============================================================================

// TestPortRegistryConcurrentAllocDealloc verifies that concurrent allocations
// and deallocations don't cause data races or inconsistent state
func TestPortRegistryConcurrentAllocDealloc(t *testing.T) {
	log := logr.FromContextOrDiscard(context.Background())
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "stress-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node).Build()

	// 5 nodes × 100 ports = 500 free ports
	pr, err := NewPortRegistry(kubeClient, &mpsv1alpha1.GameServerList{}, 20000, 20099, 5, false, log)
	if err != nil {
		t.Fatalf("failed to create PortRegistry: %v", err)
	}

	const goroutines = 50
	const opsPerGoroutine = 20
	var wg sync.WaitGroup
	var allocSuccess, allocFail, deallocSuccess atomic.Int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				name := fmt.Sprintf("gs-%d-%d", gID, i)
				ns := "default"

				// Allocate
				_, err := pr.GetNewPorts(ns, name, 1)
				if err != nil {
					allocFail.Add(1)
					continue
				}
				allocSuccess.Add(1)

				// Deallocate
				_, err = pr.DeregisterPorts(ns, name)
				if err != nil {
					t.Errorf("deregister failed for %s: %v", name, err)
				} else {
					deallocSuccess.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()

	t.Logf("Port registry stress: alloc success=%d, alloc fail=%d, dealloc success=%d",
		allocSuccess.Load(), allocFail.Load(), deallocSuccess.Load())

	// Verify final state: all ports should be free since we deallocated everything
	expectedFree := 5 * 100 // nodeCount * (max - min + 1)
	if pr.FreePortsCount != expectedFree {
		t.Errorf("expected %d free ports, got %d", expectedFree, pr.FreePortsCount)
	}

	if len(pr.HostPortsPerGameServer) != 0 {
		t.Errorf("expected 0 game servers in port map, got %d", len(pr.HostPortsPerGameServer))
	}
}

// TestPortRegistryConcurrentExhaustion tests behavior when many goroutines
// try to allocate from a nearly-exhausted port pool
func TestPortRegistryConcurrentExhaustion(t *testing.T) {
	log := logr.FromContextOrDiscard(context.Background())
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "exhaust-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node).Build()

	// Only 10 free ports with 1 node
	pr, err := NewPortRegistry(kubeClient, &mpsv1alpha1.GameServerList{}, 20000, 20009, 1, false, log)
	if err != nil {
		t.Fatalf("failed to create PortRegistry: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	var success, failure atomic.Int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			name := fmt.Sprintf("exhaust-gs-%d", gID)
			_, err := pr.GetNewPorts("default", name, 1)
			if err != nil {
				failure.Add(1)
			} else {
				success.Add(1)
			}
		}(g)
	}

	wg.Wait()

	t.Logf("Exhaustion test: %d successful, %d failed out of %d attempts (10 ports available)",
		success.Load(), failure.Load(), goroutines)

	// Exactly 10 should succeed
	if success.Load() != 10 {
		t.Errorf("expected exactly 10 successful allocations, got %d", success.Load())
	}
	if failure.Load() != 40 {
		t.Errorf("expected exactly 40 failures, got %d", failure.Load())
	}
}

// TestPortRegistryMultiPortConcurrency tests concurrent allocation of multiple
// ports per GameServer
func TestPortRegistryMultiPortConcurrency(t *testing.T) {
	log := logr.FromContextOrDiscard(context.Background())
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "multi-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node).Build()

	// 3 nodes × 50 ports = 150 total, allocating 3 ports each
	pr, err := NewPortRegistry(kubeClient, &mpsv1alpha1.GameServerList{}, 20000, 20049, 3, false, log)
	if err != nil {
		t.Fatalf("failed to create PortRegistry: %v", err)
	}

	const goroutines = 30
	var wg sync.WaitGroup
	var allocatedPorts sync.Map

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			name := fmt.Sprintf("multi-gs-%d", gID)
			ports, err := pr.GetNewPorts("default", name, 3)
			if err != nil {
				return // may run out of ports
			}
			allocatedPorts.Store(name, ports)
		}(g)
	}

	wg.Wait()

	// Verify no port appears more than nodeCount times
	portUsage := make(map[int32]int)
	allocatedPorts.Range(func(key, value interface{}) bool {
		ports := value.([]int32)
		for _, p := range ports {
			portUsage[p]++
		}
		return true
	})

	for port, count := range portUsage {
		if count > 3 { // nodeCount = 3
			t.Errorf("port %d used %d times, exceeds node count of 3", port, count)
		}
	}
}

// ============================================================================
// GameServer Queue Concurrency Stress Tests
// ============================================================================

// TestGameServerQueueConcurrentStress exercises the queue under heavy concurrent
// push, pop, and remove operations
func TestGameServerQueueConcurrentStress(t *testing.T) {
	q := NewGameServersQueue()

	const goroutines = 20
	const opsPerGoroutine = 100
	var wg sync.WaitGroup

	// Concurrent pushes
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				q.PushToQueue(&GameServerForQueue{
					Name:      fmt.Sprintf("stress-gs-%d-%d", gID, i),
					Namespace: "default",
					BuildID:   fmt.Sprintf("build-%d", gID%5),
					NodeAge:   i,
				})
			}
		}(g)
	}
	wg.Wait()

	// Concurrent pops
	var popCount atomic.Int64
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			buildID := fmt.Sprintf("build-%d", gID%5)
			for {
				gs := q.PopFromQueue(buildID)
				if gs == nil {
					return
				}
				popCount.Add(1)
			}
		}(g)
	}
	wg.Wait()

	t.Logf("Queue stress: popped %d items", popCount.Load())
}

// TestGameServerQueueConcurrentPushPopRemove tests mixed operations concurrently
func TestGameServerQueueConcurrentPushPopRemove(t *testing.T) {
	q := NewGameServersQueue()
	const buildID = "concurrent-build"
	var wg sync.WaitGroup

	// Push 500 items
	for i := 0; i < 500; i++ {
		q.PushToQueue(&GameServerForQueue{
			Name:      fmt.Sprintf("q-gs-%d", i),
			Namespace: "default",
			BuildID:   buildID,
			NodeAge:   i % 50,
		})
	}

	// Concurrently pop and remove
	var popCount, removeCount atomic.Int64

	// Poppers
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				if gs := q.PopFromQueue(buildID); gs != nil {
					popCount.Add(1)
				}
			}
		}()
	}

	// Removers
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := gID * 25; i < (gID+1)*25; i++ {
				q.RemoveFromQueue("default", fmt.Sprintf("q-gs-%d", i))
				removeCount.Add(1)
			}
		}(g)
	}

	wg.Wait()
	t.Logf("Mixed operations: %d pops, %d removes", popCount.Load(), removeCount.Load())
}

// ============================================================================
// Controller Reconciliation Stress Tests (unit level, using fake client)
// ============================================================================

// TestControllerReconciliationStressGameServerCreation stress tests GameServer
// creation by a GameServerBuild reconciler with a fake client
func TestControllerReconciliationStressGameServerCreation(t *testing.T) {
	log := logr.FromContextOrDiscard(context.Background())
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "reconcile-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node).Build()

	// Large port range for stress: 5 nodes × 500 ports = 2500
	pr, err := NewPortRegistry(kubeClient, &mpsv1alpha1.GameServerList{}, 20000, 20499, 5, false, log)
	if err != nil {
		t.Fatalf("failed to create PortRegistry: %v", err)
	}

	const buildCount = 10
	const gsPerBuild = 20
	var wg sync.WaitGroup
	var totalAllocated atomic.Int64

	// Simulate what the reconciler does: allocate ports for many GameServers
	for b := 0; b < buildCount; b++ {
		wg.Add(1)
		go func(buildIdx int) {
			defer wg.Done()
			for i := 0; i < gsPerBuild; i++ {
				name := fmt.Sprintf("reconcile-gs-%d-%d", buildIdx, i)
				_, err := pr.GetNewPorts("default", name, 1)
				if err != nil {
					t.Logf("allocation failed for %s: %v", name, err)
					continue
				}
				totalAllocated.Add(1)
			}
		}(b)
	}

	wg.Wait()

	t.Logf("Reconciliation stress: %d GameServers allocated ports out of %d total",
		totalAllocated.Load(), buildCount*gsPerBuild)

	expectedUsed := int(totalAllocated.Load())
	expectedFree := (5 * 500) - expectedUsed // nodeCount × portRange - usedPorts
	if pr.FreePortsCount != expectedFree {
		t.Errorf("expected %d free ports, got %d", expectedFree, pr.FreePortsCount)
	}
}

// TestControllerReconciliationRapidScaling simulates rapid scale up and down
func TestControllerReconciliationRapidScaling(t *testing.T) {
	log := logr.FromContextOrDiscard(context.Background())
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "scale-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node).Build()

	pr, err := NewPortRegistry(kubeClient, &mpsv1alpha1.GameServerList{}, 20000, 20499, 3, false, log)
	if err != nil {
		t.Fatalf("failed to create PortRegistry: %v", err)
	}

	// Simulate 10 rounds of scale up / scale down
	for round := 0; round < 10; round++ {
		gsNames := make([]string, 50)
		// Scale up: allocate 50 GameServers
		for i := 0; i < 50; i++ {
			gsNames[i] = fmt.Sprintf("scale-gs-r%d-%d", round, i)
			_, err := pr.GetNewPorts("default", gsNames[i], 1)
			if err != nil {
				t.Fatalf("round %d: allocation failed for %s: %v", round, gsNames[i], err)
			}
		}

		// Scale down: deallocate all
		for i := 0; i < 50; i++ {
			_, err := pr.DeregisterPorts("default", gsNames[i])
			if err != nil {
				t.Fatalf("round %d: deregister failed for %s: %v", round, gsNames[i], err)
			}
		}
	}

	// After all rounds, all ports should be free
	expectedFree := 3 * 500
	if pr.FreePortsCount != expectedFree {
		t.Errorf("after rapid scaling: expected %d free ports, got %d", expectedFree, pr.FreePortsCount)
	}
}

// TestConcurrentNodeChanges tests port registry behavior when node count changes
// concurrently with port allocations
func TestConcurrentNodeChanges(t *testing.T) {
	log := logr.FromContextOrDiscard(context.Background())
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-change-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node).Build()

	pr, err := NewPortRegistry(kubeClient, &mpsv1alpha1.GameServerList{}, 20000, 20049, 2, false, log)
	if err != nil {
		t.Fatalf("failed to create PortRegistry: %v", err)
	}

	var wg sync.WaitGroup

	// Goroutines allocating ports
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				name := fmt.Sprintf("node-change-gs-%d-%d", gID, i)
				pr.GetNewPorts("default", name, 1)
			}
		}(g)
	}

	// Goroutines simulating node additions/removals
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			pr.onNodeAdded()
			pr.onNodeRemoved()
		}
	}()

	wg.Wait()
	// The test passes if there are no panics or data races
	t.Logf("Concurrent node changes completed. NodeCount=%d, FreePortsCount=%d",
		pr.NodeCount, pr.FreePortsCount)
}
