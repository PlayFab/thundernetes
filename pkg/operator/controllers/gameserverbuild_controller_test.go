package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	assertPollingInterval = 20 * time.Millisecond
	assertTimeout         = 2 * time.Second
)

var _ = Describe("GameServerBuild controller tests", func() {
	Context("testing creating a gameserverbuild creates game servers", func() {
		ctx := context.Background()

		// tests here follow the flow
		// 1. create a gameserverbuild
		// 2. verify that we have the requested count of GameServer CRs
		// 3. since we don't have containers running (so the daemonset can update the GameServer.Status), we manually
		//    set the initializing GameServers' .Status to standingBy
		// 4. verify the number of standingBy and active GameServers
		// It's important to verify that the requested number of GameServers has been created, since if we
		// try to fetch them to update their .Status, we might end up in a state that they have not been created yet on the API server

		// tests the creation of game servers
		It("should create game servers", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 2, 0)
		})

		// simple scaling test
		It("should scale game servers", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 2, 0)

			testUpdateGameServerBuild(ctx, 4, 4, buildName)
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 4, 0)
		})
		// controller should block creating ">max" GameServers
		It("should not scale game servers beyond max", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 2, 0)

			testUpdateGameServerBuild(ctx, 5, 4, buildName)
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 4, 0)
		})
		// however, when we increase the max, controller should create the additional GameServers
		It("should scale game servers when max is increasing", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 2, 0)

			testUpdateGameServerBuild(ctx, 5, 4, buildName)
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 4, 0)

			testUpdateGameServerBuild(ctx, 5, 5, buildName)
			testVerifyTotalGameServerCount(ctx, buildID, 5)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 5, 0)
		})

		// manual allocation by manually setting .GameServer.Status.State to "Active"
		It("should increase standingBy when allocating", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testVerifyTotalGameServerCount(ctx, buildID, 2)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 2, 0)

			// allocate two GameServers and watch that new StandingBys are being created
			allocateGameServerManually(ctx, buildID)
			testVerifyTotalGameServerCount(ctx, buildID, 3)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 2, 1)

			allocateGameServerManually(ctx, buildID)
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 2, 2)

			// another scaling test, increase the standingBy to 4 and max to 6
			// since we have 2 active, we should have 4 standingBy
			testUpdateGameServerBuild(ctx, 4, 6, buildName)
			testVerifyTotalGameServerCount(ctx, buildID, 6)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 4, 2)
		})

		It("should create new game servers if game sessions end", func() {
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 4, 4, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 4, 0)

			// allocate two game servers, end up with 2 standingBy and 2 active
			allocateGameServerManually(ctx, buildID)
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 3, 1)

			allocateGameServerManually(ctx, buildID)
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 2, 2)

			// gracefully end the game session for one game server
			// we wait till we have a new GameServer, since it'll take one reconcile loop for the controller to notice the GameServer has exited
			// once we get that, we verify that the controller has created a new GameServer
			testTerminateActiveGameServer(ctx, buildID, true)
			testVerifyGameServersForBuildAreInitializing(ctx, buildID, 1)
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 3, 1)

			// as before, we gracefully terminate the second active GameServer
			// we now should have 4 standingBy
			testTerminateActiveGameServer(ctx, buildID, true)
			testVerifyGameServersForBuildAreInitializing(ctx, buildID, 1)
			testVerifyTotalGameServerCount(ctx, buildID, 4)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 4, 0)
		})

		It("should mark Build as unhealthy when there are too many crashes", func() {
			// create a Build with 6 standingBy
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 6, 6, false)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testVerifyTotalGameServerCount(ctx, buildID, 6)
			testUpdateInitializingGameServersToStandingBy(ctx, buildID)
			testVerifyStandingByActiveByCount(ctx, buildID, 6, 0)

			// allocate 5 times
			for i := 0; i < 5; i++ {
				allocateGameServerManually(ctx, buildID)
			}
			testVerifyTotalGameServerCount(ctx, buildID, 6)
			testVerifyStandingByActiveByCount(ctx, buildID, 1, 5)

			// simulate 5 crashes (5 is the default value for GameServerBuild to be marked as Unhealthy)
			for i := 0; i < 5; i++ {
				testTerminateActiveGameServer(ctx, buildID, false)
			}
			verifyThatBuildIsUnhealthy(ctx, buildName)
		})

		It("should overwrite containerPort with hostPort value when hostNetwork is required", func() {
			// create a Build with 6 standingBy
			buildName, buildID := getNewBuildNameAndID()
			gsb := testGenerateGameServerBuild(buildName, testnamespace, buildID, 2, 4, true)
			Expect(testk8sClient.Create(ctx, &gsb)).Should(Succeed())
			testVerifyTotalGameServerCount(ctx, buildID, 2)
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
