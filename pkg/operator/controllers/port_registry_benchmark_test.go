package controllers

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newPortRegistryForBenchmark creates a PortRegistry for benchmarking (no Gomega dependency)
func newPortRegistryForBenchmark(b *testing.B, min, max int32, nodeCount int) *PortRegistry {
	b.Helper()
	log := logr.FromContextOrDiscard(context.Background())
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "bench-node"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node).Build()
	pr, err := NewPortRegistry(kubeClient, &mpsv1alpha1.GameServerList{}, min, max, nodeCount, false, log)
	if err != nil {
		b.Fatalf("failed to create PortRegistry: %v", err)
	}
	return pr
}

func BenchmarkPortRegistryGetNewPorts(b *testing.B) {
	// Port range 20000-20499 = 500 ports, 1 node = 500 free ports
	pr := newPortRegistryForBenchmark(b, 20000, 20499, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("gs-%d", i)
		_, err := pr.GetNewPorts("default", name, 1)
		if err != nil {
			// when we run out of ports, reset the registry
			b.StopTimer()
			pr = newPortRegistryForBenchmark(b, 20000, 20499, 1)
			b.StartTimer()
			continue
		}
	}
}

func BenchmarkPortRegistryGetNewPortsMultiple(b *testing.B) {
	pr := newPortRegistryForBenchmark(b, 20000, 20499, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("gs-%d", i)
		_, err := pr.GetNewPorts("default", name, 3)
		if err != nil {
			b.StopTimer()
			pr = newPortRegistryForBenchmark(b, 20000, 20499, 1)
			b.StartTimer()
			continue
		}
	}
}

func BenchmarkPortRegistryDeregisterPorts(b *testing.B) {
	pr := newPortRegistryForBenchmark(b, 20000, 20499, 1)
	// pre-allocate ports for all iterations
	names := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		names[i] = fmt.Sprintf("gs-%d", i)
		_, err := pr.GetNewPorts("default", names[i], 1)
		if err != nil {
			// not enough ports for the full run; reduce N conceptually by resetting
			pr = newPortRegistryForBenchmark(b, 20000, 20499, 1)
			i = -1 // restart (b.N may have changed)
			break
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pr.DeregisterPorts("default", names[i%len(names)])
	}
}

func BenchmarkPortRegistryAllocateAndDeallocate(b *testing.B) {
	pr := newPortRegistryForBenchmark(b, 20000, 20499, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("gs-%d", i)
		_, err := pr.GetNewPorts("default", name, 1)
		if err != nil {
			b.StopTimer()
			pr = newPortRegistryForBenchmark(b, 20000, 20499, 1)
			b.StartTimer()
			continue
		}
		pr.DeregisterPorts("default", name)
	}
}

func BenchmarkPortRegistryConcurrentAllocations(b *testing.B) {
	// 10 nodes × 500 ports = 5000 free ports
	pr := newPortRegistryForBenchmark(b, 20000, 20499, 10)
	var counter atomic.Int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := counter.Add(1)
			name := fmt.Sprintf("gs-parallel-%d", id)
			_, err := pr.GetNewPorts("default", name, 1)
			if err != nil {
				// out of ports, deregister and retry
				pr.DeregisterPorts("default", name)
			}
		}
	})
}
