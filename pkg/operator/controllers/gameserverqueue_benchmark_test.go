package controllers

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkGameServerQueuePushToQueue(b *testing.B) {
	q := NewGameServersQueue()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs := &GameServerForQueue{
			Name:      fmt.Sprintf("gs-%d", i),
			Namespace: "default",
			BuildID:   "build-1",
			NodeAge:   i % 100,
		}
		q.PushToQueue(gs)
	}
}

func BenchmarkGameServerQueuePopFromQueue(b *testing.B) {
	q := NewGameServersQueue()
	// pre-populate the queue
	for i := 0; i < b.N; i++ {
		q.PushToQueue(&GameServerForQueue{
			Name:      fmt.Sprintf("gs-%d", i),
			Namespace: "default",
			BuildID:   "build-1",
			NodeAge:   i % 100,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PopFromQueue("build-1")
	}
}

func BenchmarkGameServerQueuePushAndPop(b *testing.B) {
	q := NewGameServersQueue()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs := &GameServerForQueue{
			Name:      fmt.Sprintf("gs-%d", i),
			Namespace: "default",
			BuildID:   "build-1",
			NodeAge:   i % 100,
		}
		q.PushToQueue(gs)
		q.PopFromQueue("build-1")
	}
}

func BenchmarkGameServerQueueRemoveFromQueue(b *testing.B) {
	q := NewGameServersQueue()
	for i := 0; i < b.N; i++ {
		q.PushToQueue(&GameServerForQueue{
			Name:      fmt.Sprintf("gs-%d", i),
			Namespace: "default",
			BuildID:   "build-1",
			NodeAge:   i % 100,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.RemoveFromQueue("default", fmt.Sprintf("gs-%d", i))
	}
}

func BenchmarkGameServerQueueMultipleBuilds(b *testing.B) {
	q := NewGameServersQueue()
	buildCount := 10
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildID := fmt.Sprintf("build-%d", i%buildCount)
		gs := &GameServerForQueue{
			Name:      fmt.Sprintf("gs-%d", i),
			Namespace: "default",
			BuildID:   buildID,
			NodeAge:   i % 100,
		}
		q.PushToQueue(gs)
	}
	// pop all
	for i := 0; i < buildCount; i++ {
		buildID := fmt.Sprintf("build-%d", i)
		for q.PopFromQueue(buildID) != nil {
		}
	}
}

func BenchmarkGameServerQueueConcurrentPushPop(b *testing.B) {
	q := NewGameServersQueue()
	var goroutineCounter atomic.Int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine uses its own build ID to avoid a known concurrent
		// push/pop race in the queue implementation where PopFromQueue can
		// delete a build entry between PushToQueue's existence check and its
		// final lock acquisition.
		gID := goroutineCounter.Add(1)
		localBuildID := fmt.Sprintf("build-g%d", gID)
		i := 0
		for pb.Next() {
			name := fmt.Sprintf("gs-p-%d-%d", gID, i)
			i++
			gs := &GameServerForQueue{
				Name:      name,
				Namespace: "default",
				BuildID:   localBuildID,
				NodeAge:   i % 100,
			}
			q.PushToQueue(gs)
			q.PopFromQueue(localBuildID)
		}
	})
}
