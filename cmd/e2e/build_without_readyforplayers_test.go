package main

import (
	"context"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Build without ReadyForPlayers GSDK call", func() {
	testBuildWithoutReadyForPlayers := "withoutreadyforplayers"
	testWithoutReadyForPlayersBuildID := "85ffe8da-c82f-4035-86c5-9d2b5f42d6f8"
	It("should have GameServers stuck in Initializing", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createBuildWithoutReadyForPlayers(testBuildWithoutReadyForPlayers, testWithoutReadyForPlayersBuildID, img))
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildWithoutReadyForPlayers,
				buildID:           testWithoutReadyForPlayersBuildID,
				initializingCount: 2,
				standingByCount:   0,
				podRunningCount:   2,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update the GameServerBuild to 4 standingBy
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 4
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildWithoutReadyForPlayers,
				buildID:           testWithoutReadyForPlayersBuildID,
				initializingCount: 4,
				standingByCount:   0,
				podRunningCount:   4,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update the GameServerBuild to 0 standingBy
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 0
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildWithoutReadyForPlayers,
				buildID:           testWithoutReadyForPlayersBuildID,
				initializingCount: 0,
				standingByCount:   0,
				podRunningCount:   0,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update the GameServerBuild to 2 standingBy again
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 2
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildWithoutReadyForPlayers,
				buildID:           testWithoutReadyForPlayersBuildID,
				initializingCount: 2,
				standingByCount:   0,
				podRunningCount:   2,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var gsList mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &gsList, client.MatchingLabels{"BuildName": testBuildWithoutReadyForPlayers})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(gsList.Items)).To(Equal(2))
			gs := gsList.Items[0]
			g.Expect(gs.Status.NodeName).ToNot(BeEmpty())
			g.Expect(net.ParseIP(gs.Status.PublicIP)).ToNot(BeNil())
		}, timeout, interval).Should(Succeed())

	})
})

// createBuildWithoutReadyForPlayers creates a GameServerBuild which game server process do not call ReadyForPlayers
func createBuildWithoutReadyForPlayers(buildName, buildID, img string) *mpsv1alpha1.GameServerBuild {
	gsb := createTestBuild(buildName, buildID, img)
	gsb.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name:  "SKIP_READY_FOR_PLAYERS",
			Value: "true",
		},
	}
	return gsb
}
