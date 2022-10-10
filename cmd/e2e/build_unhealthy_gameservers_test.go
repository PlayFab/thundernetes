package main

import (
	"context"
	"crypto/tls"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// this file contains tests to verify that Unhealthy GameServers get deleted
// this test checks if StandingBy and Active GameServers that are marked as Unhealthy are properly deleted
var _ = Describe("Regular GameServerBuild", func() {
	testBuildWithUnhealthyGameServersName := "unhealthygameservers"
	testBuildWithUnhealthyGameServersID := "8512e812-c82f-4b45-86c5-9d2b1ae3d6f6"
	It("should delete the Unhealthy GameServers and replace them with Healthy ones", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createE2eBuild(testBuildWithUnhealthyGameServersName, testBuildWithUnhealthyGameServersID, img))
		Expect(err).ToNot(HaveOccurred())

		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildWithUnhealthyGameServersName,
				buildID:         testBuildWithUnhealthyGameServersID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update the standingBy to 3
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithUnhealthyGameServersName, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 3
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildWithUnhealthyGameServersName,
				buildID:         testBuildWithUnhealthyGameServersID,
				standingByCount: 3,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// get the names of the game servers so we can mark them as Unhealthy
		// and later make sure that they disappeared
		var gsList mpsv1alpha1.GameServerList
		err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildWithUnhealthyGameServersName})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(gsList.Items)).To(Equal(3))

		gsNames := make(map[string]interface{})
		for _, gs := range gsList.Items {
			gsNames[gs.Name] = struct{}{}
		}

		// mark these gameservers as Unhealthy
		// under normal circumstances, this can happen if they never send a heartbeat, if they are late in sending a heartbeat
		// or if they mark themselves as Unhealthy via the relevant GSDK call
		// check NodeAgent for relevant code
		for _, gs := range gsList.Items {
			patch := client.MergeFrom(gs.DeepCopy())
			gs.Status.Health = mpsv1alpha1.GameServerUnhealthy
			err = kubeClient.Status().Patch(ctx, &gs, patch)
			Expect(err).ToNot(HaveOccurred())
		}

		// make sure 3 new servers were created to replace the ones that were deleted
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildWithUnhealthyGameServersName,
				buildID:         testBuildWithUnhealthyGameServersID,
				standingByCount: 3,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())

			// get the names of the new game servers
			var gsList mpsv1alpha1.GameServerList
			err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildWithUnhealthyGameServersName})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(gsList.Items)).To(Equal(3))

			// make sure they have different names than the ones that were deleted
			for _, gs := range gsList.Items {
				g.Expect(gs.Status.Health).To(Equal(mpsv1alpha1.GameServerHealthy))
				_, ok := gsNames[gs.Name]
				g.Expect(ok).To(BeFalse())
			}

		}, timeout, interval).Should(Succeed())

		// allocate a game server
		sessionID := uuid.New().String()
		err = allocate(testBuildWithUnhealthyGameServersID, sessionID, cert)
		Expect(err).ToNot(HaveOccurred())

		// so we now should have 1 active and 3 standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildWithUnhealthyGameServersName,
				buildID:         testBuildWithUnhealthyGameServersID,
				standingByCount: 3,
				activeCount:     1,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, testBuildWithUnhealthyGameServersID, 1)).To(Succeed())

		// get the active game server so we can mark it as Unhealthy
		// and later make sure that it was deleted
		err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildWithUnhealthyGameServersName})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(gsList.Items)).To(Equal(4))

		// get the active game server
		var activeGs mpsv1alpha1.GameServer
		for _, gs := range gsList.Items {
			if gs.Status.State == mpsv1alpha1.GameServerStateActive {
				activeGs = gs
				break
			}
		}
		Expect(activeGs.Name).ToNot(BeEmpty())

		// mark this Active GameServer as Unhealthy
		// to verify that the controller is deleting GameServers that are Active but they turn Unhealthy
		patch = client.MergeFrom(activeGs.DeepCopy())
		activeGs.Status.Health = mpsv1alpha1.GameServerUnhealthy
		err = kubeClient.Status().Patch(ctx, &activeGs, patch)
		Expect(err).ToNot(HaveOccurred())

		// make sure the active was deleted and we have 3 standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildWithUnhealthyGameServersName,
				buildID:         testBuildWithUnhealthyGameServersID,
				standingByCount: 3,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())

			// get the names of the game servers
			var gsList mpsv1alpha1.GameServerList
			err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildWithUnhealthyGameServersName})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(gsList.Items)).To(Equal(3))

			// make sure they have different names than the active that we deleted
			for _, gs := range gsList.Items {
				g.Expect(gs.Status.Health).To(Equal(mpsv1alpha1.GameServerHealthy))
				g.Expect(gs.Name).ToNot(Equal(activeGs.Name))
			}

		}, timeout, interval).Should(Succeed())
	})
})

// this test verifies that GameServers that do not call the ReadyForPlayers() GSDK method (thus, they stay stuck in Initializing state)
// and are marked as Unhealthy for any reason (e.g. missing heartbeat)
// will eventually be deleted
var _ = Describe("GameServerBuild with Unhealthy GameServers without ReadyForPlayers", func() {
	testBuildUnhealthyGameServersWithoutReadyForPlayersName := "withoutreadyforplayersunhealthy"
	testBuildUnhealthyGameServersWithoutReadyForPlayersID := "85ffe8da-c82f-a12e-86c5-9d2b7652d6f8"
	It("should delete unhealthy GameServers and replace them with healthy ones", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createBuildWithoutReadyForPlayers(testBuildUnhealthyGameServersWithoutReadyForPlayersName, testBuildUnhealthyGameServersWithoutReadyForPlayersID, img))
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildUnhealthyGameServersWithoutReadyForPlayersName, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildUnhealthyGameServersWithoutReadyForPlayersName,
				buildID:           testBuildUnhealthyGameServersWithoutReadyForPlayersID,
				initializingCount: 2,
				standingByCount:   0,
				podRunningCount:   2,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// get the names of the game servers so we can mark them as Unhealthy
		// and later make sure that they disappeared
		var gsList mpsv1alpha1.GameServerList
		err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildUnhealthyGameServersWithoutReadyForPlayersName})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(gsList.Items)).To(Equal(2))

		gsNames := make(map[string]interface{})
		for _, gs := range gsList.Items {
			gsNames[gs.Name] = struct{}{}
		}

		// mark these GameServers as Unhealthy
		for _, gs := range gsList.Items {
			patch := client.MergeFrom(gs.DeepCopy())
			gs.Status.Health = mpsv1alpha1.GameServerUnhealthy
			err = kubeClient.Status().Patch(ctx, &gs, patch)
			Expect(err).ToNot(HaveOccurred())
		}

		// make sure 2 more servers were created to replace the ones that were deleted
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:         testBuildUnhealthyGameServersWithoutReadyForPlayersName,
				buildID:           testBuildUnhealthyGameServersWithoutReadyForPlayersID,
				initializingCount: 2,
				standingByCount:   0,
				podRunningCount:   2,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())

			// get the names of the new game servers
			var gsList mpsv1alpha1.GameServerList
			err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildUnhealthyGameServersWithoutReadyForPlayersName})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(gsList.Items)).To(Equal(2))

			// make sure they have different names than the ones that were deleted
			for _, gs := range gsList.Items {
				g.Expect(gs.Status.Health).To(Equal(mpsv1alpha1.GameServerHealthy))
				_, ok := gsNames[gs.Name]
				g.Expect(ok).To(BeFalse())
			}

		}, timeout, interval).Should(Succeed())

	})
})

// this test verifies that GameServers that do not integrate with GSDK and are marked as Unhealthy
// will eventually be deleted
var _ = Describe("GameServerBuild with GameServers without Gsdk", func() {
	testBuildWithoutGsdkName := "withoutgsdk"
	testBuildWithoutGsdkID := "8511e8da-c82f-a12e-86c5-9d2b76528356"
	It("should delete unhealthy GameServers and replace them with healthy ones", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createBuildWithoutGsdk(testBuildWithoutGsdkName, testBuildWithoutGsdkID, img))
		Expect(err).ToNot(HaveOccurred())

		// get the names of all the game servers so we can mark them as Unhealthy
		// this simulates the NodeAgent marking these GameServers as unhealthy
		// because they didn't send a heartbeat within a period of time (which will be the case for GameServers that don't implement GSDK)
		var gsList mpsv1alpha1.GameServerList
		Eventually(func(g Gomega) {
			err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildWithoutGsdkName})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(gsList.Items)).To(Equal(2))
		}, timeout, interval).Should(Succeed())

		// mark these GameServers as Unhealthy
		for _, gs := range gsList.Items {
			patch := client.MergeFrom(gs.DeepCopy())
			gs.Status.Health = mpsv1alpha1.GameServerUnhealthy
			err = kubeClient.Status().Patch(ctx, &gs, patch)
			Expect(err).ToNot(HaveOccurred())
		}

		// make sure that GameServerBuild is Unhealthy (since CrashesToMarkUnhealthy threshold was reached)
		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutGsdkName, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gsb.Status.Health).To(Equal(mpsv1alpha1.BuildUnhealthy))
			g.Expect(gsb.Status.CrashesCount >= 2).To(BeTrue())
		}, timeout, interval).Should(Succeed())

	})
})

// createBuildWithoutGsdk creates a GameServerBuild without GSDK
func createBuildWithoutGsdk(buildName, buildID, img string) *mpsv1alpha1.GameServerBuild {
	gsb := createTestBuild(buildName, buildID, img)
	gsb.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "sleep 3600"}
	crashes := 2
	gsb.Spec.CrashesToMarkUnhealthy = &crashes
	return gsb
}
