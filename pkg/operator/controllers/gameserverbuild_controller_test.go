package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
)

var _ = Describe("GameServerBuild controller tests", func() {
	Context("testing creating a gameserverbuild creates game servers", func() {
		ctx := context.Background()

		// tests here follow the flow
		// 1. create a GameServerBuild
		// 2. verify that we have the requested count of GameServer CRs. This also "waits" for these GameServers, so we can update them
		// 3. since we don't have containers running (so the NodeAgent DaemonSet can update the GameServer.Status), we manually set the initializing GameServers' .Status to standingBy
		// 4. verify the number of empty state, initializing, standingBy and active GameServers
		// Note: On the second item in the above list, it's important to verify that the requested number of GameServers has been created, since if we
		// try to fetch them to update their .Status, we might end up in a state that they have not been created yet on the API server

		// tests the creation of game servers
		It("should create game servers", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 0})
		})

		// simple scaling test
		It("should scale game servers", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 0})

			testUpdateGameServerBuild(ctx, 4, 4, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 4, 0})
		})
		// controller should block creating ">max" GameServers
		It("should not scale game servers beyond max", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 0})

			testUpdateGameServerBuild(ctx, 5, 4, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 4, 0})
		})
		// however, when we increase the max, controller should create the additional GameServers
		It("should scale game servers when max is increasing", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 0})

			testUpdateGameServerBuild(ctx, 5, 4, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 4, 0})

			testUpdateGameServerBuild(ctx, 5, 5, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 5)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 5, 0})
		})

		// manual allocation by manually setting .GameServer.Status.State to "Active"
		It("should increase standingBy when allocating", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 0})

			// allocate two GameServers and watch that new StandingBys are being created
			allocateGameServerManually(ctx, buildID)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 3)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 1})

			allocateGameServerManually(ctx, buildID)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 2})

			// another scaling test, increase the standingBy to 4 and max to 6
			// since we have 2 active, we should have 4 standingBy
			testUpdateGameServerBuild(ctx, 4, 6, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 6)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 4, 2})
		})

		It("should create new game servers if game sessions end", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 4, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 4, 0})

			// allocate two game servers, end up with 2 standingBy and 2 active
			allocateGameServerManually(ctx, buildID)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 3, 1})

			allocateGameServerManually(ctx, buildID)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 2, 2})

			// gracefully end the game session for one game server
			// we wait till we have a new GameServer, since it'll take one reconcile loop for the controller to notice the GameServer has exited
			// once we get that, we verify that the controller has created a new GameServer
			testTerminateActiveGameServer(ctx, buildID, true)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testVerifyGameServerStates(ctx, buildID, testStates{1, 0, 2, 1})
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 3, 1})

			// as before, we gracefully terminate the second active GameServer
			// we now should have 4 standingBy
			testTerminateActiveGameServer(ctx, buildID, true)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testVerifyGameServerStates(ctx, buildID, testStates{1, 0, 3, 0})
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 4, 0})
		})

		It("should mark Build as unhealthy when there are too many crashes", func() {
			// create a Build with 6 standingBy
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 6, 6, false)
			crashes := 5
			gsb.Spec.CrashesToMarkUnhealthy = &crashes
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 6)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 6, 0})

			// allocate 5 times
			for i := 0; i < 5; i++ {
				allocateGameServerManually(ctx, buildID)
			}
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 1, 5})

			// simulate 5 crashes (5 is the default value for GameServerBuild to be marked as Unhealthy)
			for i := 0; i < 5; i++ {
				testTerminateActiveGameServer(ctx, buildID, false)
			}
			verifyThatBuildIsUnhealthy(ctx, buildName)
		})

		It("should delete initializing servers before deleting standingBy, during downscaling", func() {
			// create a new GameServerBuild with 4 standingBy and 16 max
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 4, 16, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateStandingBy)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 4, 0})

			// upscale the GameServerBuild to 8 requested standingBy
			testUpdateGameServerBuild(ctx, 8, 16, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 8)
			testUpdateGameServersState(ctx, buildID, "", v1alpha1.GameServerStateInitializing)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 4, 4, 0})

			// upscale the GameServerBuild to 10 requested standingBy
			testUpdateGameServerBuild(ctx, 10, 16, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 10)
			testVerifyGameServerStates(ctx, buildID, testStates{2, 4, 4, 0})

			// downscale the GameServerBuild to 9 requested standingBy
			// we want one GameServer to be deleted, this has to be one with empty state
			testUpdateGameServerBuild(ctx, 9, 16, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 9)
			testVerifyGameServerStates(ctx, buildID, testStates{1, 4, 4, 0})

			// downscale the GameServerBuild to 7 requested standingBy
			// we want two GameServers to be deleted, so we'll take down the remaining one with empty state
			// and one StandingBy
			testUpdateGameServerBuild(ctx, 7, 16, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 7)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 3, 4, 0})

			// downscale the GameServerBuild to 3 requested standingBy
			// all 3 initializing and 1 standingBy should have been deleted
			testUpdateGameServerBuild(ctx, 3, 16, buildName)
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 3)
			testVerifyGameServerStates(ctx, buildID, testStates{0, 0, 3, 0})
		})

		It("should overwrite containerPort with hostPort value when hostNetwork is required", func() {
			// create a Build with 2 standingBy
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, true)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testWaitAndVerifyTotalGameServerCount(ctx, buildID, 2)
			testVerifyGameServerStates(ctx, buildID, testStates{2, 0, 0, 0})
			verifyContainerPortIsTheSameAsHostPort(ctx, buildName)
		})

		It("should fail to create a GameServerBuild without a titleID", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, true)
			gsb.Spec.TitleID = ""
			err := testk8sClient.Create(ctx, &gsb)
			Expect(err).ToNot(BeNil())
		})

		It("should fail to create a GameServerBuild without a buildID", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, true)
			gsb.Spec.BuildID = ""
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Not(Succeed()))
		})

		It("should fail to create a GameServerBuild with a non-GUID buildID", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, true)
			gsb.Spec.BuildID = "1234"
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Not(Succeed()))
		})
	})
})
