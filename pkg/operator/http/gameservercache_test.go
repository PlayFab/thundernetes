package http

import (
	"container/heap"
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("gameservercache tests", func() {
	It("should add a game server per build", func() {
		c := NewGameServersCache()
		for i := 0; i < 4; i++ {
			gs := createGameServerForCacheTest(fmt.Sprintf("gs-%d", i), fmt.Sprintf("ns-%d", i), fmt.Sprintf("build-%d", i), 0)
			c.PushToCache(gs)
		}
		Expect(len(c.heapsPerBuilds)).To(Equal(4))
		Expect(len(c.nameNamespaceToBuildIdMap)).To(Equal(4))

		for i := 0; i < 4; i++ {
			Expect(len(c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].gameServerNameSet)).To(Equal(1))
			Expect(len(*c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].heap)).To(Equal(1))
			Expect((*c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].heap)[0].Name).To(Equal(fmt.Sprintf("gs-%d", i)))
		}

		for i := 0; i < 4; i++ {
			gs := c.PopFromCache(fmt.Sprintf("build-%d", i))
			Expect(gs.Name).To(Equal(fmt.Sprintf("gs-%d", i)))
			Expect(gs.Namespace).To(Equal(fmt.Sprintf("ns-%d", i)))
		}

		// gs should be nil
		for i := 0; i < 4; i++ {
			gs := c.PopFromCache(fmt.Sprintf("build-%d", i))
			Expect(gs).To(BeNil())
		}
	})
	It("should add multiple game servers per Build", func() {
		c := NewGameServersCache()
		for i := 0; i < 4; i++ {
			for j := 0; j < 5; j++ {
				gs := createGameServerForCacheTest(fmt.Sprintf("gs-%d-%d", i, j), fmt.Sprintf("ns-%d", i), fmt.Sprintf("build-%d", i), j)
				c.PushToCache(gs)
			}
		}
		Expect(len(c.heapsPerBuilds)).To(Equal(4))
		Expect(len(c.nameNamespaceToBuildIdMap)).To(Equal(20))

		for i := 0; i < 4; i++ {
			Expect(len(c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].gameServerNameSet)).To(Equal(5))
			Expect(len(*c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].heap)).To(Equal(5))
			for j := 0; j < 5; j++ {
				Expect((*c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].heap)[j].Name).To(Equal(fmt.Sprintf("gs-%d-%d", i, j)))
			}
		}

		for i := 0; i < 4; i++ {
			for j := 0; j < 5; j++ {
				gs := c.PopFromCache(fmt.Sprintf("build-%d", i))
				Expect(gs.Name).To(Equal(fmt.Sprintf("gs-%d-%d", i, j)))
				Expect(gs.Namespace).To(Equal(fmt.Sprintf("ns-%d", i)))
			}
		}
	})
	It("should work with deleting game server", func() {
		c := GameServerHeapForBuild{
			heap: &GameServerHeap{},
		}
		gsArray := GenerateGameServersForTest()
		for _, gs := range gsArray {
			c.heap.PushToHeap(gs)
		}

		gs := c.heap.PopFromHeap()
		Expect(gs.NodeAge).To(Equal(1))
		Expect(gs.Name).To(Equal("gs-1"))

		DeleteGameServerWithName("gs-2", c.heap)
		DeleteGameServerWithName("gs-2", c.heap)
		DeleteGameServerWithName("gs-3", c.heap)
		DeleteGameServerWithName("gs-3", c.heap)

		gs = c.heap.PopFromHeap()
		Expect(gs.NodeAge).To(Equal(4))
		Expect(gs.Name).To(Equal("gs-4"))

	})
	It("should work with simultaneous requests", func() {
		c := NewGameServersCache()
		wg := &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				gs := createGameServerForCacheTest(fmt.Sprintf("gs-%d", i), "ns", "buildId", i)
				c.PushToCache(gs)
			}(i)
		}
		wg.Wait()
		Expect(len(*c.heapsPerBuilds["buildId"].heap)).To(Equal(100))

		wg = &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_ = c.PopFromCache("buildId")
			}(i)
		}
		wg.Wait()
		Expect(len(*c.heapsPerBuilds["buildId"].heap)).To(Equal(0))
	})
})

func createGameServerForCacheTest(name, namespace, buildID string, nodeAge int) *GameServerForHeap {
	return &GameServerForHeap{
		Name:      name,
		Namespace: namespace,
		BuildID:   buildID,
		NodeAge:   nodeAge,
	}
}

func DeleteGameServerWithName(name string, h *GameServerHeap) {
	for i, gs := range *h {
		if gs.Name == name {
			heap.Remove(h, i)
			return
		}
	}
}

func GenerateGameServersForTest() []*GameServerForHeap {
	nums := []int{3, 2, 20, 5, 3, 1, 2, 5, 6, 9, 10, 4}
	var gameServers []*GameServerForHeap
	for _, i := range nums {
		gs := GameServerForHeap{
			Name:    fmt.Sprintf("gs-%d", i),
			NodeAge: i,
		}
		gameServers = append(gameServers, &gs)
	}
	return gameServers
}
