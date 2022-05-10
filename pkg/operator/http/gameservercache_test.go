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

			_, exists := c.heapsPerBuilds[fmt.Sprintf("build-%d", i)]
			Expect(exists).To(BeFalse())
			_, exists = c.nameNamespaceToBuildIdMap[fmt.Sprintf("gs-%d", i)+fmt.Sprintf("ns-%d", i)]
			Expect(exists).To(BeFalse())
		}

		// next pops should be nil
		for i := 0; i < 4; i++ {
			gs := c.PopFromCache(fmt.Sprintf("build-%d", i))
			Expect(gs).To(BeNil())
		}
	})
	It("should add multiple game servers per Build", func() {
		c := NewGameServersCache()
		const totalBuilds = 4
		const gameServersPerBuild = 5
		for i := 0; i < totalBuilds; i++ {
			for j := 0; j < gameServersPerBuild; j++ {
				gs := createGameServerForCacheTest(fmt.Sprintf("gs-%d-%d", i, j), fmt.Sprintf("ns-%d", i), fmt.Sprintf("build-%d", i), j)
				c.PushToCache(gs)
			}
		}
		Expect(len(c.heapsPerBuilds)).To(Equal(totalBuilds))
		Expect(len(c.nameNamespaceToBuildIdMap)).To(Equal(totalBuilds * gameServersPerBuild))

		for i := 0; i < totalBuilds; i++ {
			Expect(len(c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].gameServerNameSet)).To(Equal(gameServersPerBuild))
			Expect(len(*c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].heap)).To(Equal(gameServersPerBuild))
			for j := 0; j < gameServersPerBuild; j++ {
				Expect((*c.heapsPerBuilds[fmt.Sprintf("build-%d", i)].heap)[j].Name).To(Equal(fmt.Sprintf("gs-%d-%d", i, j)))
			}
		}

		for i := 0; i < totalBuilds; i++ {
			for j := 0; j < gameServersPerBuild; j++ {
				gs := c.PopFromCache(fmt.Sprintf("build-%d", i))
				Expect(gs.Name).To(Equal(fmt.Sprintf("gs-%d-%d", i, j)))
				Expect(gs.Namespace).To(Equal(fmt.Sprintf("ns-%d", i)))
				_, exists := c.nameNamespaceToBuildIdMap[fmt.Sprintf("gs-%d-%d", i, j)+fmt.Sprintf("ns-%d", i)]
				Expect(exists).To(BeFalse())
			}
			_, exists := c.heapsPerBuilds[fmt.Sprintf("build-%d", i)]
			Expect(exists).To(BeFalse())
		}
	})
	It("should work with deleting game server", func() {
		c := GameServerHeapForBuild{
			heap: &GameServerHeap{},
		}
		gsArray := generateGameServersForTest()
		for _, gs := range gsArray {
			c.heap.PushToHeap(gs)
		}

		gs := c.heap.PopFromHeap()
		Expect(gs.NodeAge).To(Equal(1))
		Expect(gs.Name).To(Equal("gs-1"))

		deleteGameServerWithName("gs-2", c.heap)
		deleteGameServerWithName("gs-2", c.heap)
		deleteGameServerWithName("gs-3", c.heap)
		deleteGameServerWithName("gs-3", c.heap)

		gs = c.heap.PopFromHeap()
		Expect(gs.NodeAge).To(Equal(4))
		Expect(gs.Name).To(Equal("gs-4"))

		gs = c.heap.PopFromHeap()
		Expect(gs.NodeAge).To(Equal(5))
		Expect(gs.Name).To(Equal("gs-5"))

		deleteGameServerWithName("gs-5", c.heap)

		gs = c.heap.PopFromHeap()
		Expect(gs.NodeAge).To(Equal(6))
		Expect(gs.Name).To(Equal("gs-6"))
	})
	It("should work with multiple simultaneous requests", func() {
		const testBuildID = "test-build-id"
		c := NewGameServersCache()
		wg := &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				gs := createGameServerForCacheTest(fmt.Sprintf("gs-%d", i), "ns", testBuildID, i)
				c.PushToCache(gs)
			}(i)
		}
		wg.Wait()
		Expect(len(*c.heapsPerBuilds[testBuildID].heap)).To(Equal(100))

		wg = &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_ = c.PopFromCache(testBuildID)
			}(i)
		}
		wg.Wait()
		_, exists := c.heapsPerBuilds[testBuildID]
		Expect(exists).To(Equal(false))
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

func deleteGameServerWithName(name string, h *GameServerHeap) {
	for i, gs := range *h {
		if gs.Name == name {
			heap.Remove(h, i)
			return
		}
	}
}

func generateGameServersForTest() []*GameServerForHeap {
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
