package main

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Build which sleeps before calling GSDK ReadyForPlayers", func() {
	testBuildSleepBeforeReadyForPlayersName := "sleepbeforereadyforplayers"
	testBuildSleepBeforeReadyForPlayersID := "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6"
	It("should scale as usual", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		err = kubeClient.Create(ctx, createBuildWithSleepBeforeReadyForPlayers(testBuildSleepBeforeReadyForPlayersName, testBuildSleepBeforeReadyForPlayersID, img))
		Expect(err).ToNot(HaveOccurred())

		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildSleepBeforeReadyForPlayersName,
				buildID:         testBuildSleepBeforeReadyForPlayersID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// update the standingBy to 3
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testBuildSleepBeforeReadyForPlayersName, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 3
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildSleepBeforeReadyForPlayersName,
				buildID:         testBuildSleepBeforeReadyForPlayersID,
				standingByCount: 3,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocate a game server
		sessionID2 := uuid.New().String()
		err = allocate(testBuildSleepBeforeReadyForPlayersID, sessionID2, cert)
		Expect(err).ToNot(HaveOccurred())

		// so we now should have 1 active and 3 standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildSleepBeforeReadyForPlayersName,
				buildID:         testBuildSleepBeforeReadyForPlayersID,
				standingByCount: 3,
				activeCount:     1,
				podRunningCount: 4,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, kubeClient, coreClient, testBuildSleepBeforeReadyForPlayersID, 1)).To(Succeed())

		// killing an Active game server
		err = stopActiveGameServer(ctx, kubeClient, coreClient, kubeConfig, testBuildSleepBeforeReadyForPlayersID)
		Expect(err).ToNot(HaveOccurred())

		// so we now should have 3 standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildSleepBeforeReadyForPlayersName,
				buildID:         testBuildSleepBeforeReadyForPlayersID,
				standingByCount: 3,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// make sure all GameServers have a Public IP and NodeName
		Eventually(func(g Gomega) {
			var gsList mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: testBuildSleepBeforeReadyForPlayersName})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(gsList.Items)).To(Equal(3))
			for _, gs := range gsList.Items {
				g.Expect(gs.Status.NodeName).ToNot(BeEmpty())
				g.Expect(net.ParseIP(gs.Status.PublicIP)).ToNot(BeNil())
			}
		}, timeout, interval).Should(Succeed())
	})
})

// createBuildWithSleepBeforeReadyForPlayers creates a build which game server process will sleep for a while before it calls ReadyForPlayers
// useful to track the Initializing state of the GameServers
func createBuildWithSleepBeforeReadyForPlayers(buildName, buildID, img string) *mpsv1alpha1.GameServerBuild {
	gsb := createTestBuild(buildName, buildID, img)
	gsb.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name:  "SLEEP_BEFORE_READY_FOR_PLAYERS",
			Value: "true",
		},
	}
	return gsb
}
