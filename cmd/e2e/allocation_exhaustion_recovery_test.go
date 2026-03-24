package main

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test verifies the allocation exhaustion and recovery scenario:
// 1. Allocate all standingBy servers until exhaustion (429 error)
// 2. Scale up the build to add more capacity
// 3. Verify allocation succeeds again after new standingBy servers are available
var _ = Describe("Allocation exhaustion and recovery", func() {
	testBuildExhaustionName := "alloc-exhaustion"
	testBuildExhaustionID := "a4ffe8da-c82f-4035-86c5-9d2b5f42d6a4"
	It("should recover from allocation exhaustion after scaling up", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		// create a small build with standingBy=2 and max=2
		gsb := createTestBuild(testBuildExhaustionName, testBuildExhaustionID, img)
		gsb.Spec.StandingBy = 2
		gsb.Spec.Max = 2
		err = kubeClient.Create(ctx, gsb)
		Expect(err).ToNot(HaveOccurred())

		// wait for standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildExhaustionName,
				buildID:         testBuildExhaustionID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocate both standingBy servers
		sessionID1 := uuid.New().String()
		Expect(allocate(testBuildExhaustionID, sessionID1, cert)).To(Succeed())

		sessionID2 := uuid.New().String()
		Expect(allocate(testBuildExhaustionID, sessionID2, cert)).To(Succeed())

		// verify all servers are active and standingBy is 0
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildExhaustionName,
				buildID:         testBuildExhaustionID,
				standingByCount: 0,
				activeCount:     2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// try to allocate one more - should fail with 429
		sessionID3 := uuid.New().String()
		Expect(allocate(testBuildExhaustionID, sessionID3, cert)).To(Equal(fmt.Errorf("%s 429", invalidStatusCode)))

		// now scale up the build to add more capacity
		gsbObj := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildExhaustionName, Namespace: testNamespace}, gsbObj)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsbObj.DeepCopy())
		gsbObj.Spec.Max = 4
		gsbObj.Spec.StandingBy = 2
		err = kubeClient.Patch(ctx, gsbObj, patch)
		Expect(err).ToNot(HaveOccurred())

		// wait for new standingBy servers to come up
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildExhaustionName,
				buildID:         testBuildExhaustionID,
				standingByCount: 2,
				activeCount:     2,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// now allocation should succeed again
		sessionID4 := uuid.New().String()
		Expect(allocate(testBuildExhaustionID, sessionID4, cert)).To(Succeed())

		// verify we now have 3 actives and 1 standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildExhaustionName,
				buildID:         testBuildExhaustionID,
				standingByCount: 1,
				activeCount:     3,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
})
