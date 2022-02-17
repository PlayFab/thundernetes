/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"sort"
	"sync"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// We have observed cases in which we'll create more than one GameServer for a GameServerBuild
// This is because on the first reconcile we'll see that we have 0 GameServers and we'll create some
// On the subsequent reconcile, the cache might have not been updated yet, so we'll still see 0 GameServers (or less than asked) and create more,
// so eventually we'll end up with more GameServers than requested
// The code will handle and delete the extra GameServers eventually, but it's a waste of resources unfortunately.
// The solution is to create a synchonized map (since it will be accessed by multiple reconciliations - one for each GameServerBuild)
// In this map, the key is GameServerBuild.Name whereas the value is map[string]interface{} and contains the GameServer.Name for all the GameServers under creation
// We use map[string]interface{} instead a []string to facilitate constant time lookups for GameServer names.
// On every reconcile loop, we check if all the GameServers for this GameServerBuild are present in cache)
// If they are, we remove the GameServerBuild entry from the gameServersUnderCreation map
// If at least one of them is not in the cache, this means that the cache has not been fully updated yet
// so we will exit the current reconcile loop, cache will be updated in a subsequent loop
var gameServersUnderCreation = sync.Map{}

// Similar logic to gameServersUnderCreation, but this time for deletion of game servers
// On every reconcile loop, we check if all the GameServers under deletion for this GameServerBuild have been removed from cache
// If even one of them exists in cache, we exit the reconcile loop
// In a subsequent loop, cache will be updated
var gameServersUnderDeletion = sync.Map{}

// a map to hold the number of crashes per Build
// concurrent since the reconcile loop can be called multiple times for different GameServerBuilds
var crashesPerBuild = sync.Map{}

const (
	maxNumberOfGameServersToAdd    = 20
	maxNumberOfGameServersToDelete = 20
)

// GameServerBuildReconciler reconciles a GameServerBuild object
type GameServerBuildReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	PortRegistry *PortRegistry
	Recorder     record.EventRecorder
}

//+kubebuilder:rbac:groups=mps.playfab.com,resources=gameserverbuilds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mps.playfab.com,resources=gameserverbuilds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mps.playfab.com,resources=gameserverbuilds/finalizers,verbs=update
//+kubebuilder:rbac:groups=mps.playfab.com,resources=gameservers,verbs=get;list;watch
//+kubebuilder:rbac:groups=mps.playfab.com,resources=gameservers/status,verbs=get
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *GameServerBuildReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var gsb mpsv1alpha1.GameServerBuild
	if err := r.Get(ctx, req.NamespacedName, &gsb); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Unable to fetch GameServerBuild - skipping")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch gameServerBuild")
		return ctrl.Result{}, err
	}

	// if GameServerBuild is unhealthy and current crashes equal or more than the crashesToMarkUnhealthy, so do nothing more
	if gsb.Status.Health == mpsv1alpha1.BuildUnhealthy && gsb.Status.CrashesCount >= gsb.Spec.CrashesToMarkUnhealthy {
		log.Info("GameServerBuild is unhealthy, do nothing")
		r.Recorder.Event(&gsb, corev1.EventTypeNormal, "Unhealthy Build", "GameServerBuild is unhealthy, do nothing")
		return ctrl.Result{}, nil
	}

	deletionsCompleted, err := r.gameServersUnderDeletionWereDeleted(ctx, &gsb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !deletionsCompleted {
		return ctrl.Result{}, nil
	}

	creationsCompleted, err := r.gameServersUnderCreationWereCreated(ctx, &gsb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !creationsCompleted {
		return ctrl.Result{}, nil
	}

	if !gsb.DeletionTimestamp.IsZero() {
		// GameServerBuild is being deleted so clear its entry from the crashesPerBuild map
		// no-op if the entry is not present
		crashesPerBuild.Delete(getKeyForCrashesPerBuildMap(&gsb))
	}

	// get the gameServers that are owned by this gameServerBuild
	var gameServers mpsv1alpha1.GameServerList
	if err := r.List(ctx, &gameServers, client.InNamespace(req.Namespace), client.MatchingFields{ownerKey: req.Name}); err != nil {
		// there has been an error
		return ctrl.Result{}, err
	}

	// calculate counts by state so we can update .status accordingly
	var activeCount, standingByCount, crashesCount, initializingCount, pendingCount int
	for i := 0; i < len(gameServers.Items); i++ {
		gs := gameServers.Items[i]

		if gs.Status.State == "" {
			pendingCount++
		} else if gs.Status.State == mpsv1alpha1.GameServerStateInitializing {
			initializingCount++
		} else if gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
			standingByCount++
		} else if gs.Status.State == mpsv1alpha1.GameServerStateActive {
			activeCount++
		} else if gs.Status.State == mpsv1alpha1.GameServerStateCrashed {
			crashesCount++
			if err := r.Delete(ctx, &gs); err != nil {
				return ctrl.Result{}, err
			}
			GameServersCrashedCounter.WithLabelValues(gsb.Name).Inc()
			addGameServerToUnderDeletionMap(gsb.Name, gs.Name)
			r.Recorder.Eventf(&gsb, corev1.EventTypeNormal, "Crashed", "GameServer %s crashed", gs.Name)
		} else if gs.Status.State == mpsv1alpha1.GameServerStateGameCompleted {
			if err := r.Delete(ctx, &gs); err != nil {
				return ctrl.Result{}, err
			}
			GameServersSessionEndedCounter.WithLabelValues(gsb.Name).Inc()
			addGameServerToUnderDeletionMap(gsb.Name, gs.Name)
			r.Recorder.Eventf(&gsb, corev1.EventTypeNormal, "Exited", "GameServer %s session completed", gs.Name)
		}
	}

	// we are sorting GameServers from newest to oldest, since newest have more chances of being in an initializing or pending state
	// prioritizing deletion of newest GameServers, if this is needed
	sort.SliceStable(gameServers.Items, func(i, j int) bool {
		return gameServers.Items[i].GetCreationTimestamp().After(gameServers.Items[j].GetCreationTimestamp().Time)
	})

	// user has decreased standingBy numbers
	if pendingCount+initializingCount+standingByCount > gsb.Spec.StandingBy {
		for i := 0; i < pendingCount+initializingCount+standingByCount-gsb.Spec.StandingBy && i < maxNumberOfGameServersToDelete; i++ {
			gs := gameServers.Items[i]
			// we're deleting only initializing/pending/standingBy servers, never touching active
			if gs.Status.State == "" || gs.Status.State == mpsv1alpha1.GameServerStateInitializing || gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
				// we're requesting the GameServer to be deleted to have the same ResourceVersion
				// since it might have been updated (e.g. allocated) and the cache hasn't been updated yet
				if err := r.Delete(ctx, &gs, &client.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						ResourceVersion: &gs.ResourceVersion,
					},
				}); err != nil {
					if apierrors.IsConflict(err) { // this GameServer has been updated, skip it
						continue
					}
					return ctrl.Result{}, err
				}
				GameServersDeletedCounter.WithLabelValues(gsb.Name).Inc()
				addGameServerToUnderDeletionMap(gsb.Name, gs.Name)
				r.Recorder.Eventf(&gsb, corev1.EventTypeNormal, "GameServer deleted", "GameServer %s deleted", gs.Name)
			}
		}
	}

	// we need to check if we are above the max
	// this will happen if the user modifies the spec.Max during the GameServerBuild's lifetime
	if pendingCount+initializingCount+standingByCount+activeCount > gsb.Spec.Max {
		for i := 0; i < pendingCount+initializingCount+standingByCount+activeCount-gsb.Spec.Max && i < maxNumberOfGameServersToDelete; i++ {
			gs := gameServers.Items[i]
			// we're deleting only standingBy or initializing servers
			if gs.Status.State == "" || gs.Status.State == mpsv1alpha1.GameServerStateInitializing || gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
				// we're requesting the GameServer to be deleted to have the same ResourceVersion
				// since it might have been updated (e.g. allocated) and the cache hasn't been updated yet
				if err := r.Delete(ctx, &gs, &client.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						ResourceVersion: &gs.ResourceVersion,
					},
				}); err != nil {
					if apierrors.IsConflict(err) { // this GameServer has been updated, skip it
						continue
					}
					return ctrl.Result{}, err
				}
				GameServersDeletedCounter.WithLabelValues(gsb.Name).Inc()
				addGameServerToUnderDeletionMap(gsb.Name, gs.Name)
			}
		}
	}

	nonActiveGameServersCount := standingByCount + initializingCount + pendingCount
	// we are in need of standingBy servers, so we're creating them here
	// we're also limiting the number of game servers that are created to avoid issues like this https://github.com/kubernetes-sigs/controller-runtime/issues/1782
	for i := 0; i < gsb.Spec.StandingBy-nonActiveGameServersCount &&
		i+nonActiveGameServersCount+activeCount < gsb.Spec.Max &&
		i < maxNumberOfGameServersToAdd; i++ {
		newgs, err := NewGameServerForGameServerBuild(&gsb, r.PortRegistry)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, newgs); err != nil {
			return ctrl.Result{}, err
		}
		addGameServerToUnderCreationMap(gsb.Name, newgs.Name)
		GameServersCreatedCounter.WithLabelValues(gsb.Name).Inc()
		r.Recorder.Eventf(&gsb, corev1.EventTypeNormal, "Creating", "Creating GameServer %s", newgs.Name)
	}

	return r.updateStatus(ctx, &gsb, pendingCount, initializingCount, standingByCount, activeCount, crashesCount)
}

func (r *GameServerBuildReconciler) updateStatus(ctx context.Context, gsb *mpsv1alpha1.GameServerBuild, pendingCount, initializingCount, standingByCount, activeCount, crashesCount int) (ctrl.Result, error) {
	// update GameServerBuild status only if one of the fields has changed
	if gsb.Status.CurrentPending != pendingCount ||
		gsb.Status.CurrentInitializing != initializingCount ||
		gsb.Status.CurrentActive != activeCount ||
		gsb.Status.CurrentStandingBy != standingByCount ||
		crashesCount > 0 {

		patch := client.MergeFrom(gsb.DeepCopy())

		gsb.Status.CurrentPending = pendingCount
		gsb.Status.CurrentInitializing = initializingCount
		gsb.Status.CurrentActive = activeCount
		gsb.Status.CurrentStandingBy = standingByCount

		// try and get existing crashesCount from the map
		// if it doesn't exist, create it with initial value the number of crashes we detected on this reconcile loop
		key := getKeyForCrashesPerBuildMap(gsb)
		val, ok := crashesPerBuild.LoadOrStore(key, crashesCount)
		// if we have existing crashes, get the value
		var existingCrashes int = 0
		if ok {
			existingCrashes = val.(int)
			// and store the new one
			crashesPerBuild.Store(key, crashesCount+existingCrashes)
		}
		// update the crashesCount status with the new value of total crashes
		gsb.Status.CrashesCount = existingCrashes + crashesCount
		gsb.Status.CurrentStandingByReadyDesired = fmt.Sprintf("%d/%d", standingByCount, gsb.Spec.StandingBy)

		if gsb.Status.CrashesCount >= gsb.Spec.CrashesToMarkUnhealthy {
			gsb.Status.Health = mpsv1alpha1.BuildUnhealthy
		} else {
			gsb.Status.Health = mpsv1alpha1.BuildHealthy
		}

		if err := r.Status().Patch(ctx, gsb, patch); err != nil {
			return ctrl.Result{}, err
		}
	}

	CurrentGameServerGauge.WithLabelValues(gsb.Name, PendingServerStatus).Set(float64(pendingCount))
	CurrentGameServerGauge.WithLabelValues(gsb.Name, InitializingServerStatus).Set(float64(initializingCount))
	CurrentGameServerGauge.WithLabelValues(gsb.Name, StandingByServerStatus).Set(float64(standingByCount))
	CurrentGameServerGauge.WithLabelValues(gsb.Name, ActiveServerStatus).Set(float64(activeCount))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GameServerBuildReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &mpsv1alpha1.GameServer{}, ownerKey, func(rawObj client.Object) []string {
		// grab the GameServer object, extract the owner...
		gs := rawObj.(*mpsv1alpha1.GameServer)
		owner := metav1.GetControllerOf(gs)
		if owner == nil {
			return nil
		}
		// ...make sure it's a GameServerBuild...
		if owner.APIVersion != apiGVStr || owner.Kind != "GameServerBuild" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&mpsv1alpha1.GameServerBuild{}).
		Owns(&mpsv1alpha1.GameServer{}).
		Complete(r)
}

// addGameServerToUnderDeletionMap adds the GameServer to the map of GameServers to be deleted for this GameServerBuild
func addGameServerToUnderDeletionMap(gameServerBuildName, gameServerName string) {
	val, _ := gameServersUnderDeletion.LoadOrStore(gameServerBuildName, make(map[string]interface{}))
	v := val.(map[string]interface{})
	v[gameServerName] = struct{}{}
}

// addGameServerToUnderCreationMap adds a GameServer to the map of GameServers that are under creation for this GameServerBuild
func addGameServerToUnderCreationMap(gameServerBuildName, gameServerName string) {
	val, _ := gameServersUnderCreation.LoadOrStore(gameServerBuildName, make(map[string]interface{}))
	v := val.(map[string]interface{})
	v[gameServerName] = struct{}{}
}

// gameServersUnderDeletionWereDeleted is a helper function that checks if all the GameServers in the map have been deleted from cache
// returns true if all the GameServers have been deleted, false otherwise
func (r *GameServerBuildReconciler) gameServersUnderDeletionWereDeleted(ctx context.Context, gsb *mpsv1alpha1.GameServerBuild) (bool, error) {
	// if this gameServerBuild has GameServers under deletion
	if val, exists := gameServersUnderDeletion.Load(gsb.Name); exists {
		gameServersUnderDeletionForBuild := val.(map[string]interface{})
		// check all GameServers under deletion, if they exist in cache
		for k := range gameServersUnderDeletionForBuild {
			var g mpsv1alpha1.GameServer
			if err := r.Get(ctx, types.NamespacedName{Name: k, Namespace: gsb.Namespace}, &g); err != nil {
				// if one does not exist in cache, this means that cache has been updated (with its deletion)
				// so remove it from the map
				if apierrors.IsNotFound(err) {
					delete(gameServersUnderDeletionForBuild, k)
					continue
				}
				return false, err
			}
		}

		// all GameServers under deletion do not exist in cache
		if len(gameServersUnderDeletionForBuild) == 0 {
			// so it's safe to remove the GameServerBuild entry from the map
			gameServersUnderDeletion.Delete(gsb.Name)
			return true, nil
		}
		return false, nil
	}
	return true, nil
}

// gameServersUnderCreationWereCreated checks if all GameServers under creation exist in cache
// returns true if all GameServers under creation exist in cache
// false otherwise
func (r *GameServerBuildReconciler) gameServersUnderCreationWereCreated(ctx context.Context, gsb *mpsv1alpha1.GameServerBuild) (bool, error) {
	// if this GameServerBuild has GameServers under creation
	if val, exists := gameServersUnderCreation.Load(gsb.Name); exists {
		gameServersUnderCreationForBuild := val.(map[string]interface{})
		for k := range gameServersUnderCreationForBuild {
			var g mpsv1alpha1.GameServer
			if err := r.Get(ctx, types.NamespacedName{Name: k, Namespace: gsb.Namespace}, &g); err != nil {
				// this GameServer doesn't exist in cache, so return false
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			// GameServer exists in cache, so remove it from the map
			delete(gameServersUnderCreationForBuild, k)
		}
		// all GameServers under creation do not exist in cache
		if len(gameServersUnderCreationForBuild) == 0 {
			// so it's safe to remove the GameServerBuild entry from the map
			gameServersUnderCreation.Delete(gsb.Name)
			return true, nil
		}
		return false, nil
	}
	return true, nil
}

func getKeyForCrashesPerBuildMap(gsb *mpsv1alpha1.GameServerBuild) string {
	return fmt.Sprintf("%s/%s", gsb.Namespace, gsb.Name)
}
