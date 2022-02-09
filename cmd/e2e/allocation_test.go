package main

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("test GameServerBuild with allocation tests", Ordered, func() {
	testBuildAllocationName := "testbuildallocation"
	testBuildAllocationID := "85ffe8da-c82f-4035-86c5-9d2b5f42d6aa"
	var cert tls.Certificate
	ctx := context.Background()
	var kubeClient client.Client
	var coreClient *kubernetes.Clientset
	BeforeAll(func() {
		var err error
		cert, err = tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err = createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createE2eBuild(testBuildAllocationName, testBuildAllocationID, img))
		Expect(err).ToNot(HaveOccurred())
		coreClient, err = kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildAllocationName,
				buildID:         testBuildAllocationID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
	It("should return 400 with a non-GUID sessionID", func() {
		// allocating with a non-Guid sessionID, expecting 400
		sessionID1_5 := "notAGuid"
		Expect(allocate(testBuildAllocationID, sessionID1_5, cert)).To(Equal(fmt.Errorf("%s 400", invalidStatusCode)))
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildAllocationName,
				buildID:         testBuildAllocationID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

	})
	It("should return 400 with a non-GUID BuildID", func() {
		// allocating with a non-Guid BuildID, expecting 400
		sessionID1_6 := uuid.New().String()
		Expect(allocate("not_a_guid", sessionID1_6, cert)).To(Equal(fmt.Errorf("%s 400", invalidStatusCode)))
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildAllocationName,
				buildID:         testBuildAllocationID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})

	It("should return 404 with a non-existent BuildID", func() {
		// allocating on non-existent BuildID, expecting 404
		sessionID1_7 := uuid.New().String()
		Expect(allocate(uuid.New().String(), sessionID1_7, cert)).To(Equal(fmt.Errorf("%s 404", invalidStatusCode)))
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildAllocationName,
				buildID:         testBuildAllocationID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})

	It("should allocate properly and get one active", func() {
		// allocating correctly, expecting one active
		sessionID1_2 := uuid.New().String()
		Expect(allocate(testBuildAllocationID, sessionID1_2, cert)).To(Succeed())
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildAllocationName,
				buildID:         testBuildAllocationID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, testBuildAllocationID, 1)).To(Succeed())

	})
})
