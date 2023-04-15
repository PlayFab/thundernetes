package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("expectations tests", func() {
	// we are performing the tests by including an extra external expectations struct
	// since the actual one is encapsulated in the GameServerBuild reconciler
	It("should add and remove game servers to the expectation maps", func() {
		buildName := "test-build-expectations"
		gsName1 := "test-gs-expectations-1"
		gsName2 := "test-gs-expectations-2"
		buildID := "d82e2163-5912-46cb-9bea-0b783221bb8b"
		buildNamespace := "default"
		// create a client to interact with the cluster
		client := testNewSimpleK8sClient()
		// create a new expectations struct
		e := NewGameServerExpectations(client)
		// create a new GameServerBuild with a single game server
		_, err := testCreateGameServerAndBuild(client, gsName1, buildName, buildID, "", mpsv1alpha1.GameServerStateInitializing)
		Expect(err).ToNot(HaveOccurred())
		// we're calling the add method manually since the actual expectations class is encapsulated in the controller
		e.addGameServerToUnderCreationMap(buildName, gsName1)
		// make sure GameServerBuild and GameServer are in the underCreation map
		gsUnderCreation, ok := e.gameServersUnderCreation.Load(buildName)
		Expect(ok).To(BeTrue())
		mmCreations := gsUnderCreation.(*MutexMap)
		_, ok = mmCreations.data[gsName1]
		Expect(ok).To(BeTrue())

		// create a new GameServer and make sure it's added in the map
		err = client.Create(context.Background(), testGenerateGameServer(buildName, buildID, buildNamespace, gsName2))
		Expect(err).ToNot(HaveOccurred())
		e.addGameServerToUnderCreationMap(buildName, gsName2)
		_, ok = mmCreations.data[gsName2]
		Expect(ok).To(BeTrue())

		// make sure create expectations have been satisfied
		gsb := mpsv1alpha1.GameServerBuild{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name:      buildName,
			Namespace: buildNamespace,
		}, &gsb)
		Expect(err).To(Not(HaveOccurred()))
		created, err := e.gameServersUnderCreationWereCreated(context.Background(), &gsb)
		Expect(err).To(Not(HaveOccurred()))
		Expect(created).To(BeTrue())

		// make sure the creations map doesn't include the GameServerBuild any more
		_, ok = e.gameServersUnderCreation.Load(buildName)
		Expect(ok).To(BeFalse())

		// get and delete the second GameServer
		gs := mpsv1alpha1.GameServer{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name:      gsName2,
			Namespace: buildNamespace,
		}, &gs)
		Expect(err).To(Not(HaveOccurred()))

		// make sure it's added to the under deletion map
		e.addGameServerToUnderDeletionMap(buildName, gsName2)
		gsUnderDeletion, ok := e.gameServersUnderDeletion.Load(buildName)
		Expect(ok).To(BeTrue())
		mmDeletion := gsUnderDeletion.(*MutexMap)
		_, ok = mmDeletion.data[gsName2]
		Expect(ok).To(BeTrue())

		// delete it and make sure the deletion expectations are satisfied
		err = client.Delete(context.Background(), &gs)
		Expect(err).To(Not(HaveOccurred()))
		deleted, err := e.gameServersUnderDeletionWereDeleted(context.Background(), &gsb)
		Expect(err).To(Not(HaveOccurred()))
		Expect(deleted).To(BeTrue())

		// make sure the deletions map doesn't include the GameServerBuild any more
		_, ok = e.gameServersUnderDeletion.Load(buildName)
		Expect(ok).To(BeFalse())
	})
})
