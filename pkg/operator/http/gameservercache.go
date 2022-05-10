package http

import (
	"container/heap"
	"sync"
)

// GameServersCache encapsulates a map of GameServersPerBuildHeaps
// essentially a set of PriorityQueues, one for each GameServerBuild
type GameServersCache struct {
	mutex                     *sync.RWMutex
	heapsPerBuilds            map[string]*GameServerHeapForBuild
	nameNamespaceToBuildIdMap map[string]string
}

// NewGameServersCache returns a new GameServersCache
func NewGameServersCache() *GameServersCache {
	return &GameServersCache{
		mutex:                     &sync.RWMutex{},
		heapsPerBuilds:            make(map[string]*GameServerHeapForBuild),
		nameNamespaceToBuildIdMap: make(map[string]string),
	}
}

// PushToCache pushes a GameServerForHeap onto the cache
func (gsc *GameServersCache) PushToCache(gs *GameServerForHeap) {
	gsc.mutex.RLock()
	_, exists := gsc.heapsPerBuilds[gs.BuildID]
	gsc.mutex.RUnlock()

	if !exists {
		gsc.mutex.Lock()
		gsc.heapsPerBuilds[gs.BuildID] = NewGameServersPerBuildHeap()
		gsc.mutex.Unlock()
	}

	gsc.mutex.Lock()
	gsc.nameNamespaceToBuildIdMap[getNamespacedName(gs.Namespace, gs.Name)] = gs.BuildID
	gsc.heapsPerBuilds[gs.BuildID].PushToCache(gs)
	gsc.mutex.Unlock()
}

// PopFromCache pops the top GameServerForHeap off the heap
func (gsc *GameServersCache) PopFromCache(buildID string) *GameServerForHeap {
	gsc.mutex.Lock()
	defer gsc.mutex.Unlock()
	if _, exists := gsc.heapsPerBuilds[buildID]; !exists {
		return nil
	}
	gsfh := gsc.heapsPerBuilds[buildID].PopFromHeap()
	// if we ran out of GameServers for this GameServerBuild
	if len(gsc.heapsPerBuilds[buildID].gameServerNameSet) == 0 {
		delete(gsc.heapsPerBuilds, buildID)
	}
	return gsfh
}

// RemoveFromCache removes a GameServer from the heap based on the provided namespace/name tuple
func (gsc *GameServersCache) RemoveFromCache(namespace, name string) {
	gsc.mutex.Lock()
	defer gsc.mutex.Unlock()
	// get the buildID for this GameServer
	buildID := gsc.nameNamespaceToBuildIdMap[getNamespacedName(namespace, name)]
	if _, exists := gsc.heapsPerBuilds[buildID]; !exists {
		return
	}
	gsc.heapsPerBuilds[buildID].RemoveFromCache(namespace, name)
	// remove the GameServer from the nameNamespaceToBuildIdMap
	delete(gsc.nameNamespaceToBuildIdMap, getNamespacedName(namespace, name))
	// if we ran out of GameServers for this GameServerBuild
	if len(gsc.heapsPerBuilds[buildID].gameServerNameSet) == 0 {
		delete(gsc.heapsPerBuilds, buildID)
	}
}

// GameServerHeapForBuild encapsulates a heap of GameServerForHeap for a specific GameServerBuild
// also contains a map of all the GameServers for that GameServerBuild, to facilitate O(1) lookups
type GameServerHeapForBuild struct {
	mutex             *sync.RWMutex
	heap              *GameServerHeap
	gameServerNameSet map[string]interface{}
}

// NewGameServersPerBuildHeap returns a new GameServersPerBuildHeap
func NewGameServersPerBuildHeap() *GameServerHeapForBuild {
	return &GameServerHeapForBuild{
		mutex:             &sync.RWMutex{},
		heap:              &GameServerHeap{},
		gameServerNameSet: make(map[string]interface{}),
	}
}

// PushToCache pushes a GameServerForHeap onto the heap
func (gsbc *GameServerHeapForBuild) PushToCache(gs *GameServerForHeap) {
	gsbc.mutex.RLock()
	_, exists := gsbc.gameServerNameSet[gs.Name]
	gsbc.mutex.RUnlock()
	if !exists {
		gsbc.mutex.Lock()
		defer gsbc.mutex.Unlock()
		gsbc.gameServerNameSet[gs.Name] = struct{}{}
		gsbc.heap.PushToHeap(gs)
	}
}

// PopFromHeap pops the top GameServerForHeap off the heap
func (gsbc *GameServerHeapForBuild) PopFromHeap() *GameServerForHeap {
	gsbc.mutex.Lock()
	defer gsbc.mutex.Unlock()
	if len(*gsbc.heap) == 0 {
		return nil
	}
	gsfh := heap.Pop(gsbc.heap).(*GameServerForHeap)
	delete(gsbc.gameServerNameSet, gsfh.Name)
	return gsfh
}

// RemoveFromCache removes a GameServer from the heap based on the provided namespace/name tuple
func (gsbc *GameServerHeapForBuild) RemoveFromCache(namespace, name string) {
	gsbc.mutex.RLock()
	_, exists := gsbc.gameServerNameSet[name]
	gsbc.mutex.RUnlock()
	if !exists {
		return
	}
	gsbc.mutex.Lock()
	defer gsbc.mutex.Unlock()
	for i, gs2 := range *gsbc.heap {
		if name == gs2.Name && namespace == gs2.Namespace {
			heap.Remove(gsbc.heap, i)
			delete(gsbc.gameServerNameSet, name)
			return
		}
	}
}

// GameServerForHeap is a helper struct that encapsulates all the details we need from a GameServer object
// in order to keep it on the heap
type GameServerForHeap struct {
	Name            string
	Namespace       string
	BuildID         string
	NodeAge         int
	ResourceVersion string
}

// GameServerHeap implements a PriorityQueue for GameServer objects
// https://pkg.go.dev/container/heap
type GameServerHeap []*GameServerForHeap

// Len returns the number of elements in the heap
func (h GameServerHeap) Len() int {
	return len(h)
}

// Less returns true if the GameServerForHeap with index i is in a newer Node compared to the GameServerForHeap with index j
func (h GameServerHeap) Less(i, j int) bool {
	return h[i].NodeAge < h[j].NodeAge
}

// Swap swaps the GameServerForHeap with index i and the GameServerForHeap with index j
func (h GameServerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push pushes a interface{} element onto the heap
func (h *GameServerHeap) Push(x interface{}) {
	*h = append(*h, x.(*GameServerForHeap))
}

// Pop pops the top interface{} element off the heap
func (h *GameServerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// PushToHeap pushes a GameServerForHeap onto the heap
// It should be used instead of heap.Push
func (h *GameServerHeap) PushToHeap(gs *GameServerForHeap) {
	heap.Push(h, gs)
}

// PopFromHeap pops the top GameServerForHeap off the heap
// It should be used instead of heap.Pop
func (h *GameServerHeap) PopFromHeap() *GameServerForHeap {
	return heap.Pop(h).(*GameServerForHeap)
}

// getNamespacedName returns a namespaced name for a GameServer
func getNamespacedName(namespace, name string) string {
	return namespace + "/" + name
}
