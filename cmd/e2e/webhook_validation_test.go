package main

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test verifies that the admission webhooks correctly reject invalid GameServerBuild configurations.
var _ = Describe("Webhook validation", func() {
	ctx := context.Background()
	kubeConfig := ctrl.GetConfigOrDie()
	kubeClient, err := createKubeClient(kubeConfig)
	Expect(err).ToNot(HaveOccurred())

	It("should reject a GameServerBuild with standingBy greater than max", func() {
		gsb := &mpsv1alpha1.GameServerBuild{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "webhook-standingby-gt-max",
				Namespace: testNamespace,
			},
			Spec: mpsv1alpha1.GameServerBuildSpec{
				BuildID:       "f1ffe8da-c82f-4035-86c5-9d2b5f42d6f1",
				TitleID:       "1E03",
				PortsToExpose: []int32{80},
				StandingBy:    5,
				Max:           2, // max < standingBy
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
		err := kubeClient.Create(ctx, gsb)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid API error, got: %v", err)
	})

	It("should reject a GameServerBuild with duplicate BuildID in the same namespace", func() {
		duplicateBuildID := "f2ffe8da-c82f-4035-86c5-9d2b5f42d6f2"

		// create the first build
		gsb1 := createTestBuild("webhook-dup-id-1", duplicateBuildID, img)
		err := kubeClient.Create(ctx, gsb1)
		Expect(err).ToNot(HaveOccurred())

		// wait for it to be created
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       "webhook-dup-id-1",
				buildID:         duplicateBuildID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// try to create a second build with the same BuildID
		gsb2 := createTestBuild("webhook-dup-id-2", duplicateBuildID, img)
		err = kubeClient.Create(ctx, gsb2)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid API error for duplicate BuildID, got: %v", err)

		// cleanup
		err = kubeClient.Delete(ctx, gsb1)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should reject a GameServerBuild with ports that have hostPort set", func() {
		gsb := &mpsv1alpha1.GameServerBuild{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "webhook-hostport-set",
				Namespace: testNamespace,
			},
			Spec: mpsv1alpha1.GameServerBuildSpec{
				BuildID:       "f3ffe8da-c82f-4035-86c5-9d2b5f42d6f3",
				TitleID:       "1E03",
				PortsToExpose: []int32{80},
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
										HostPort:      8080, // should not be set
									},
								},
							},
						},
					},
				},
			},
		}
		err := kubeClient.Create(ctx, gsb)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid API error for hostPort set, got: %v", err)
	})

	It("should reject a GameServerBuild with ports missing names", func() {
		gsb := &mpsv1alpha1.GameServerBuild{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "webhook-port-no-name",
				Namespace: testNamespace,
			},
			Spec: mpsv1alpha1.GameServerBuildSpec{
				BuildID:       "f4ffe8da-c82f-4035-86c5-9d2b5f42d6f4",
				TitleID:       "1E03",
				PortsToExpose: []int32{80},
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
										ContainerPort: 80,
										// Name is missing
									},
								},
							},
						},
					},
				},
			},
		}
		err := kubeClient.Create(ctx, gsb)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid API error for missing port name, got: %v", err)
	})

	It("should allow GameServerBuilds with different BuildIDs in the same namespace", func() {
		// create two builds with different IDs - should succeed
		gsb1 := createTestBuild("webhook-diff-id-1", "f5ffe8da-c82f-4035-86c5-9d2b5f42d6f5", img)
		err := kubeClient.Create(ctx, gsb1)
		Expect(err).ToNot(HaveOccurred())

		gsb2 := createTestBuild("webhook-diff-id-2", "f6ffe8da-c82f-4035-86c5-9d2b5f42d6f6", img)
		err = kubeClient.Create(ctx, gsb2)
		Expect(err).ToNot(HaveOccurred())

		// cleanup
		err = kubeClient.Delete(ctx, gsb1)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Delete(ctx, gsb2)
		Expect(err).ToNot(HaveOccurred())

		// verify they are gone
		Eventually(func(g Gomega) {
			var gsList mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: "webhook-diff-id-1"}, client.InNamespace(testNamespace))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(gsList.Items)).To(Equal(0))
		}, timeout, interval).Should(Succeed())
	})
})
