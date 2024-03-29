package main

import (
	"context"
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
	It("should become Unhealthy, then transition to Healthy and then Unhealthy again", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createCrashingBuild(testBuildCrashingName, testCrashingBuildID, img))
		Expect(err).ToNot(HaveOccurred())

		// this test simulates the scenario where
		// a GameServerBuild becomes Unhealthy because of multiple crashes
		// user manually increases the CrashesToMarkUnhealthy so GameServerBuild transitions to Healthy again
		// multiple crashes occur, so the GameServerBuild becomes Unhealthy again
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

		// we are updating the GameServerBuild with a nil CrashesToMarkUnhealthy so the GameServerBuild will become Healthy
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildCrashingName, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.CrashesToMarkUnhealthy = nil
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		// so we expect it to be healthy again
		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildCrashingName, Namespace: testNamespace}, gsb)
			Expect(err).ToNot(HaveOccurred())
			g.Expect(gsb.Status.Health).To(Equal(mpsv1alpha1.BuildHealthy))
		}, 10*time.Second, interval).Should(Succeed())

		// we're setting the CrashesToMarkUnhealthy to 10 so that the
		// GameServerBuild will eventually become Unhealthy
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildCrashingName, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		crashes := 10
		gsb.Spec.CrashesToMarkUnhealthy = &crashes
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		// now, let's make sure that GameServerBuild is Unhealthy
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
	})
})

// createCrashingBuild creates a build which contains game servers that will crash on start
func createCrashingBuild(buildName, buildID, img string) *mpsv1alpha1.GameServerBuild {
	gsb := createTestBuild(buildName, buildID, img)
	gsb.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "sleep 0.5 && command_that_does_not_exist"}
	crashes := 5
	gsb.Spec.CrashesToMarkUnhealthy = &crashes
	return gsb
}
