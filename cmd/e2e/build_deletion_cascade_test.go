package main

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test verifies that deleting a GameServerBuild cascades the deletion to all
// its child GameServers and their Pods via Kubernetes owner references.
var _ = Describe("GameServerBuild deletion cascade", func() {
	testBuildDeletionName := "deletion-cascade"
	testBuildDeletionID := "d1ffe8da-c82f-4035-86c5-9d2b5f42d6d1"
	It("should delete all child GameServers and Pods when the build is deleted", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		// create the build
		gsb := createTestBuild(testBuildDeletionName, testBuildDeletionID, img)
		gsb.Spec.StandingBy = 3
		gsb.Spec.Max = 4
		err = kubeClient.Create(ctx, gsb)
		Expect(err).ToNot(HaveOccurred())

		// wait for 3 standingBy servers
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildDeletionName,
				buildID:         testBuildDeletionID,
				standingByCount: 3,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// verify we have GameServers and Pods
		var gsList mpsv1alpha1.GameServerList
		err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildID: testBuildDeletionID}, client.InNamespace(testNamespace))
		Expect(err).ToNot(HaveOccurred())
		Expect(len(gsList.Items)).To(Equal(3))

		var podList corev1.PodList
		err = kubeClient.List(ctx, &podList, client.MatchingLabels{LabelBuildName: testBuildDeletionName}, client.InNamespace(testNamespace))
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(BeNumerically(">=", 3))

		// delete the GameServerBuild
		err = kubeClient.Delete(ctx, &mpsv1alpha1.GameServerBuild{
			ObjectMeta: gsb.ObjectMeta,
		})
		Expect(err).ToNot(HaveOccurred())

		// verify all GameServers are deleted via cascade
		Eventually(func(g Gomega) {
			var remainingGs mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &remainingGs, client.MatchingLabels{LabelBuildID: testBuildDeletionID}, client.InNamespace(testNamespace))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(remainingGs.Items)).To(Equal(0))
		}, timeout, interval).Should(Succeed())

		// verify all Pods are deleted via cascade
		Eventually(func(g Gomega) {
			var remainingPods corev1.PodList
			err := kubeClient.List(ctx, &remainingPods, client.MatchingLabels{LabelBuildName: testBuildDeletionName}, client.InNamespace(testNamespace))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(remainingPods.Items)).To(Equal(0))
		}, timeout, interval).Should(Succeed())
	})
})
