package main

import (
	"context"
	"crypto/tls"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test verifies that the operator recovers correctly after a restart.
// GameServers running before the restart should continue to be tracked and managed.
// These tests must run in order since they simulate an operator restart mid-operation.
var _ = Describe("Operator restart recovery", Ordered, func() {
	testBuildRestartName := "operator-restart"
	testBuildRestartID := "a3ffe8da-c82f-4035-86c5-9d2b5f42d6a3"
	ctx := context.Background()
	kubeConfig := ctrl.GetConfigOrDie()
	kubeClient, err := createKubeClient(kubeConfig)
	Expect(err).ToNot(HaveOccurred())
	coreClient, err := kubernetes.NewForConfig(kubeConfig)
	Expect(err).ToNot(HaveOccurred())

	It("should create a build and allocate a server", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		gsb := createTestBuild(testBuildRestartName, testBuildRestartID, img)
		gsb.Spec.StandingBy = 2
		gsb.Spec.Max = 4
		err = kubeClient.Create(ctx, gsb)
		Expect(err).ToNot(HaveOccurred())

		// wait for standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildRestartName,
				buildID:         testBuildRestartID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocate a server
		sessionID := uuid.New().String()
		Expect(allocate(testBuildRestartID, sessionID, cert)).To(Succeed())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildRestartName,
				buildID:         testBuildRestartID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})

	It("should restart the operator and verify recovery", func() {
		// find the operator pod
		var podList corev1.PodList
		err := kubeClient.List(ctx, &podList, client.InNamespace(thundernetesSystemNamespace), client.MatchingLabels{
			"control-plane": "controller-manager",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(1))

		operatorPodName := podList.Items[0].Name

		// delete the operator pod (it will be restarted by the Deployment)
		err = kubeClient.Delete(ctx, &podList.Items[0])
		Expect(err).ToNot(HaveOccurred())

		// wait for the new operator pod to be ready
		Eventually(func(g Gomega) {
			var newPodList corev1.PodList
			err := kubeClient.List(ctx, &newPodList, client.InNamespace(thundernetesSystemNamespace), client.MatchingLabels{
				"control-plane": "controller-manager",
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(newPodList.Items)).To(Equal(1))
			// verify it's a new pod (different name due to restart)
			g.Expect(newPodList.Items[0].Name).ToNot(Equal(operatorPodName))
			g.Expect(newPodList.Items[0].Status.Phase).To(Equal(corev1.PodRunning))
			for _, c := range newPodList.Items[0].Status.ContainerStatuses {
				g.Expect(c.Ready).To(BeTrue())
			}
		}, 120*timeout, interval).Should(Succeed()) // longer timeout for pod restart
	})

	It("should have reconciled GameServers after restart", func() {
		// verify the build still has the correct state after operator restart
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildRestartName,
				buildID:         testBuildRestartID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// verify we can still allocate after operator restart
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		// the allocation API server is the same pod as the operator so it needs to be back up
		Eventually(func(g Gomega) {
			sessionID := uuid.New().String()
			g.Expect(allocate(testBuildRestartID, sessionID, cert)).To(Succeed())
		}, 120*timeout, interval).Should(Succeed()) // longer timeout, allocation API needs to reload

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildRestartName,
				buildID:         testBuildRestartID,
				standingByCount: 2,
				activeCount:     2,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, testBuildRestartID, 2)).To(Succeed())
	})
})
