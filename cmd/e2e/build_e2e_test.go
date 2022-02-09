package main

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("E2E Build", func() {
	testBuild1Name := "testbuild"
	test1BuildID := "85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"
	It("should scale as usual", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createE2eBuild(testBuild1Name, test1BuildID, img))
		Expect(err).ToNot(HaveOccurred())

		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update with 4 standingBy
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuild1Name, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 4
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 4,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update with 3 standingBy - 1 should be removed
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuild1Name, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 3
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 3,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocating
		sessionID1 := uuid.New().String()
		Expect(allocate(test1BuildID, sessionID1, cert)).To(Succeed())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 3,
				podRunningCount: 4,
				activeCount:     1,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, test1BuildID, 1)).To(Succeed())

		// allocating with same session ID, no more actives
		Expect(allocate(test1BuildID, sessionID1, cert)).To(Succeed())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 3,
				podRunningCount: 4,
				activeCount:     1,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocating with a new session ID
		sessionID2 := uuid.New().String()
		Expect(allocate(test1BuildID, sessionID2, cert)).To(Succeed())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 2,
				podRunningCount: 4,
				activeCount:     2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, test1BuildID, 2)).To(Succeed())

		// allocating with a new session ID - we should have 3 actives
		sessionID3 := uuid.New().String()
		Expect(allocate(test1BuildID, sessionID3, cert)).To(Succeed())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 1,
				podRunningCount: 4,
				activeCount:     3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, test1BuildID, 3)).To(Succeed())

		// allocating with a new session ID - 4 actives
		sessionID4 := uuid.New().String()
		Expect(allocate(test1BuildID, sessionID4, cert)).To(Succeed())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 0,
				podRunningCount: 4,
				activeCount:     4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, test1BuildID, 4)).To(Succeed())

		// allocating with a new session ID - we have 0 standing by so we should get a 429
		sessionID5 := uuid.New().String()
		Expect(allocate(test1BuildID, sessionID5, cert)).To(Equal(fmt.Errorf("%s 429", invalidStatusCode)))

		// updating with 3 max - we never kill actives so we should stay with 4 actives
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuild1Name, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.Max = 3
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				activeCount:     4,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// updating with 5 max - we should get 1 standingBy
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuild1Name, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.Max = 5
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 1,
				activeCount:     4,
				podRunningCount: 5,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// updating with 4 max and 2 standingBy - since max is 4, we should have 0 standingBy
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuild1Name, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.Max = 4
		gsb.Spec.StandingBy = 2
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				activeCount:     4,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// updating with 5 max and 2 standingBy - since we have 4 actives, we should have 1 standingBy
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuild1Name, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.Max = 5
		gsb.Spec.StandingBy = 2
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 1,
				activeCount:     4,
				podRunningCount: 5,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// Killing an active gameserver from Build
		Expect(stopActiveGameServer(ctx, kubeClient, coreClient, kubeConfig, test1BuildID)).To(Succeed())
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 2,
				activeCount:     3,
				podRunningCount: 5,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// Killing another active gameserver from Build
		Expect(stopActiveGameServer(ctx, kubeClient, coreClient, kubeConfig, test1BuildID)).To(Succeed())
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuild1Name,
				buildID:         test1BuildID,
				standingByCount: 2,
				activeCount:     2,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

	})
})

func createE2eBuild(buildname, buildID, img string) *mpsv1alpha1.GameServerBuild {
	return &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildname,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       buildID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: containerName, PortName: portKey}},
			BuildMetadata: []mpsv1alpha1.BuildMetadataItem{
				{Key: "metadatakey1", Value: "metadatavalue1"},
				{Key: "metadatakey2", Value: "metadatavalue2"},
				{Key: "metadatakey3", Value: "metadatavalue3"},
			},
			StandingBy: 2,
			Max:        4,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           img,
							Name:            containerName,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          portKey,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}
