package v1alpha1

import (
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var _ = Describe("GameServerBuild webhook tests", func() {
	Context("testing validation webhooks for gameserverbuild", func() {

		It("validates unique buildID", FlakeAttempts(3), func() {
			// this test is marked as Flakey because the two GameServerBuild creation happen almost immediately
			// so the cache doesn't have time to update.
			buildName, buildID := getNewNameAndID()
			buildName2, _ := getNewNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			// make sure the new GameServerBuild is part of the cache
			gsb = createTestGameServerBuild(buildName2, buildID, 2, 4, false)
			err := k8sClient.Create(ctx, &gsb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errBuildIdUnique))
		})

		It("validates that updating the buildID is not allowed", func() {
			buildName, buildID := getNewNameAndID()
			_, buildID2 := getNewNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			Expect(k8sClient.Create(ctx, &gsb)).Should(Succeed())
			gsb.Spec.BuildID = buildID2
			err := k8sClient.Update(ctx, &gsb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errBuildIdImmutable))
		})

		It("validates that the port to expose exists", func() {
			buildName, buildID := getNewNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			gsb.Spec.PortsToExpose = append(gsb.Spec.PortsToExpose, 70)
			err := k8sClient.Create(ctx, &gsb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errPortsMatchingPortsToExpose))
		})

		It("validates that the port to expose has a name", func() {
			buildName, buildID := getNewNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			gsb.Spec.Template.Spec.Containers[0].Ports[0].Name = ""
			err := k8sClient.Create(ctx, &gsb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errNoPortName))
		})

		It("validates that the port to expose doesn't have a hostPort", func() {
			buildName, buildID := getNewNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 2, 4, false)
			gsb.Spec.Template.Spec.Containers[0].Ports[0].HostPort = 1000
			err := k8sClient.Create(ctx, &gsb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errNoHostPort))
		})

		It("validates that standingBy must be less than equal to max", func() {
			buildName, buildID := getNewNameAndID()
			gsb := createTestGameServerBuild(buildName, buildID, 5, 4, false)
			err := k8sClient.Create(ctx, &gsb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errStandingByLessThanMax))
		})

	})
})

// getNewNameAndID returns a new build name and ID
func getNewNameAndID() (string, string) {
	name := randString(5)
	buildID := string(uuid.NewUUID())
	return name, buildID
}

// randString creates a random string with lowercase characters
func randString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// createTestGameServerBuild creates a GameServerBuild with the given name and ID.
func createTestGameServerBuild(buildName, buildID string, standingBy, max int, hostNetwork bool) GameServerBuild {
	return GameServerBuild{
		Spec: GameServerBuildSpec{
			TitleID:       "test-title-id",
			BuildID:       buildID,
			StandingBy:    standingBy,
			Max:           max,
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
			Name:      buildName,
			Namespace: "default",
		},
	}
}
