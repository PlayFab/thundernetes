package controllers

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	assertPollingInterval = 20 * time.Millisecond
	assertTimeout         = 2 * time.Second
)

var _ = Describe("GameServerBuild controller tests", func() {
	Context("testing creating a gameserverbuild creates game servers", func() {
		ctx := context.Background()

		// tests here follow the flow
		// 1. create a gameserverbuild
		// 2. verify that we have the requested count of GameServer CRs
		// 3. since we don't have containers running (so the daemonset can update the GameServer.Status), we manually
		//    set the initializing GameServers' .Status to standingBy
		// 4. verify the number of standingBy and active GameServers
		// It's important to verify that the requested number of GameServers has been created, since if we
		// try to fetch them to update their .Status, we might end up in a state that they have not been created yet on the API server

		// tests the creation of game servers
		It("should create game servers", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			verifyTotalGameServerCount(ctx, buildID, 2)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 2, 0)
		})

		// simple scaling test
		It("should scale game servers", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			verifyTotalGameServerCount(ctx, buildID, 2)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 2, 0)

			updateGameServerBuild(ctx, 4, 4, buildName)
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 4, 0)
		})
		// controller should block creating ">max" GameServers
		It("should not scale game servers beyond max", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			verifyTotalGameServerCount(ctx, buildID, 2)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 2, 0)

			updateGameServerBuild(ctx, 5, 4, buildName)
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 4, 0)
		})
		// however, when we increase the max, controller should create the additional GameServers
		It("should scale game servers when max is increasing", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			verifyTotalGameServerCount(ctx, buildID, 2)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 2, 0)

			updateGameServerBuild(ctx, 5, 4, buildName)
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 4, 0)

			updateGameServerBuild(ctx, 5, 5, buildName)
			verifyTotalGameServerCount(ctx, buildID, 5)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 5, 0)
		})

		// manual allocation by manually setting .GameServer.Status.State to "Active"
		It("should increase standingBy when allocating", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			verifyTotalGameServerCount(ctx, buildID, 2)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 2, 0)

			// allocate two GameServers and watch that new StandingBys are being created
			allocateGameServer(ctx, buildID)
			verifyTotalGameServerCount(ctx, buildID, 3)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 2, 1)

			allocateGameServer(ctx, buildID)
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 2, 2)

			// another scaling test, increase the standingBy to 4 and max to 6
			// since we have 2 active, we should have 4 standingBy
			updateGameServerBuild(ctx, 4, 6, buildName)
			verifyTotalGameServerCount(ctx, buildID, 6)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 4, 2)
		})

		It("should create new game servers if game sessions end", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 4, 4, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 4, 0)

			// allocate two game servers, end up with 2 standingBy and 2 active
			allocateGameServer(ctx, buildID)
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 3, 1)

			allocateGameServer(ctx, buildID)
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 2, 2)

			// gracefully end the game session for one game server
			// we wait till we have a new GameServer, since it'll take one reconcile loop for the controller to notice the GameServer has exited
			// once we get that, we verify that the controller has created a new GameServer
			terminateActiveSession(ctx, buildID, true)
			waitTillCountGameServersAreInitializing(ctx, buildID, 1)
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 3, 1)

			// as before, we gracefully terminate the second active GameServer
			// we now should have 4 standingBy
			terminateActiveSession(ctx, buildID, true)
			waitTillCountGameServersAreInitializing(ctx, buildID, 1)
			verifyTotalGameServerCount(ctx, buildID, 4)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 4, 0)
		})

		It("should mark Build as unhealthy when there are too many crashes", func() {
			// create a Build with 6 standingBy
			buildName, buildID := getNewBuildNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 6, 6, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			verifyTotalGameServerCount(ctx, buildID, 6)
			updateInitializingGameServersToStandingBy(ctx, buildID)
			verifyStandingByActiveByCount(ctx, buildID, 6, 0)

			// allocate 5 times
			for i := 0; i < 5; i++ {
				allocateGameServer(ctx, buildID)
			}
			verifyTotalGameServerCount(ctx, buildID, 6)
			verifyStandingByActiveByCount(ctx, buildID, 1, 5)

			// simulate 5 crashes (5 is the default value for GameServerBuild to be marked as Unhealthy)
			for i := 0; i < 5; i++ {
				terminateActiveSession(ctx, buildID, false)
			}
			verifyThatBuildIsUnhealthy(ctx, buildName)
		})

		It("should overwrite containerPort with hostPort value when hostNetwork is required", func() {
			// create a Build with 6 standingBy
			buildName, buildID := getNewBuildNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, true)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			verifyTotalGameServerCount(ctx, buildID, 2)
			verifyContainerPortIsTheSameAsHostPort(ctx, buildName)
		})
	})
})

// getNewBuildNameAndID returns a new build name and ID
func getNewBuildNameAndID() (string, string) {
	buildName := randString(5)
	buildID := string(uuid.NewUUID())
	return buildName, buildID
}

func verifyContainerPortIsTheSameAsHostPort(ctx context.Context, buildID string) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := k8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		for _, gameServer := range gameServers.Items {
			// get the Pod with the same name as this GameServer
			var pod corev1.Pod
			err := k8sClient.Get(ctx, types.NamespacedName{Name: gameServer.Name, Namespace: testnamespace}, &pod)
			Expect(err).ToNot(HaveOccurred())
			// get the container port
			containerPort := pod.Spec.Containers[0].Ports[0].ContainerPort
			// get the host port
			hostPort := pod.Spec.Containers[0].Ports[0].HostPort
			if containerPort != hostPort {
				return false
			}
		}
		return true
	}, timeout, interval).Should(BeTrue())
}

// verifyThatBuildIsUnhealthy verifies that the Build is unhealthy
func verifyThatBuildIsUnhealthy(ctx context.Context, buildName string) {
	Eventually(func() bool {
		var gameServerBuild mpsv1alpha1.GameServerBuild
		err := k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testnamespace}, &gameServerBuild)
		Expect(err).ShouldNot(HaveOccurred())
		return gameServerBuild.Status.Health == mpsv1alpha1.BuildUnhealthy
	}, assertTimeout, assertPollingInterval).Should(BeTrue())
}

func waitTillCountGameServersAreInitializing(ctx context.Context, buildID string, count int) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := k8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		var initializingCount int
		for i := 0; i < len(gameServers.Items); i++ {
			gs := gameServers.Items[i]
			if gs.Status.State == "" || gs.Status.State == mpsv1alpha1.GameServerStateInitializing {
				initializingCount++
			}
		}
		return initializingCount == count
	}, assertTimeout, assertPollingInterval).Should(BeTrue())
}

func terminateActiveSession(ctx context.Context, buildID string, gracefulTermination bool) {
	var gameServers mpsv1alpha1.GameServerList
	err := k8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
	Expect(err).ToNot(HaveOccurred())
	for i := 0; i < len(gameServers.Items); i++ {
		gs := gameServers.Items[i]
		if gs.Status.State == mpsv1alpha1.GameServerStateActive {
			if gracefulTermination {
				gs.Status.State = mpsv1alpha1.GameServerStateGameCompleted
			} else {
				gs.Status.State = mpsv1alpha1.GameServerStateCrashed
			}
			err = k8sClient.Status().Update(ctx, &gs)
			Expect(err).ToNot(HaveOccurred())
			return
		}
	}
	Expect(true).To(BeFalse()) // should never get here
}

// allocateGameServer converts the state of a GameServer to Active
func allocateGameServer(ctx context.Context, buildID string) {
	var gameServers mpsv1alpha1.GameServerList
	err := k8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
	Expect(err).ToNot(HaveOccurred())
	for i := 0; i < len(gameServers.Items); i++ {
		gs := gameServers.Items[i]
		if gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
			gs.Status.State = mpsv1alpha1.GameServerStateActive
			err = k8sClient.Status().Update(ctx, &gs)
			Expect(err).ToNot(HaveOccurred())
			return
		}
	}
	Expect(true).To(BeFalse()) // should never get here
}

// updateGameServerBuild updates the GameServerBuild with the requested standingBy and max
func updateGameServerBuild(ctx context.Context, standingBy, max int, buildName string) {
	Eventually(func() error {
		gsb := getGameServerBuild(ctx, buildName)
		gsb.Spec.StandingBy = standingBy
		gsb.Spec.Max = max
		return k8sClient.Update(ctx, &gsb)
	}, timeout, interval).Should(Succeed())
}

// verifyTotalGameServerCount verifies the total number of game servers
func verifyTotalGameServerCount(ctx context.Context, buildID string, total int) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := k8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		return len(gameServers.Items) == total
	}, timeout, interval).Should(BeTrue())
}

// verifyStandindByActiveCount verifies that the number of standingBy and active game servers is equal to the expected
func verifyStandingByActiveByCount(ctx context.Context, buildID string, standingByCount, activeCount int) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := k8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		var currentActive, currentStandingBy int
		for i := 0; i < len(gameServers.Items); i++ {
			gs := gameServers.Items[i]
			if gs.Status.State == mpsv1alpha1.GameServerStateActive {
				currentActive++
			} else if gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
				currentStandingBy++
			}
		}
		return currentActive == activeCount && currentStandingBy == standingByCount
	}, timeout, interval).Should(BeTrue())
}

// getGameServerBuild returns the GameServerBuild with the given name
func getGameServerBuild(ctx context.Context, buildName string) mpsv1alpha1.GameServerBuild {
	var gameServerBuild mpsv1alpha1.GameServerBuild
	err := k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testnamespace}, &gameServerBuild)
	Expect(err).ToNot(HaveOccurred())
	return gameServerBuild
}

// getGameServer returns the GameServer with the given name
func getGameServer(ctx context.Context, gameServerName string) mpsv1alpha1.GameServer {
	var gameServer mpsv1alpha1.GameServer
	err := k8sClient.Get(ctx, types.NamespacedName{Name: gameServerName, Namespace: testnamespace}, &gameServer)
	Expect(err).ToNot(HaveOccurred())
	return gameServer
}

// updateInitializingGameServersToStandingBy sets all initializing game servers to standing by
func updateInitializingGameServersToStandingBy(ctx context.Context, buildID string) {
	var gameServers mpsv1alpha1.GameServerList
	err := k8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
	Expect(err).ToNot(HaveOccurred())
	for _, gameServer := range gameServers.Items {
		if gameServer.Status.State == "" || gameServer.Status.State == mpsv1alpha1.GameServerStateInitializing {
			gs := getGameServer(ctx, gameServer.Name) // getting the latest updated GameServer object
			gs.Status.State = mpsv1alpha1.GameServerStateStandingBy
			gs.Status.Health = mpsv1alpha1.Healthy
			err = k8sClient.Status().Update(ctx, &gs)
			Expect(err).ToNot(HaveOccurred())
		}
	}
}

// createTestGameServerBuild creates a GameServerBuild with the given name and ID.
func createTestGameServerBuild(buildName, buildID string, standingBy, max int, hostNetwork bool) mpsv1alpha1.GameServerBuild {
	return mpsv1alpha1.GameServerBuild{
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:    buildID,
			StandingBy: standingBy,
			Max:        max,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testcontainer",
							Image: os.Getenv("THUNDERNETES_SAMPLE_IMAGE"),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
					HostNetwork: hostNetwork,
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: testnamespace,
		},
	}
}
