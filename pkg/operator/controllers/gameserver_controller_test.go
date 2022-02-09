package controllers

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
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
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{"label1": "value1", "label2": "value2"},
							Annotations: map[string]string{"annotation1": "value1", "annotation2": "value2"},
						},
						Spec: corev1.PodSpec{
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
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      buildName,
					Namespace: testnamespace,
					Labels:    map[string]string{LabelBuildName: buildName},
				},
			}

			Expect(k8sClient.Create(ctx, &gs)).Should(Succeed())

			var pod corev1.Pod
			Eventually(func() bool {
				var pods corev1.PodList
				err := k8sClient.List(ctx, &pods, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
				Expect(err).ToNot(HaveOccurred())
				if len(pods.Items) == 1 {
					pod = pods.Items[0]
				}
				return len(pods.Items) == 1
			}, timeout, interval).Should(BeTrue())

			Expect(pod.Labels["label1"]).To(Equal("value1"))
			Expect(pod.Labels["label2"]).To(Equal("value2"))

			Expect(pod.Annotations["annotation1"]).To(Equal("value1"))
			Expect(pod.Annotations["annotation2"]).To(Equal("value2"))

			Expect(pod.Labels[LabelBuildID]).To(Equal(buildID))
			Expect(pod.Labels[LabelBuildName]).To(Equal(buildName))
			Expect(pod.Labels[LabelOwningGameServer]).To(Equal(gs.Name))
		})
	})
})
