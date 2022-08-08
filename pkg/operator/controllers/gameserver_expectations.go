package controllers

import (
	"context"
	"sync"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GameServerExpectations struct {
	// gameServersUnderCreation is a map of GameServerBuilds to a map of GameServers that are under creation
	// We have observed cases in which the controller creates more GameServers than necessary for a GameServerBuild
	// This is because e.g. on the first reconcile call, the controller sees that we have 0 GameServers and it starts to create some
	// On the subsequent reconcile, the cache might have not been updated yet, so we'll still see 0 GameServers (or less than asked) and create more,
	// so eventually we'll end up with more GameServers than requested
	// The controller code will eventually delete the extra GameServers, but we can improve this process.
	// The solution is to create a synchronized map to track which objects were just created
	// (synchronized since it might be accessed by multiple reconciliation goroutines - one for each GameServerBuild)
	// In this map, the key is the name of the GameServerBuild
	// The value is a struct with map[string]interface{} and a mutex
	// The map acts like a set which contains the name of the GameServer for all the GameServers under creation
	// (We use map[string]interface{} instead of a []string to facilitate constant time lookups for GameServer names)
	// On every reconcile loop, we check if all the GameServers for this GameServerBuild are present in cache
	// If they are, we remove the GameServerBuild entry from the gameServersUnderCreation map
	// If at least one of them is not in the cache, this means that the cache has not been fully updated yet
	// so we will exit the current reconcile loop, as GameServers are created the controller will reconcile again
	gameServersUnderCreation sync.Map

	// gameServersUnderDeletion is a map of GameServerBuilds to a map of GameServers that are under deletion
	// Similar logic to gameServersUnderCreation, but this time regarding deletion of game servers
	// On every reconcile loop, we check if all the GameServers under deletion for this GameServerBuild have been removed from cache
	// If even one of them exists in cache, we exit the reconcile loop
	// In a subsequent loop, cache will be updated
	gameServersUnderDeletion sync.Map
	// client is an K8s API reader
	client client.Reader
}

// NewGameServerExpectations returns a pointer to a new GameServerExpectations struct
func NewGameServerExpectations(c client.Reader) *GameServerExpectations {
	return &GameServerExpectations{
		gameServersUnderCreation: sync.Map{},
		gameServersUnderDeletion: sync.Map{},
		client:                   c,
	}
}

// addGameServerToUnderDeletionMap adds the GameServer to the map of GameServers to be deleted for this GameServerBuild
func (g *GameServerExpectations) addGameServerToUnderDeletionMap(gameServerBuildName, gameServerName string) {
	val, _ := g.gameServersUnderDeletion.LoadOrStore(gameServerBuildName, &MutexMap{make(map[string]interface{}), sync.Mutex{}})
	v := val.(*MutexMap)
	v.mu.Lock()
	v.data[gameServerName] = struct{}{}
	v.mu.Unlock()
}

// addGameServerToUnderCreationMap adds a GameServer to the map of GameServers that are under creation for this GameServerBuild
func (g *GameServerExpectations) addGameServerToUnderCreationMap(gameServerBuildName, gameServerName string) {
	val, _ := g.gameServersUnderCreation.LoadOrStore(gameServerBuildName, &MutexMap{make(map[string]interface{}), sync.Mutex{}})
	v := val.(*MutexMap)
	v.mu.Lock()
	v.data[gameServerName] = struct{}{}
	v.mu.Unlock()
}

// gameServersUnderDeletionWereDeleted is a helper function that checks if all the GameServers in the map have been deleted from cache
// returns true if all the GameServers have been deleted, false otherwise
func (g *GameServerExpectations) gameServersUnderDeletionWereDeleted(ctx context.Context, gsb *mpsv1alpha1.GameServerBuild) (bool, error) {
	// if this gameServerBuild has GameServers under deletion
	if val, exists := g.gameServersUnderDeletion.Load(gsb.Name); exists {
		gameServersUnderDeletionForBuild := val.(*MutexMap)
		// check all GameServers under deletion, if they exist in cache
		gameServersUnderDeletionForBuild.mu.Lock()
		defer gameServersUnderDeletionForBuild.mu.Unlock()
		for k := range gameServersUnderDeletionForBuild.data {
			var gs mpsv1alpha1.GameServer
			if err := g.client.Get(ctx, types.NamespacedName{Name: k, Namespace: gsb.Namespace}, &gs); err != nil {
				// if one does not exist in cache, this means that cache has been updated (with its deletion)
				// so remove it from the map
				if apierrors.IsNotFound(err) {
					delete(gameServersUnderDeletionForBuild.data, k)
					continue
				}
				return false, err
			}
		}

		// all GameServers under deletion do not exist in cache
		if len(gameServersUnderDeletionForBuild.data) == 0 {
			// so it's safe to remove the GameServerBuild entry from the map
			g.gameServersUnderDeletion.Delete(gsb.Name)
			return true, nil
		}
		return false, nil
	}
	return true, nil
}

// gameServersUnderCreationWereCreated checks if all GameServers under creation exist in cache
// returns true if all GameServers under creation exist in cache
// false otherwise
func (g *GameServerExpectations) gameServersUnderCreationWereCreated(ctx context.Context, gsb *mpsv1alpha1.GameServerBuild) (bool, error) {
	// if this GameServerBuild has GameServers under creation
	if val, exists := g.gameServersUnderCreation.Load(gsb.Name); exists {
		gameServersUnderCreationForBuild := val.(*MutexMap)
		gameServersUnderCreationForBuild.mu.Lock()
		defer gameServersUnderCreationForBuild.mu.Unlock()
		for k := range gameServersUnderCreationForBuild.data {
			var gs mpsv1alpha1.GameServer
			if err := g.client.Get(ctx, types.NamespacedName{Name: k, Namespace: gsb.Namespace}, &gs); err != nil {
				// this GameServer doesn't exist in cache, so return false
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			// GameServer exists in cache, so remove it from the map
			delete(gameServersUnderCreationForBuild.data, k)
		}
		// all GameServers under creation do not exist in cache
		if len(gameServersUnderCreationForBuild.data) == 0 {
			// so it's safe to remove the GameServerBuild entry from the map
			g.gameServersUnderCreation.Delete(gsb.Name)
			return true, nil
		}
		return false, nil
	}
	return true, nil
}
