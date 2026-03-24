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

// This test verifies that multiple GameServerBuilds can coexist independently.
// Allocating from one build should not affect the other, and deleting one should not impact the other.
var _ = Describe("Multiple concurrent GameServerBuilds", func() {
	testBuildAName := "multibuild-a"
	testBuildAID := "b1ffe8da-c82f-4035-86c5-9d2b5f42d6b1"
	testBuildBName := "multibuild-b"
	testBuildBID := "b2ffe8da-c82f-4035-86c5-9d2b5f42d6b2"
	It("should keep builds independent", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		// create both builds simultaneously
		gsbA := createTestBuild(testBuildAName, testBuildAID, img)
		gsbA.Spec.StandingBy = 2
		gsbA.Spec.Max = 4
		err = kubeClient.Create(ctx, gsbA)
		Expect(err).ToNot(HaveOccurred())

		gsbB := createTestBuild(testBuildBName, testBuildBID, img)
		gsbB.Spec.StandingBy = 2
		gsbB.Spec.Max = 4
		err = kubeClient.Create(ctx, gsbB)
		Expect(err).ToNot(HaveOccurred())

		// verify both builds reach their standingBy count
		Eventually(func(g Gomega) {
			stateA := buildState{
				buildName:       testBuildAName,
				buildID:         testBuildAID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, stateA)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			stateB := buildState{
				buildName:       testBuildBName,
				buildID:         testBuildBID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, stateB)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocate from Build A
		sessionIDA := uuid.New().String()
		Expect(allocate(testBuildAID, sessionIDA, cert)).To(Succeed())

		// verify Build A has 1 active, Build B is unaffected
		Eventually(func(g Gomega) {
			stateA := buildState{
				buildName:       testBuildAName,
				buildID:         testBuildAID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, stateA)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, testBuildAID, 1)).To(Succeed())

		// verify Build B is unaffected
		Eventually(func(g Gomega) {
			stateB := buildState{
				buildName:       testBuildBName,
				buildID:         testBuildBID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, stateB)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocate from Build B
		sessionIDB := uuid.New().String()
		Expect(allocate(testBuildBID, sessionIDB, cert)).To(Succeed())

		// verify Build B has 1 active, Build A still has 1 active
		Eventually(func(g Gomega) {
			stateA := buildState{
				buildName:       testBuildAName,
				buildID:         testBuildAID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, stateA)).To(Succeed())
			stateB := buildState{
				buildName:       testBuildBName,
				buildID:         testBuildBID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, stateB)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, testBuildBID, 1)).To(Succeed())

		// delete Build A
		err = kubeClient.Delete(ctx, &mpsv1alpha1.GameServerBuild{
			ObjectMeta: gsbA.ObjectMeta,
		})
		Expect(err).ToNot(HaveOccurred())

		// verify Build A GameServers are all gone
		Eventually(func(g Gomega) {
			var gsList mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildID: testBuildAID}, client.InNamespace(testNamespace))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(gsList.Items)).To(Equal(0))
		}, timeout, interval).Should(Succeed())

		// verify Build B is completely unaffected
		Eventually(func(g Gomega) {
			stateB := buildState{
				buildName:       testBuildBName,
				buildID:         testBuildBID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, stateB)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
})
