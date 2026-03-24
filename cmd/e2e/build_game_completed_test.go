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

// This test verifies the GameCompleted lifecycle:
// When a game server process exits gracefully (exit code 0), the controller sets state to GameCompleted,
// the build controller deletes it, and a replacement server is created to fill the standingBy count.
var _ = Describe("GameCompleted lifecycle", func() {
	testBuildGameCompletedName := "gamecompleted"
	testBuildGameCompletedID := "a1ffe8da-c82f-4035-86c5-9d2b5f42d6a1"
	It("should handle graceful server termination and replacement", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		// create a build where game servers will terminate gracefully after a short delay
		gsb := createTestBuild(testBuildGameCompletedName, testBuildGameCompletedID, img)
		gsb.Spec.StandingBy = 2
		gsb.Spec.Max = 4
		// TERMINATE_AFTER_SECONDS=15 means servers will exit with code 0
		// after a random delay between 20 and 30 seconds
		gsb.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{
				Name:  "TERMINATE_AFTER_SECONDS",
				Value: "15",
			},
		}
		err = kubeClient.Create(ctx, gsb)
		Expect(err).ToNot(HaveOccurred())

		// wait for the initial standingBy servers to be ready
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildGameCompletedName,
				buildID:         testBuildGameCompletedID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocate a server
		sessionID := uuid.New().String()
		Expect(allocate(testBuildGameCompletedID, sessionID, cert)).To(Succeed())

		// verify we have 1 active and 2 standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildGameCompletedName,
				buildID:         testBuildGameCompletedID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, testBuildGameCompletedID, 1)).To(Succeed())

		// get the active GameServer name so we can verify it disappears
		var gsList mpsv1alpha1.GameServerList
		err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildID: testBuildGameCompletedID})
		Expect(err).ToNot(HaveOccurred())
		var activeGsName string
		for _, gs := range gsList.Items {
			if gs.Status.State == mpsv1alpha1.GameServerStateActive {
				activeGsName = gs.Name
				break
			}
		}
		Expect(activeGsName).ToNot(BeEmpty())

		// the active server should gracefully exit (exit code 0) after TERMINATE_AFTER_SECONDS
		// and transition to GameCompleted, then be deleted by the controller
		// once deleted, the controller should create a new standingBy to fill up to 2
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildGameCompletedName,
				buildID:         testBuildGameCompletedID,
				standingByCount: 2,
				activeCount:     0,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())

			// verify that the previously active GameServer has been deleted
			var currentGsList mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &currentGsList, client.MatchingLabels{LabelBuildID: testBuildGameCompletedID})
			g.Expect(err).ToNot(HaveOccurred())
			for _, gs := range currentGsList.Items {
				g.Expect(gs.Name).ToNot(Equal(activeGsName))
			}
		}, 120*timeout, interval).Should(Succeed()) // longer timeout for auto-termination
	})
})
