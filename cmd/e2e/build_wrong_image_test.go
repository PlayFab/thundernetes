package main

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// this test checks if the controller will not go into a loop of deleting/creating Pods
// when the image is wrong. This is checked by consistenly checking if we have the same number of GameServers
var _ = Describe("GameServerBuild with a wrong image", func() {
	testBuildWrongImageName := "wrongimage"
	testBuildWrongImageID := "8512e812-a2c1-4b45-86c5-9d2b12e3d6f6"
	It("should not create any new Pods and should not delete the GameServers", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createE2eBuild(testBuildWrongImageName, testBuildWrongImageID, "wrongimage"))
		Expect(err).ToNot(HaveOccurred())

		Consistently(func(g Gomega) {
			state := buildState{
				buildName:         testBuildWrongImageName,
				buildID:           testBuildWrongImageID,
				pendingCount:      2,
				initializingCount: 0,
				standingByCount:   0,
				podRunningCount:   0,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, time.Second*5, interval).Should(Succeed())
	})
})
