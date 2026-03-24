package main

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test verifies that BuildMetadata set on a GameServerBuild is propagated
// all the way through to the game server container via the GSDK config.
// The netcore sample prints all GSDK config settings including metadata keys,
// so we can verify the metadata appears in the container logs.
var _ = Describe("BuildMetadata end-to-end propagation", func() {
	testBuildMetadataName := "metadata-propagation"
	testBuildMetadataID := "c1ffe8da-c82f-4035-86c5-9d2b5f42d6c1"
	It("should propagate metadata from GameServerBuild to game server container", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		// create a build with specific metadata
		gsb := createE2eBuild(testBuildMetadataName, testBuildMetadataID, img)
		// createE2eBuild already includes metadata: metadatakey1/2/3
		err = kubeClient.Create(ctx, gsb)
		Expect(err).ToNot(HaveOccurred())

		// wait for standingBy servers
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildMetadataName,
				buildID:         testBuildMetadataID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// verify metadata is present on GameServer objects
		var gsList mpsv1alpha1.GameServerList
		err = kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildID: testBuildMetadataID}, client.InNamespace(testNamespace))
		Expect(err).ToNot(HaveOccurred())
		Expect(len(gsList.Items)).To(BeNumerically(">=", 2))

		for _, gs := range gsList.Items {
			Expect(len(gs.Spec.BuildMetadata)).To(Equal(3))
			metadataMap := make(map[string]string)
			for _, item := range gs.Spec.BuildMetadata {
				metadataMap[item.Key] = item.Value
			}
			Expect(metadataMap["metadatakey1"]).To(Equal("metadatavalue1"))
			Expect(metadataMap["metadatakey2"]).To(Equal("metadatavalue2"))
			Expect(metadataMap["metadatakey3"]).To(Equal("metadatavalue3"))
		}

		// allocate a server to trigger ReadyForPlayers which prints GSDK config
		sessionID := uuid.New().String()
		Expect(allocate(testBuildMetadataID, sessionID, cert)).To(Succeed())

		// wait for allocation
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildMetadataName,
				buildID:         testBuildMetadataID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// verify metadata appears in container logs
		// The netcore sample prints: "Config with key {key} has value {value}" for all GSDK config
		Eventually(func(g Gomega) {
			var gameServers mpsv1alpha1.GameServerList
			err := kubeClient.List(ctx, &gameServers, client.MatchingLabels{LabelBuildID: testBuildMetadataID})
			g.Expect(err).ToNot(HaveOccurred())

			for _, gs := range gameServers.Items {
				if gs.Status.State == mpsv1alpha1.GameServerStateActive {
					containerLogs, err := getContainerLogs(ctx, coreClient, gs.Name, containerName, gs.Namespace)
					g.Expect(err).ToNot(HaveOccurred())

					// the GSDK config settings include build metadata
					g.Expect(strings.Contains(containerLogs, "Config with key metadatakey1 has value metadatavalue1")).To(BeTrue(),
						"expected metadata key1 in logs, got: %s", containerLogs)
					g.Expect(strings.Contains(containerLogs, "Config with key metadatakey2 has value metadatavalue2")).To(BeTrue(),
						"expected metadata key2 in logs, got: %s", containerLogs)
					g.Expect(strings.Contains(containerLogs, "Config with key metadatakey3 has value metadatavalue3")).To(BeTrue(),
						"expected metadata key3 in logs, got: %s", containerLogs)
				}
			}
		}, timeout, interval).Should(Succeed())
	})
})
