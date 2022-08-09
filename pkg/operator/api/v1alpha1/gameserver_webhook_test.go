package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var _ = Describe("GameServer webhook tests", func() {
	Context("testing validation webhooks for gameserver", func() {
		It("validates that a gameserver must have an ownerreference to a gameserverbuild", func() {
			name, buildID := getNewNameAndID()
			gs := createTestGameServer(name, buildID, false)
			gs.ObjectMeta.OwnerReferences = make([]metav1.OwnerReference, 0)
			err := k8sClient.Create(ctx, &gs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errNoOwner))
		})

		It("validates that the port to expose exists", func() {
			name, buildID := getNewNameAndID()
			gs := createTestGameServer(name, buildID, false)
			gs.Spec.PortsToExpose = append(gs.Spec.PortsToExpose, 70)
			err := k8sClient.Create(ctx, &gs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errPortsMatchingPortsToExpose))
		})

		It("validates that the port to expose does not need to exist if the HostNetwork is enabled", func() {
			name, buildID := getNewNameAndID()
			gs := createTestGameServer(name, buildID, true)
			gs.Spec.PortsToExpose = append(gs.Spec.PortsToExpose, 70)
			err := k8sClient.Create(ctx, &gs)
			Expect(err).To(Succeed())
		})

		It("validates that the port to expose has a name", func() {
			name, buildID := getNewNameAndID()
			gs := createTestGameServer(name, buildID, false)
			gs.Spec.Template.Spec.Containers[0].Ports[0].Name = ""
			err := k8sClient.Create(ctx, &gs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errNoPortName))
		})
	})
})

// createTestGameServer creates a GameServerBuild with the given name and ID.
func createTestGameServer(name, buildID string, hostNetwork bool) GameServer {
	return GameServer{
		Spec: GameServerSpec{
			TitleID:       "test-title-id",
			BuildID:       buildID,
			PortsToExpose: []int32{80},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testcontainer",
							Image: "testimage",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Name:          "gamePort",
								},
							},
						},
					},
					HostNetwork: hostNetwork,
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(
					&GameServerBuild{
						ObjectMeta: metav1.ObjectMeta{
							Name: randString(5),
							UID:  uuid.NewUUID()}},
					schema.GroupVersionKind{
						Group:   "mps.playfab.com",
						Version: "v1alpha1",
						Kind:    "GameServerBuild",
					}),
			},
		},
	}
}
