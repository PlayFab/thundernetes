package controllers

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GameServer controller tests", func() {
	Context("testing creating a gameserver creates a pod", func() {
		buildName := randString(5)
		buildID := string(uuid.NewUUID())
		It("should create pod", func() {
			ctx := context.Background()
			gs := mpsv1alpha1.GameServer{
				Spec: mpsv1alpha1.GameServerSpec{
					BuildID: buildID,
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "testcontainer",
								Image: os.Getenv("THUNDERNETES_SAMPLE_IMAGE"),
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 80,
									},
								},
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      buildName,
					Namespace: testnamespace,
				},
			}

			Expect(k8sClient.Create(ctx, &gs)).Should(Succeed())

			Eventually(func() bool {
				var pods corev1.PodList
				err := k8sClient.List(ctx, &pods, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
				Expect(err).ToNot(HaveOccurred())
				return len(pods.Items) == 1
			}, timeout, interval).Should(BeTrue())

		})
	})
})
