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
	"math"
	"runtime"
	"sort"
	"sync"
	"time"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// a map to hold the number of crashes per Build
// concurrent since the reconcile loop can be called multiple times for different GameServerBuilds
// key is namespace/name of the GameServerBuild
// value is the number of crashes
var crashesPerBuild = sync.Map{}

const (
	// maximum number of GameServers to create per reconcile loop
	// we have this in place since each create call is synchronous and we want to minimize the time for each reconcile loop
	maxNumberOfGameServersToAdd = 20
	// maximum number of GameServers to delete per reconcile loop
	maxNumberOfGameServersToDelete = 20
)

// Simple async map implementation using a mutex
// used to manage the expected GameServer creations and deletions
type MutexMap struct {
	data map[string]interface{}
	mu   sync.Mutex
}

// GameServerBuildReconciler reconciles a GameServerBuild object
type GameServerBuildReconciler struct {
	client.Client
	Scheme       *k8sruntime.Scheme
	PortRegistry *PortRegistry
	Recorder     record.EventRecorder
	expectations *GameServerExpectations
}

// NewGameServerBuildReconciler returns a pointer to a new GameServerBuildReconciler
func NewGameServerBuildReconciler(mgr manager.Manager, portRegistry *PortRegistry) *GameServerBuildReconciler {
	cl := mgr.GetClient()
	return &GameServerBuildReconciler{
		Client:       cl,
		Scheme:       mgr.GetScheme(),
		PortRegistry: portRegistry,
		Recorder:     mgr.GetEventRecorderFor("GameServerBuild"),
		expectations: NewGameServerExpectations(cl),
	}
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
			log.Info("Unable to fetch GameServerBuild - it is being deleted")
			// GameServerBuild is being deleted so clear its entry from the crashesPerBuild map
			// no-op if the entry is not present
			crashesPerBuild.Delete(getKeyForCrashesPerBuildMap(&gsb))
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch gameServerBuild")
		return ctrl.Result{}, err
	}

	// if GameServerBuild is unhealthy and current crashes equal or more than CrashesToMarkUnhealthy, do nothing more
	if gsb.Status.Health == mpsv1alpha1.BuildUnhealthy &&
		gsb.Spec.CrashesToMarkUnhealthy != nil &&
		gsb.Status.CrashesCount >= *gsb.Spec.CrashesToMarkUnhealthy {
		log.Info("GameServerBuild is Unhealthy, do nothing")
		r.Recorder.Event(&gsb, corev1.EventTypeNormal, "Unhealthy Build", "GameServerBuild is Unhealthy, stopping reconciliation")
		return ctrl.Result{}, nil
	}

	deletionsCompleted, err := r.expectations.gameServersUnderDeletionWereDeleted(ctx, &gsb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !deletionsCompleted {
		return ctrl.Result{}, nil
	}

	creationsCompleted, err := r.expectations.gameServersUnderCreationWereCreated(ctx, &gsb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !creationsCompleted {
		return ctrl.Result{}, nil
	}

	// get the gameServers that are owned by this GameServerBuild
	var gameServers mpsv1alpha1.GameServerList
	if err := r.List(ctx, &gameServers, client.InNamespace(req.Namespace), client.MatchingFields{ownerKey: req.Name}); err != nil {
		// there has been an error
		return ctrl.Result{}, err
	}

	// calculate counts by state so we can update .status accordingly
	var activeCount, standingByCount, crashesCount, initializingCount, pendingCount int
	// Gather sum of time taken to reach standingby phase and server count to produce the recent average gameserver initialization time
	var timeToStandBySum float64
	var recentStandingByCount int
	timeToStandBySum = 0

	// Gather current sum of estimated time taken to clean up crashed or pending deletion gameservers
	var timeToDeleteBySum float64
	var pendingCleanUpCount int
	timeToStandBySum = 0

	for i := 0; i < len(gameServers.Items); i++ {
		gs := gameServers.Items[i]

		if gs.Status.State == "" && gs.Status.Health != mpsv1alpha1.GameServerUnhealthy { // under normal circumstances, Health will also be equal to ""
			pendingCount++
		} else if gs.Status.State == mpsv1alpha1.GameServerStateInitializing && gs.Status.Health == mpsv1alpha1.GameServerHealthy {
			initializingCount++
		} else if gs.Status.State == mpsv1alpha1.GameServerStateStandingBy && gs.Status.Health == mpsv1alpha1.GameServerHealthy {
			standingByCount++
			if gs.Status.State != gs.Status.PrevState {
				timeToStandBySum += float64(gs.Status.ReachedStandingByOn.Sub(gs.CreationTimestamp.Time).Milliseconds())
				recentStandingByCount++
			}
		} else if gs.Status.State == mpsv1alpha1.GameServerStateActive && gs.Status.Health == mpsv1alpha1.GameServerHealthy {
			activeCount++
		} else if gs.Status.State == mpsv1alpha1.GameServerStateGameCompleted && gs.Status.Health == mpsv1alpha1.GameServerHealthy {
			// game server process exited with code 0
			if err := r.Delete(ctx, &gs); err != nil {
				return ctrl.Result{}, err
			}

			GameServersSessionEndedCounter.WithLabelValues(gsb.Name).Inc()
			r.expectations.addGameServerToUnderDeletionMap(gsb.Name, gs.Name)
			r.Recorder.Eventf(&gsb, corev1.EventTypeNormal, "Exited", "GameServer %s session completed", gs.Name)

			pendingCleanUpCount++
			timeToDeleteBySum += math.Abs(float64(time.Until(gs.DeletionTimestamp.Time).Milliseconds()))
		} else if gs.Status.State == mpsv1alpha1.GameServerStateCrashed {
			// game server process exited with code != 0 (crashed)
			crashesCount++
			if err := r.Delete(ctx, &gs); err != nil {
				return ctrl.Result{}, err
			}
			GameServersCrashedCounter.WithLabelValues(gsb.Name).Inc()
			r.expectations.addGameServerToUnderDeletionMap(gsb.Name, gs.Name)
			r.Recorder.Eventf(&gsb, corev1.EventTypeNormal, "Unhealthy", "GameServer %s was deleted because it became unhealthy, state: %s, health: %s", gs.Name, gs.Status.State, gs.Status.Health)

			pendingCleanUpCount++
			timeToDeleteBySum += math.Abs(float64(time.Until(gs.DeletionTimestamp.Time).Milliseconds()))
		} else if gs.Status.Health == mpsv1alpha1.GameServerUnhealthy {
			// all cases where the game server was marked as Unhealthy
			crashesCount++
			if err := r.Delete(ctx, &gs); err != nil {
				return ctrl.Result{}, err
			}
			GameServersUnhealthyCounter.WithLabelValues(gsb.Name).Inc()
			r.expectations.addGameServerToUnderDeletionMap(gsb.Name, gs.Name)
			r.Recorder.Eventf(&gsb, corev1.EventTypeNormal, "Crashed", "GameServer %s was deleted because it crashed, state: %s, health: %s", gs.Name, gs.Status.State, gs.Status.Health)

			pendingCleanUpCount++
			timeToDeleteBySum += math.Abs(float64(time.Until(gs.DeletionTimestamp.Time).Milliseconds()))
		}
		if gs.Status.State != gs.Status.PrevState {
			gs.Status.PrevState = gs.Status.State
		}
	}

	if recentStandingByCount > 0 {
		GameServersCreatedDuration.WithLabelValues(gsb.Name).Set(timeToStandBySum / float64(recentStandingByCount))
	}

	if pendingCleanUpCount > 0 {
		GameServersCleanUpDuration.WithLabelValues(gsb.Name).Set(timeToDeleteBySum / float64(pendingCleanUpCount))
	}

	// calculate the total amount of servers not in the active state
	nonActiveGameServersCount := standingByCount + initializingCount + pendingCount

	// Evaluate desired number of servers against actual
	var totalNumberOfGameServersToDelete int

	// user has decreased standingBy numbers
	if nonActiveGameServersCount > gsb.Spec.StandingBy {
		totalNumberOfGameServersToDelete += int(math.Min(float64(nonActiveGameServersCount-gsb.Spec.StandingBy), maxNumberOfGameServersToDelete))
	}
	// we also need to check if we are above the max
	// this can happen if the user modifies the spec.Max during the GameServerBuild's lifetime
	if nonActiveGameServersCount+activeCount > gsb.Spec.Max {
		totalNumberOfGameServersToDelete = int(math.Min(float64(totalNumberOfGameServersToDelete+(nonActiveGameServersCount+activeCount-gsb.Spec.Max)), maxNumberOfGameServersToDelete))
	}
	if totalNumberOfGameServersToDelete > 0 {
		err := r.deleteNonActiveGameServers(ctx, &gsb, &gameServers, totalNumberOfGameServersToDelete)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// we are in need of standingBy servers, so we're creating them here
	// we're also limiting the number of game servers that are created to avoid issues like this https://github.com/kubernetes-sigs/controller-runtime/issues/1782
	// we attempt to create the missing number of game servers, but we don't want to create more than the max
	// an error channel for the go routines to write errors
	errCh := make(chan error, maxNumberOfGameServersToAdd)

	// Time how long it takes to trigger new standby gameservers
	standByReconcileStartTime := time.Now()
	// a waitgroup for async create calls
	var wg sync.WaitGroup
	for i := 0; i < gsb.Spec.StandingBy-nonActiveGameServersCount &&
		i+nonActiveGameServersCount+activeCount < gsb.Spec.Max &&
		i < maxNumberOfGameServersToAdd; i++ {
		wg.Add(1)
		go func(standByStartTime time.Time) {
			defer wg.Done()
			newgs, err := NewGameServerForGameServerBuild(&gsb, r.PortRegistry)
			if err != nil {
				errCh <- err
				return
			}
			if err := r.Create(ctx, newgs); err != nil {
				errCh <- err
				return
			}
			newgs.Status.PrevState = mpsv1alpha1.GameServerStateInitializing
			r.expectations.addGameServerToUnderCreationMap(gsb.Name, newgs.Name)
			GameServersCreatedCounter.WithLabelValues(gsb.Name).Inc()
			r.Recorder.Eventf(&gsb, corev1.EventTypeNormal, "Creating", "Creating GameServer %s", newgs.Name)
			GameServersStandByReconcileDuration.WithLabelValues(gsb.Name).Set(float64(time.Since(standByStartTime).Milliseconds()))
		}(standByReconcileStartTime)
	}
	wg.Wait()

	if len(errCh) > 0 {
		return ctrl.Result{}, <-errCh
	}

	return r.updateStatus(ctx, &gsb, pendingCount, initializingCount, standingByCount, activeCount, crashesCount)
}

// updateStatus patches the GameServerBuild's status only if the status of at least one of its GameServers has changed
func (r *GameServerBuildReconciler) updateStatus(ctx context.Context, gsb *mpsv1alpha1.GameServerBuild, pendingCount, initializingCount, standingByCount, activeCount, crashesCount int) (ctrl.Result, error) {
	// patch GameServerBuild status only if one of the fields has changed
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

		existingCrashes := r.getExistingCrashes(gsb, crashesCount)

		// update the crashesCount status with the new value of total crashes
		gsb.Status.CrashesCount = existingCrashes + crashesCount
		gsb.Status.CurrentStandingByReadyDesired = fmt.Sprintf("%d/%d", standingByCount, gsb.Spec.StandingBy)

		// GameServerBuild can only be set as Unhealthy if CrashesToMarkUnhealthy has been explicitly been set by the user
		if gsb.Spec.CrashesToMarkUnhealthy != nil && gsb.Status.CrashesCount >= *gsb.Spec.CrashesToMarkUnhealthy {
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
		WithOptions(controller.Options{
			MaxConcurrentReconciles: runtime.NumCPU(),
		}).
		Complete(r)
}

// getKeyForCrashesPerBuildMap returns the key for the map of crashes per build
// key is namespace/name
func getKeyForCrashesPerBuildMap(gsb *mpsv1alpha1.GameServerBuild) string {
	return fmt.Sprintf("%s/%s", gsb.Namespace, gsb.Name)
}

// deleteNonActiveGameServers loops through all the GameServers CRs and deletes non-Active ones
// after it sorts all of them by state
func (r *GameServerBuildReconciler) deleteNonActiveGameServers(ctx context.Context,
	gsb *mpsv1alpha1.GameServerBuild,
	gameServers *mpsv1alpha1.GameServerList,
	totalNumberOfGameServersToDelete int) error {
	// an error channel for the go routines to write errors
	errCh := make(chan error, totalNumberOfGameServersToDelete)
	// a waitgroup for async deletion calls
	var wg sync.WaitGroup
	deletionCalls := 0
	deletionStartTime := time.Now()

	// we sort the GameServers by state so that we can delete the ones that are empty state or Initializing before we delete the StandingBy ones (if needed)
	// this is to make sure we don't fall below the desired number of StandingBy during scaling down
	sort.Sort(ByState(gameServers.Items))
	for i := 0; i < len(gameServers.Items) && deletionCalls < totalNumberOfGameServersToDelete; i++ {
		gs := gameServers.Items[i]
		// we're deleting only initializing/pending/standingBy servers, never touching active
		if gs.Status.State == "" || gs.Status.State == mpsv1alpha1.GameServerStateInitializing || gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
			deletionCalls++
			wg.Add(1)
			go func(deletionStartTime time.Time) {
				defer wg.Done()
				if err := r.deleteGameServer(ctx, &gs); err != nil {
					if apierrors.IsConflict(err) { // this GameServer has been updated, skip it
						return
					}
					errCh <- err
					return
				}
				GameServersDeletedCounter.WithLabelValues(gsb.Name).Inc()
				r.expectations.addGameServerToUnderDeletionMap(gsb.Name, gs.Name)
				r.Recorder.Eventf(gsb, corev1.EventTypeNormal, "GameServer deleted", "GameServer %s deleted", gs.Name)
				duration := time.Since(deletionStartTime).Milliseconds()
				GameServersEndedDuration.WithLabelValues(gsb.Name).Set(float64(duration))
			}(deletionStartTime)
		}
	}
	wg.Wait()
	if len(errCh) > 0 {
		return <-errCh
	}
	return nil
}

// deleteGameServer deletes the provided GameServer
func (r *GameServerBuildReconciler) deleteGameServer(ctx context.Context, gs *mpsv1alpha1.GameServer) error {
	// we're requesting the GameServer to be deleted to have the same ResourceVersion
	// since it might have been updated (e.g. allocated) and the cache hasn't been updated yet
	return r.Client.Delete(ctx, gs, &client.DeleteOptions{
		Preconditions: &metav1.Preconditions{
			ResourceVersion: &gs.ResourceVersion,
		}})
}

// getTotalCrashes returns the total number of crashes for this GameServerBuild
func (r *GameServerBuildReconciler) getExistingCrashes(gsb *mpsv1alpha1.GameServerBuild, newCrashesCount int) int {
	// try and get existing crashesCount from the map
	// if it doesn't exist, create it with initial value the number of crashes we detected on this reconcile loop
	key := getKeyForCrashesPerBuildMap(gsb)
	val, ok := crashesPerBuild.LoadOrStore(key, newCrashesCount)
	// if we have existing crashes, get the value
	var existingCrashes int = 0
	if ok {
		existingCrashes = val.(int)
		// and store the new one
		crashesPerBuild.Store(key, newCrashesCount+existingCrashes)
	}
	return existingCrashes
}
