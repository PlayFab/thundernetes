package main

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Build without ReadyForPlayers GSDK call", func() {
	testBuildWithoutReadyForPlayers := "withoutreadyforplayers"
	testWithoutReadyForPlayersBuildID := "85ffe8da-c82f-4035-86c5-9d2b5f42d6f8"
	It("should have GameServers stuck in Initializing", func() {
		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createBuildWithoutReadyForPlayers(testBuildWithoutReadyForPlayers, testWithoutReadyForPlayersBuildID, img))
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildWithoutReadyForPlayers,
				buildID:           testWithoutReadyForPlayersBuildID,
				initializingCount: 2,
				standingByCount:   0,
				podRunningCount:   2,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update the GameServerBuild to 4 standingBy
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 4
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildWithoutReadyForPlayers,
				buildID:           testWithoutReadyForPlayersBuildID,
				initializingCount: 4,
				standingByCount:   0,
				podRunningCount:   4,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update the GameServerBuild to 0 standingBy
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 0
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildWithoutReadyForPlayers,
				buildID:           testWithoutReadyForPlayersBuildID,
				initializingCount: 0,
				standingByCount:   0,
				podRunningCount:   0,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update the GameServerBuild to 2 standingBy again
		gsb = &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch = client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 2
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			gsb := &mpsv1alpha1.GameServerBuild{}
			err := kubeClient.Get(ctx, client.ObjectKey{Name: testBuildWithoutReadyForPlayers, Namespace: testNamespace}, gsb)
			g.Expect(err).ToNot(HaveOccurred())
			state := buildState{
				buildName:         testBuildWithoutReadyForPlayers,
				buildID:           testWithoutReadyForPlayersBuildID,
				initializingCount: 2,
				standingByCount:   0,
				podRunningCount:   2,
				gsbHealth:         mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
})

// createBuildWithoutReadyForPlayers creates a GameServerBuild which game server process do not call ReadyForPlayers
func createBuildWithoutReadyForPlayers(buildName, buildID, img string) *mpsv1alpha1.GameServerBuild {
	return &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       buildID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: containerName, PortName: portKey}},
			StandingBy:    2,
			Max:           4,
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
							Env: []corev1.EnvVar{
								{
									Name:  "SKIP_READY_FOR_PLAYERS",
									Value: "true",
								},
							},
						},
					},
				},
			},
		},
	}

}
