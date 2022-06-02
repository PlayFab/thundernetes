package main

import (
	"context"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Crashing Build", func() {
	testBuildCrashingName := "crashing"
	testCrashingBuildID := "85ffe8da-c82f-4035-86c5-9d2b5f42d6f7"
	It("should become unhealthy", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createCrashingBuild(testBuildCrashingName, testCrashingBuildID, img))
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildCrashingName, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			crashesEqualOrLargerThan5 := gsb.Status.CrashesCount >= 5
			g.Expect(crashesEqualOrLargerThan5).To(BeTrue())
			state := buildState{
				buildName:         testBuildCrashingName,
				buildID:           testCrashingBuildID,
				initializingCount: 0,
				standingByCount:   0,
				podRunningCount:   0,
				gsbHealth:         mpsv1alpha1.BuildUnhealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, 45*time.Second, interval).Should(Succeed()) // bigger timeout because of the time crashes take to occur and captured by the controller

		// we are updating the GameServerBuild to be able to have more crashes for it to become Unhealthy
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildCrashingName, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.CrashesToMarkUnhealthy = 10

		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		// so we expect it to be healthy again
		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildCrashingName, Namespace: testNamespace}, gsb)
			Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildCrashingName,
				buildID:           testCrashingBuildID,
				initializingCount: 0,
				standingByCount:   0,
				podRunningCount:   0,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, 10*time.Second, interval).Should(Succeed())

		// but only temporarily, since the game servers will continue to crash
		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildCrashingName, Namespace: testNamespace}, gsb)
			Expect(err).ToNot(HaveOccurred())
			var crashesEqualOrLargerThan10 bool = gsb.Status.CrashesCount >= 10
			g.Expect(crashesEqualOrLargerThan10).To(BeTrue())
			state := buildState{
				buildName:         testBuildCrashingName,
				buildID:           testCrashingBuildID,
				initializingCount: 0,
				standingByCount:   0,
				podRunningCount:   0,
				gsbHealth:         mpsv1alpha1.BuildUnhealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, 40*time.Second, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var gsList mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &gsList, client.MatchingLabels{"BuildName": testBuildCrashingName})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(gsList.Items)).To(Equal(2))
			gs := gsList.Items[0]
			g.Expect(gs.Status.NodeName).ToNot(BeEmpty())
			g.Expect(net.ParseIP(gs.Status.PublicIP)).ToNot(BeNil())
		}, 10*time.Second, interval).Should(Succeed())
	})
})

// createCrashingBuild creates a build which contains game servers that will crash on start
func createCrashingBuild(buildName, buildID, img string) *mpsv1alpha1.GameServerBuild {
	gsb := createTestBuild(buildName, buildID, img)
	gsb.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "sleep 2 && command_that_does_not_exist"}
	gsb.Spec.CrashesToMarkUnhealthy = 5
	return gsb
}
