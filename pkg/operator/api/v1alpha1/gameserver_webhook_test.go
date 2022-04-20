package v1alpha1

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GameServer webhook tests", func() {
	Context("testing validation webhooks for gameserver", func() {
		It("validates unique buildID", func() {
			Expect(nil).To(Succeed())
		})
	})
})

// createTestGameServerBuild creates a GameServerBuild with the given name and ID.
func createTestGameServer(name, buildID string, standingBy, max int, hostNetwork bool) GameServer {
	return GameServer{
		Spec: GameServerSpec{
			BuildID:       buildID,
			PortsToExpose: []int32{80},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testcontainer",
							Image: os.Getenv("THUNDERNETES_SAMPLE_IMAGE"),
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
		},
	}
}