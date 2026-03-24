package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
)

var _ = Describe("Controller reconciliation stress tests", func() {
	ctx := context.Background()

	// This test creates multiple GameServerBuilds simultaneously and verifies
	// the controller handles all of them correctly
	It("should handle multiple concurrent GameServerBuilds", func() {
		const buildCount = 5
		const standingBy = 2
		const max = 4

		type buildInfo struct {
			name    string
			buildID string
		}
		builds := make([]buildInfo, buildCount)

		// Create all builds
		for i := 0; i < buildCount; i++ {
			buildName, buildID := getNewBuildNameAndID()
			builds[i] = buildInfo{name: buildName, buildID: buildID}
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, standingBy, max, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
		}

		// Wait for all GameServers to be created for all builds
		for _, bi := range builds {
			testWaitAndVerifyTotalGameServerCount(ctx, bi.buildID, standingBy)
		}

		// Set all GameServers to StandingBy
		for _, bi := range builds {
			testUpdateGameServersState(ctx, bi.buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, bi.buildID, testStates{0, 0, standingBy, 0})
		}
	})

	// Stress test: rapid scale up and down
	It("should handle rapid scale changes", func() {
		buildName, buildID := getNewBuildNameAndID()
		gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 10, false)
		Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())

		testWaitAndVerifyTotalGameServerCount(ctx, buildID, 2)
		testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
		testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 0})

		// Scale up to 5
		testUpdateGameServerBuild(ctx, 5, 10, buildName)
		testWaitAndVerifyTotalGameServerCount(ctx, buildID, 5)
		testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
		testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 5, 0})

		// Scale back down to 2
		testUpdateGameServerBuild(ctx, 2, 10, buildName)
		testWaitAndVerifyTotalGameServerCount(ctx, buildID, 2)
		testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 0})
	})

	// Stress test: allocate and replace cycle
	It("should handle rapid allocation and replacement cycles", func() {
		buildName, buildID := getNewBuildNameAndID()
		gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 3, 6, false)
		Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())

		testWaitAndVerifyTotalGameServerCount(ctx, buildID, 3)
		testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
		testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 3, 0})

		// Rapidly allocate all 3 servers and verify replacements are created
		for cycle := 0; cycle < 2; cycle++ {
			// Allocate one
			allocateGameServerManually(ctx, buildID)

			// Wait for replacement
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)

			// Complete the active session
			testTerminateActiveGameServer(ctx, buildID, true)

			// Verify the build stabilizes
			Eventually(func() bool {
				return getGameServerBuild(ctx, buildName).Status.CurrentActive == 0
			}, timeout, interval).Should(BeTrue())

			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 3)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 3, 0})
		}
	})

	// Stress test: multiple builds with allocation churn
	It("should handle allocation churn across multiple builds", func() {
		const buildCount = 3
		type buildInfo struct {
			name    string
			buildID string
		}
		builds := make([]buildInfo, buildCount)

		for i := 0; i < buildCount; i++ {
			buildName, buildID := getNewBuildNameAndID()
			builds[i] = buildInfo{name: buildName, buildID: buildID}
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
		}

		// Wait for initial GameServers
		for _, bi := range builds {
			testWaitAndVerifyTotalGameServerCount(ctx, bi.buildID, 2)
			testUpdateGameServersState(ctx, bi.buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, bi.buildID, testStates{0, 0, 2, 0})
		}

		// Allocate one server from each build
		for _, bi := range builds {
			allocateGameServerManually(ctx, bi.buildID)
		}

		// Wait for replacements
		for _, bi := range builds {
			testWaitAndVerifyTotalGameServerCount(ctx, bi.buildID, 3) // 2 standingBy + 1 active
			testUpdateGameServersState(ctx, bi.buildID, "", v1alpha1.GameServerStateStandingBy)
		}

		// Complete all active sessions
		for _, bi := range builds {
			testTerminateActiveGameServer(ctx, bi.buildID, true)
			testWaitAndVerifyTotalGameServerCount(ctx, bi.buildID, 2)
			testUpdateGameServersState(ctx, bi.buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, bi.buildID, testStates{0, 0, 2, 0})
		}
	})

	// Stress test: crashes during scaling
	It("should handle crashes during scaling operations", func() {
		buildName, buildID := getNewBuildNameAndID()
		gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 3, 6, false)
		Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())

		testWaitAndVerifyTotalGameServerCount(ctx, buildID, 3)
		testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
		testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 3, 0})

		// Crash one GameServer
		testUpdateGameServersState(ctx, buildID, v1alpha1.GameServerStateStandingBy, v1alpha1.GameServerStateCrashed)

		// Wait for recovery - crashed servers should be replaced
		Eventually(func() int {
			gsb := getGameServerBuild(ctx, buildName)
			return gsb.Status.CrashesCount
		}, timeout, interval).Should(BeNumerically(">", 0))

		// The controller should either create replacements or mark the build as unhealthy
		Eventually(func(g Gomega) {
			gsb := getGameServerBuild(ctx, buildName)
			// Either the build is unhealthy (too many crashes) or replacements are being created
			isUnhealthy := gsb.Status.Health == v1alpha1.BuildUnhealthy
			hasReplacements := gsb.Status.CurrentStandingBy+gsb.Status.CurrentActive+
				gsb.Status.CurrentInitializing+gsb.Status.CurrentPending > 0
			g.Expect(isUnhealthy || hasReplacements).To(BeTrue(),
				"expected build to be unhealthy or have replacement servers, got health=%s servers=%d",
				gsb.Status.Health,
				gsb.Status.CurrentStandingBy+gsb.Status.CurrentActive+gsb.Status.CurrentInitializing+gsb.Status.CurrentPending)
		}, timeout, interval).Should(Succeed())

		fmt.Fprintf(GinkgoWriter, "Build %s health: %s, crashes: %d\n",
			buildName,
			getGameServerBuild(ctx, buildName).Status.Health,
			getGameServerBuild(ctx, buildName).Status.CrashesCount)
	})
})
