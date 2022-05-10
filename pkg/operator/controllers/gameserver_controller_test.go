package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GameServer controller tests", func() {
	Context("testing creating a gameserver creates a pod", func() {
		buildName := randString(5)
		gsName := fmt.Sprintf("%s-%s", buildName, randString(5))
		buildID := string(uuid.NewUUID())
		It("should create pod", func() {
			ctx := context.Background()

			gs := generateGameServer(buildName, buildID, testnamespace, gsName)
			Expect(testk8sClient.Create(ctx, gs)).Should(Succeed())

			var pod corev1.Pod
			Eventually(func() bool {
				var pods corev1.PodList
				err := testk8sClient.List(ctx, &pods, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
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
