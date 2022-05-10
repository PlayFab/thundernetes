package controllers

import (
	"container/heap"
	"sync"
)

// GameServersQueue encapsulates a map of GameServersPerBuildHeaps
// essentially a set of PriorityQueues, one for each GameServerBuild
type GameServersQueue struct {
	mutex *sync.RWMutex
	// queuesPerBuilds is a map of all priority queues, one for each GameServerBuild
	// key to the map is the BuildID
	queuesPerBuilds map[string]*GameServerQueueForBuild
	// namespacedNameToBuildId is a map of the namespaced name of a GameServer to the buildID
	// this is used when we are deleting a GameServer from the queue
	// since we need to know in which GameServerBuild it belongs to
	namespacedNameToBuildId map[string]string
}

// NewGameServersQueue returns a new GameServersQueue
func NewGameServersQueue() *GameServersQueue {
	return &GameServersQueue{
		mutex:                   &sync.RWMutex{},
		queuesPerBuilds:         make(map[string]*GameServerQueueForBuild),
		namespacedNameToBuildId: make(map[string]string),
	}
}

// PushToQueue pushes a GameServerForQueue onto the queue
func (gsc *GameServersQueue) PushToQueue(gs *GameServerForQueue) {
	gsc.mutex.RLock()
	_, exists := gsc.queuesPerBuilds[gs.BuildID]
	gsc.mutex.RUnlock()

	// check if we have created a queue for this GameServerBuild
	if !exists {
		gsc.mutex.Lock()
		gsc.queuesPerBuilds[gs.BuildID] = NewGameServersPerBuildQueue()
		gsc.mutex.Unlock()
	}

	gsc.mutex.Lock()
	defer gsc.mutex.Unlock()
	// store the BuildID for this GameServer
	gsc.namespacedNameToBuildId[getNamespacedName(gs.Namespace, gs.Name)] = gs.BuildID
	gsc.queuesPerBuilds[gs.BuildID].PushToQueue(gs)
}

// PopFromQueue pops the top GameServerForHeap off the queue
func (gsc *GameServersQueue) PopFromQueue(buildID string) *GameServerForQueue {
	gsc.mutex.Lock()
	defer gsc.mutex.Unlock()
	if _, exists := gsc.queuesPerBuilds[buildID]; !exists {
		return nil
	}
	gsfh := gsc.queuesPerBuilds[buildID].PopFromQueue()
	// we ran out of GameServers for this GameServerBuild
	if len(gsc.queuesPerBuilds[buildID].gameServerNameSet) == 0 {
		delete(gsc.queuesPerBuilds, buildID)
	}
	return gsfh
}

// RemoveFromQueue removes a GameServer from the queue based on the provided namespace/name tuple
func (gsc *GameServersQueue) RemoveFromQueue(namespace, name string) {
	gsc.mutex.Lock()
	defer gsc.mutex.Unlock()
	// get the buildID for this GameServer
	buildID := gsc.namespacedNameToBuildId[getNamespacedName(namespace, name)]
	if _, exists := gsc.queuesPerBuilds[buildID]; !exists {
		return
	}
	gsc.queuesPerBuilds[buildID].RemoveFromQueue(namespace, name)
	// remove the GameServer from the nameNamespaceToBuildIdMap
	delete(gsc.namespacedNameToBuildId, getNamespacedName(namespace, name))
	// we ran out of GameServers for this GameServerBuild
	if len(gsc.queuesPerBuilds[buildID].gameServerNameSet) == 0 {
		delete(gsc.queuesPerBuilds, buildID)
	}
}

// GameServerQueueForBuild encapsulates a queue of GameServerForQueue for a specific GameServerBuild
// also contains a map of all the GameServers for that GameServerBuild
type GameServerQueueForBuild struct {
	mutex *sync.RWMutex
	// queue is the actual priority queue that stores the GameServers
	queue *GameServerQueue
	// gameServerNameSet is a map of all the GameServers for that GameServerBuild
	// this is used to facilitate O(1) lookup of a GameServer
	gameServerNameSet map[string]interface{}
}

// NewGameServersPerBuildQueue returns a new priority queue for a single GameServerBuild
func NewGameServersPerBuildQueue() *GameServerQueueForBuild {
	return &GameServerQueueForBuild{
		mutex:             &sync.RWMutex{},
		queue:             &GameServerQueue{},
		gameServerNameSet: make(map[string]interface{}),
	}
}

// PushToQueue pushes a GameServerForQueue onto the queue
func (gsbc *GameServerQueueForBuild) PushToQueue(gs *GameServerForQueue) {
	gsbc.mutex.RLock()
	_, exists := gsbc.gameServerNameSet[gs.Name]
	gsbc.mutex.RUnlock()
	if !exists {
		gsbc.mutex.Lock()
		defer gsbc.mutex.Unlock()
		gsbc.gameServerNameSet[gs.Name] = struct{}{}
		gsbc.queue.PushToQueue(gs)
	}
}

// PopFromQueue pops the top GameServerForQueue off the queue
func (gsbc *GameServerQueueForBuild) PopFromQueue() *GameServerForQueue {
	gsbc.mutex.Lock()
	defer gsbc.mutex.Unlock()
	if len(*gsbc.queue) == 0 {
		return nil
	}
	gsfh := heap.Pop(gsbc.queue).(*GameServerForQueue)
	delete(gsbc.gameServerNameSet, gsfh.Name)
	return gsfh
}

// RemoveFromQueue removes a GameServer from the queue based on the provided namespace/name tuple
func (gsbc *GameServerQueueForBuild) RemoveFromQueue(namespace, name string) {
	gsbc.mutex.RLock()
	_, exists := gsbc.gameServerNameSet[name]
	gsbc.mutex.RUnlock()
	if !exists {
		return
	}
	gsbc.mutex.Lock()
	defer gsbc.mutex.Unlock()
	for i, gs2 := range *gsbc.queue {
		if name == gs2.Name && namespace == gs2.Namespace {
			heap.Remove(gsbc.queue, i)
			delete(gsbc.gameServerNameSet, name)
			return
		}
	}
}

// GameServerForQueue is a helper struct that encapsulates all the details we need from a GameServer object
// in order to store it on the queue
type GameServerForQueue struct {
	Name            string
	Namespace       string
	BuildID         string
	NodeAge         int
	ResourceVersion string
}

// GameServerQueue implements a PriorityQueue for GameServer objects
// GameServers are sorted in the queue in ascending order based on the NodeAge field
// this queue is used by the allocation algorithm, to prioritize allocations on the Nodes that are newer
// based on https://pkg.go.dev/container/heap
type GameServerQueue []*GameServerForQueue

// Len returns the number of elements in the heap
func (h GameServerQueue) Len() int {
	return len(h)
}

// Less returns true if the GameServerForHeap with index i is in a newer Node (smaller NodeAge) compared to the GameServerForHeap with index j
func (h GameServerQueue) Less(i, j int) bool {
	return h[i].NodeAge < h[j].NodeAge
}

// Swap swaps the GameServerForHeap with index i and the GameServerForHeap with index j
func (h GameServerQueue) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push pushes a interface{} element onto the heap
// this is written just to help implement the heap interface
// PopFromQueue should be used instead
func (h *GameServerQueue) Push(x interface{}) {
	*h = append(*h, x.(*GameServerForQueue))
}

// Pop pops the top interface{} element off the heap
// this is written just to help implement the heap interface
// PopFromQueue should be used instead
func (h *GameServerQueue) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// PushToQueue pushes a GameServerForHeap onto the heap
// It should be used instead of heap.Push
func (h *GameServerQueue) PushToQueue(gs *GameServerForQueue) {
	heap.Push(h, gs)
}

// PopFromQueue pops the top GameServerForHeap off the heap
// It should be used instead of heap.Pop
func (h *GameServerQueue) PopFromQueue() *GameServerForQueue {
	return heap.Pop(h).(*GameServerForQueue)
}

// getNamespacedName returns a namespaced name for a GameServer
func getNamespacedName(namespace, name string) string {
	return namespace + "/" + name
}
