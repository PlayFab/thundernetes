package http

import (
	"container/heap"
	"fmt"
	"sync"
)

type GameServersCache struct {
	mutex                     *sync.RWMutex
	cache                     map[string]*GameServersPerBuildCache
	nameNamespaceBuildToIdMap map[string]string
}

func NewGameServersCache() *GameServersCache {
	return &GameServersCache{
		mutex:                     &sync.RWMutex{},
		cache:                     make(map[string]*GameServersPerBuildCache),
		nameNamespaceBuildToIdMap: make(map[string]string),
	}
}

func (gsc *GameServersCache) PushToCache(gs *GameServerForHeap) {
	gsc.mutex.RLock()
	_, exists := gsc.cache[gs.BuildID]
	gsc.mutex.RUnlock()

	if !exists {
		gsc.mutex.Lock()
		gsc.cache[gs.BuildID] = NewGameServersPerBuildCache()
		gsc.mutex.Unlock()
	}

	gsc.mutex.Lock()
	gsc.nameNamespaceBuildToIdMap[getNamespacedName(gs.Namespace, gs.Name)] = gs.BuildID
	gsc.cache[gs.BuildID].PushToCache(gs)
	gsc.mutex.Unlock()
}

func (gsc *GameServersCache) PopFromCache(buildID string) *GameServerForHeap {
	gsc.mutex.Lock()
	defer gsc.mutex.Unlock()
	if _, exists := gsc.cache[buildID]; !exists {
		return nil
	}
	return gsc.cache[buildID].PopFromCache()
}

func (gsc *GameServersCache) RemoveFromCache(namespace, name string) {
	gsc.mutex.Lock()
	defer gsc.mutex.Unlock()
	buildID := gsc.nameNamespaceBuildToIdMap[getNamespacedName(namespace, name)]
	if _, exists := gsc.cache[buildID]; !exists {
		return
	}
	gsc.cache[buildID].RemoveFromCache(namespace, name)
}

type GameServersPerBuildCache struct {
	mutex               *sync.RWMutex
	heap                *GameServerHeap
	gameServerNameCache map[string]interface{}
}

func NewGameServersPerBuildCache() *GameServersPerBuildCache {
	return &GameServersPerBuildCache{
		mutex:               &sync.RWMutex{},
		heap:                &GameServerHeap{},
		gameServerNameCache: make(map[string]interface{}),
	}
}

func (gsbc *GameServersPerBuildCache) PushToCache(gs *GameServerForHeap) {
	gsbc.mutex.RLock()
	_, exists := gsbc.gameServerNameCache[gs.Name]
	gsbc.mutex.RUnlock()
	if !exists {
		gsbc.mutex.Lock()
		gsbc.gameServerNameCache[gs.Name] = struct{}{}
		gsbc.heap.PushToHeap(gs)
		fmt.Printf("Pushed to heap %s\n", gs.Name)
		gsbc.mutex.Unlock()
	}
}

func (gsbc *GameServersPerBuildCache) PopFromCache() *GameServerForHeap {
	gsbc.mutex.Lock()
	defer gsbc.mutex.Unlock()
	if len(*gsbc.heap) == 0 {
		return nil
	}
	return heap.Pop(gsbc.heap).(*GameServerForHeap)
}

func (gsbc *GameServersPerBuildCache) RemoveFromCache(namespace, name string) {
	gsbc.mutex.RLock()
	_, exists := gsbc.gameServerNameCache[name]
	gsbc.mutex.RUnlock()
	if !exists {
		return
	}
	gsbc.mutex.Lock()
	defer gsbc.mutex.Unlock()
	for i, gs2 := range *gsbc.heap {
		if name == gs2.Name && namespace == gs2.Namespace {
			fmt.Printf("Removed from heap %s\n", gs2.Name)
			heap.Remove(gsbc.heap, i)
			delete(gsbc.gameServerNameCache, name)
			return
		}
	}
}

type GameServerForHeap struct {
	Name            string
	Namespace       string
	BuildID         string
	NodeAge         int
	ResourceVersion string
}
type GameServerHeap []*GameServerForHeap

func (h GameServerHeap) Len() int {
	return len(h)
}

func (h GameServerHeap) Less(i, j int) bool {
	return h[i].NodeAge < h[j].NodeAge
}

func (h GameServerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *GameServerHeap) Push(x interface{}) {
	*h = append(*h, x.(*GameServerForHeap))
}

func (h *GameServerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *GameServerHeap) PushToHeap(gs *GameServerForHeap) {
	heap.Push(h, gs)
}

func (h *GameServerHeap) PopFromHeap() *GameServerForHeap {
	return heap.Pop(h).(*GameServerForHeap)
}

func getNamespacedName(namespace, name string) string {
	return namespace + "/" + name
}
