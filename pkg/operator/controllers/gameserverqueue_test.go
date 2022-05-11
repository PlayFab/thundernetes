package controllers

import (
	"container/heap"
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("gameserverqueue tests", func() {
	It("should add a game server per build", func() {
		c := NewGameServersQueue()
		for i := 0; i < 4; i++ {
			gs := testCreateGameServerForQueue(fmt.Sprintf("gs-%d", i), fmt.Sprintf("ns-%d", i), fmt.Sprintf("build-%d", i), 0)
			c.PushToQueue(gs)
		}
		Expect(len(c.queuesPerBuilds)).To(Equal(4))
		Expect(len(c.namespacedNameToBuildId)).To(Equal(4))

		for i := 0; i < 4; i++ {
			Expect(len(c.queuesPerBuilds[fmt.Sprintf("build-%d", i)].gameServerNameSet)).To(Equal(1))
			Expect(len(*c.queuesPerBuilds[fmt.Sprintf("build-%d", i)].queue)).To(Equal(1))
			Expect((*c.queuesPerBuilds[fmt.Sprintf("build-%d", i)].queue)[0].Name).To(Equal(fmt.Sprintf("gs-%d", i)))
		}

		for i := 0; i < 4; i++ {
			gs := c.PopFromQueue(fmt.Sprintf("build-%d", i))
			Expect(gs.Name).To(Equal(fmt.Sprintf("gs-%d", i)))
			Expect(gs.Namespace).To(Equal(fmt.Sprintf("ns-%d", i)))

			_, exists := c.queuesPerBuilds[fmt.Sprintf("build-%d", i)]
			Expect(exists).To(BeFalse())
			_, exists = c.namespacedNameToBuildId[fmt.Sprintf("gs-%d", i)+fmt.Sprintf("ns-%d", i)]
			Expect(exists).To(BeFalse())
		}

		// next pops should be nil
		for i := 0; i < 4; i++ {
			gs := c.PopFromQueue(fmt.Sprintf("build-%d", i))
			Expect(gs).To(BeNil())
		}
	})
	It("should add multiple game servers per Build", func() {
		c := NewGameServersQueue()
		const totalBuilds = 4
		const gameServersPerBuild = 5
		for i := 0; i < totalBuilds; i++ {
			for j := 0; j < gameServersPerBuild; j++ {
				gs := testCreateGameServerForQueue(fmt.Sprintf("gs-%d-%d", i, j), fmt.Sprintf("ns-%d", i), fmt.Sprintf("build-%d", i), j)
				c.PushToQueue(gs)
			}
		}
		Expect(len(c.queuesPerBuilds)).To(Equal(totalBuilds))
		Expect(len(c.namespacedNameToBuildId)).To(Equal(totalBuilds * gameServersPerBuild))

		for i := 0; i < totalBuilds; i++ {
			Expect(len(c.queuesPerBuilds[fmt.Sprintf("build-%d", i)].gameServerNameSet)).To(Equal(gameServersPerBuild))
			Expect(len(*c.queuesPerBuilds[fmt.Sprintf("build-%d", i)].queue)).To(Equal(gameServersPerBuild))
			for j := 0; j < gameServersPerBuild; j++ {
				Expect((*c.queuesPerBuilds[fmt.Sprintf("build-%d", i)].queue)[j].Name).To(Equal(fmt.Sprintf("gs-%d-%d", i, j)))
			}
		}

		for i := 0; i < totalBuilds; i++ {
			for j := 0; j < gameServersPerBuild; j++ {
				gs := c.PopFromQueue(fmt.Sprintf("build-%d", i))
				Expect(gs.Name).To(Equal(fmt.Sprintf("gs-%d-%d", i, j)))
				Expect(gs.Namespace).To(Equal(fmt.Sprintf("ns-%d", i)))
				_, exists := c.namespacedNameToBuildId[fmt.Sprintf("gs-%d-%d", i, j)+fmt.Sprintf("ns-%d", i)]
				Expect(exists).To(BeFalse())
			}
			_, exists := c.queuesPerBuilds[fmt.Sprintf("build-%d", i)]
			Expect(exists).To(BeFalse())
		}
	})
	It("should work with deleting game server", func() {
		c := GameServerQueueForBuild{
			queue: &GameServerQueue{},
		}
		gsArray := testFenerateGameServers()
		for _, gs := range gsArray {
			c.queue.PushToQueue(gs)
		}

		gs := c.queue.PopFromQueue()
		Expect(gs.NodeAge).To(Equal(1))
		Expect(gs.Name).To(Equal("gs-1"))

		testDeleteGameServerWithName("gs-2", c.queue)
		testDeleteGameServerWithName("gs-2", c.queue)
		testDeleteGameServerWithName("gs-3", c.queue)
		testDeleteGameServerWithName("gs-3", c.queue)

		gs = c.queue.PopFromQueue()
		Expect(gs.NodeAge).To(Equal(4))
		Expect(gs.Name).To(Equal("gs-4"))

		gs = c.queue.PopFromQueue()
		Expect(gs.NodeAge).To(Equal(5))
		Expect(gs.Name).To(Equal("gs-5"))

		testDeleteGameServerWithName("gs-5", c.queue)

		gs = c.queue.PopFromQueue()
		Expect(gs.NodeAge).To(Equal(6))
		Expect(gs.Name).To(Equal("gs-6"))
	})
	It("should work with multiple simultaneous requests", func() {
		const testBuildID = "test-build-id"
		c := NewGameServersQueue()
		wg := &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				gs := testCreateGameServerForQueue(fmt.Sprintf("gs-%d", i), "ns", testBuildID, i)
				c.PushToQueue(gs)
			}(i)
		}
		wg.Wait()
		Expect(len(*c.queuesPerBuilds[testBuildID].queue)).To(Equal(100))

		wg = &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_ = c.PopFromQueue(testBuildID)
			}(i)
		}
		wg.Wait()
		_, exists := c.queuesPerBuilds[testBuildID]
		Expect(exists).To(Equal(false))
	})
})

func testCreateGameServerForQueue(name, namespace, buildID string, nodeAge int) *GameServerForQueue {
	return &GameServerForQueue{
		Name:      name,
		Namespace: namespace,
		BuildID:   buildID,
		NodeAge:   nodeAge,
	}
}

func testDeleteGameServerWithName(name string, h *GameServerQueue) {
	for i, gs := range *h {
		if gs.Name == name {
			heap.Remove(h, i)
			return
		}
	}
}

func testFenerateGameServers() []*GameServerForQueue {
	nums := []int{3, 2, 20, 5, 3, 1, 2, 5, 6, 9, 10, 4}
	var gameServers []*GameServerForQueue
	for _, i := range nums {
		gs := GameServerForQueue{
			Name:    fmt.Sprintf("gs-%d", i),
			NodeAge: i,
		}
		gameServers = append(gameServers, &gs)
	}
	return gameServers
}
