package main

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// this file contains tests to verify that Unhealthy GameServers get deleted
// this test checks if StandingBy and Active GameServers that are marked as Unhealthy are properly deleted
var _ = Describe("GameServerBuild with Pods that get evicted", func() {
	testBuildEvictedName := "evicted"
	testBuildEvictedID := "8512e812-a12e-4b45-86c5-9d2b185e16f6"
	It("should delete the Unhealthy GameServers and replace them with Healthy ones", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createE2eBuild(testBuildEvictedName, testBuildEvictedID, img))
		Expect(err).ToNot(HaveOccurred())

		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildEvictedName,
				buildID:         testBuildEvictedID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// get the names of the game servers so we can evict the Pods
		// and later make sure that they disappeared
		var gsList mpsv1alpha1.GameServerList
		err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildEvictedName})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(gsList.Items)).To(Equal(2))

		gsNames := make(map[string]interface{})
		for _, gs := range gsList.Items {
			gsNames[gs.Name] = struct{}{}
		}

		// evict the Pods
		for _, gs := range gsList.Items {
			coreClient.PolicyV1().Evictions(gs.Namespace).Evict(ctx, &policyv1.Eviction{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gs.Name,
					Namespace: gs.Namespace,
				},
			})
		}

		// make sure 2 new servers were created to replace the ones that were deleted
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildEvictedName,
				buildID:         testBuildEvictedID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())

			// get the names of the new game servers
			var gsList mpsv1alpha1.GameServerList
			err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildEvictedName})
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
