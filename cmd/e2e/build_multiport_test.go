package main

import (
	"context"
	"crypto/tls"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test verifies that a GameServerBuild with multiple ports exposed works correctly.
// The controller should allocate separate host ports for each exposed container port.
var _ = Describe("Build with multiple exposed ports", func() {
	testBuildMultiportName := "multiport"
	testBuildMultiportID := "e1ffe8da-c82f-4035-86c5-9d2b5f42d6e1"
	It("should allocate host ports for all exposed ports", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		// create a build with two exposed ports
		gsb := createBuildWithMultiplePorts(testBuildMultiportName, testBuildMultiportID, img)
		err = kubeClient.Create(ctx, gsb)
		Expect(err).ToNot(HaveOccurred())

		// wait for standingBy servers
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildMultiportName,
				buildID:         testBuildMultiportID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// verify that each Pod has host ports assigned for both exposed ports
		Eventually(func(g Gomega) {
			var pods corev1.PodList
			err := kubeClient.List(ctx, &pods, client.MatchingLabels{LabelBuildName: testBuildMultiportName}, client.InNamespace(testNamespace))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(pods.Items)).To(Equal(2))

			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					continue
				}
				// check that the main container has two ports with non-zero host ports
				var portsWithHostPort int
				for _, container := range pod.Spec.Containers {
					if container.Name == containerName {
						for _, p := range container.Ports {
							if p.HostPort != 0 {
								portsWithHostPort++
							}
						}
					}
				}
				g.Expect(portsWithHostPort).To(Equal(2), "expected 2 ports with host ports assigned, pod: %s", pod.Name)
			}
		}, timeout, interval).Should(Succeed())

		// allocate a server
		sessionID := uuid.New().String()
		Expect(allocate(testBuildMultiportID, sessionID, cert)).To(Succeed())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildMultiportName,
				buildID:         testBuildMultiportID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, testBuildMultiportID, 1)).To(Succeed())

		// verify the GameServer status has ports listed
		Eventually(func(g Gomega) {
			var gsList mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildID: testBuildMultiportID})
			g.Expect(err).ToNot(HaveOccurred())
			for _, gs := range gsList.Items {
				// the Ports field should contain entries for both ports
				g.Expect(gs.Status.Ports).ToNot(BeEmpty())
			}
		}, timeout, interval).Should(Succeed())
	})
})

// createBuildWithMultiplePorts creates a GameServerBuild with two exposed ports
func createBuildWithMultiplePorts(buildName, buildID, img string) *mpsv1alpha1.GameServerBuild {
	gsb := createTestBuild(buildName, buildID, img)
	// add a second port (the game server may not actually listen on it, but the port
	// allocation and host port assignment should still work)
	gsb.Spec.PortsToExpose = []int32{80, 81}
	gsb.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{
			Name:          portKey,
			ContainerPort: 80,
		},
		{
			Name:          "queryport",
			ContainerPort: 81,
		},
	}
	return gsb
}
