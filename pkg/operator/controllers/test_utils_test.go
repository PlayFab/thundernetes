package controllers

import (
	"context"

	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// getNewBuildNameAndID returns a new build name and ID
func getNewBuildNameAndID() (string, string) {
	buildName := randString(5)
	buildID := string(uuid.NewUUID())
	return buildName, buildID
}

func verifyContainerPortIsTheSameAsHostPort(ctx context.Context, buildID string) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		for _, gameServer := range gameServers.Items {
			// get the Pod with the same name as this GameServer
			var pod corev1.Pod
			err := testk8sClient.Get(ctx, types.NamespacedName{Name: gameServer.Name, Namespace: testnamespace}, &pod)
			Expect(err).ToNot(HaveOccurred())
			// get the container port
			containerPort := pod.Spec.Containers[0].Ports[0].ContainerPort
			// get the host port
			hostPort := pod.Spec.Containers[0].Ports[0].HostPort
			if containerPort != hostPort {
				return false
			}
		}
		return true
	}, timeout, interval).Should(BeTrue())
}

// verifyThatBuildIsUnhealthy verifies that the Build is unhealthy
func verifyThatBuildIsUnhealthy(ctx context.Context, buildName string) {
	Eventually(func() bool {
		var gameServerBuild mpsv1alpha1.GameServerBuild
		err := testk8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testnamespace}, &gameServerBuild)
		Expect(err).ShouldNot(HaveOccurred())
		return gameServerBuild.Status.Health == mpsv1alpha1.BuildUnhealthy
	}, assertTimeout, assertPollingInterval).Should(BeTrue())
}

func testTerminateActiveGameServer(ctx context.Context, buildID string, gracefulTermination bool) {
	var gameServers mpsv1alpha1.GameServerList
	err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
	Expect(err).ToNot(HaveOccurred())
	for i := 0; i < len(gameServers.Items); i++ {
		gs := gameServers.Items[i]
		if gs.Status.State == mpsv1alpha1.GameServerStateActive {
			if gracefulTermination {
				gs.Status.State = mpsv1alpha1.GameServerStateGameCompleted
			} else {
				gs.Status.State = mpsv1alpha1.GameServerStateCrashed
			}
			err = testk8sClient.Status().Update(ctx, &gs)
			Expect(err).ToNot(HaveOccurred())
			return
		}
	}
	Expect(true).To(BeFalse()) // should never get here
}

// allocateGameServerManually converts the state of a GameServer to Active
func allocateGameServerManually(ctx context.Context, buildID string) {
	var gameServers mpsv1alpha1.GameServerList
	err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
	Expect(err).ToNot(HaveOccurred())
	for i := 0; i < len(gameServers.Items); i++ {
		gs := gameServers.Items[i]
		if gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
			gs.Status.State = mpsv1alpha1.GameServerStateActive
			err = testk8sClient.Status().Update(ctx, &gs)
			Expect(err).ToNot(HaveOccurred())
			return
		}
	}
	Expect(true).To(BeFalse()) // should never get here
}

// testUpdateGameServerBuild updates the GameServerBuild with the requested standingBy and max
func testUpdateGameServerBuild(ctx context.Context, standingBy, max int, buildName string) {
	Eventually(func() error {
		gsb := getGameServerBuild(ctx, buildName)
		gsb.Spec.StandingBy = standingBy
		gsb.Spec.Max = max
		return testk8sClient.Update(ctx, &gsb)
	}, timeout, interval).Should(Succeed())
}

// testWaitAndVerifyTotalGameServerCount verifies the total number of game servers
// useful to wait for the GameServers to be created, so we can update their status
func testWaitAndVerifyTotalGameServerCount(ctx context.Context, buildID string, total int) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		return len(gameServers.Items) == total
	}, timeout, interval).Should(BeTrue())
}

// testStates encapsulates the expected test states for GameServers in a GameServerBuild
type testStates struct {
	emptyStateCount   int
	initializingCount int
	standingByCount   int
	activeCount       int
}

// testVerifyGameServerStates verifies that the number of standingBy and active game servers is equal to the expected
func testVerifyGameServerStates(ctx context.Context, buildID string, t testStates) {
	Eventually(func(g Gomega) {
		var gameServers mpsv1alpha1.GameServerList
		err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		g.Expect(err).ToNot(HaveOccurred())
		var currentEmptyState, currentInitializing, currentActive, currentStandingBy, currentUnknown int
		for i := 0; i < len(gameServers.Items); i++ {
			gs := gameServers.Items[i]
			if gs.Status.State == "" {
				currentEmptyState++
			} else if gs.Status.State == mpsv1alpha1.GameServerStateInitializing {
				currentInitializing++
			} else if gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
				currentStandingBy++
			} else if gs.Status.State == mpsv1alpha1.GameServerStateActive {
				currentActive++
			} else {
				currentUnknown++
			}
		}
		g.Expect(currentEmptyState).To(Equal(t.emptyStateCount))
		g.Expect(currentInitializing).To(Equal(t.initializingCount))
		g.Expect(currentActive).To(Equal(t.activeCount))
		g.Expect(currentStandingBy).To(Equal(t.standingByCount))
		g.Expect(currentUnknown).To(Equal(0))
	}, timeout, interval).Should(Succeed())
}

// getGameServerBuild returns the GameServerBuild with the given name
func getGameServerBuild(ctx context.Context, buildName string) mpsv1alpha1.GameServerBuild {
	var gameServerBuild mpsv1alpha1.GameServerBuild
	err := testk8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testnamespace}, &gameServerBuild)
	Expect(err).ToNot(HaveOccurred())
	return gameServerBuild
}

// getGameServer returns the GameServer with the given name
func getGameServer(ctx context.Context, gameServerName string) mpsv1alpha1.GameServer {
	var gameServer mpsv1alpha1.GameServer
	err := testk8sClient.Get(ctx, types.NamespacedName{Name: gameServerName, Namespace: testnamespace}, &gameServer)
	Expect(err).ToNot(HaveOccurred())
	return gameServer
}

// testUpdateGameServersState updates the state of the GameServers in the given GameServerBuild
func testUpdateGameServersState(ctx context.Context, buildID string, oldState, newState mpsv1alpha1.GameServerState) {
	var gameServers mpsv1alpha1.GameServerList
	err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
	Expect(err).ToNot(HaveOccurred())
	for _, gameServer := range gameServers.Items {
		if gameServer.Status.State == oldState {
			gs := getGameServer(ctx, gameServer.Name) // getting the latest updated GameServer object
			patch := client.MergeFrom(gs.DeepCopy())
			gs.Status.State = newState
			gs.Status.Health = mpsv1alpha1.GameServerHealthy
			err = testk8sClient.Status().Patch(ctx, &gs, patch)
			Expect(err).ToNot(HaveOccurred())
		}
	}
}

// testGenerateGameServerBuild creates a GameServerBuild with the given name and ID.
func testGenerateGameServerBuild(buildName, buildNamespace, buildID string, standingBy, max int, hostNetwork bool) mpsv1alpha1.GameServerBuild {
	return mpsv1alpha1.GameServerBuild{
		Spec: mpsv1alpha1.GameServerBuildSpec{
			PortsToExpose: []int32{80},
			TitleID:       "test-title-id",
			BuildID:       buildID,
			StandingBy:    standingBy,
			Max:           max,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testcontainer",
							Image: "testimage",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
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
			Namespace: buildNamespace,
		},
	}
}

// testGenerateGameServer returns a new GameServer with the given name and ID.
func testGenerateGameServer(buildName, buildID, gsNamespace, gsName string) *mpsv1alpha1.GameServer {
	return &mpsv1alpha1.GameServer{
		Spec: mpsv1alpha1.GameServerSpec{
			TitleID:       "testtitleid",
			PortsToExpose: []int32{80},
			BuildID:       buildID,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"label1": "value1", "label2": "value2"},
					Annotations: map[string]string{"annotation1": "value1", "annotation2": "value2"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testcontainer",
							Image: "testimage",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      gsName,
			Namespace: gsNamespace,
			Labels:    map[string]string{LabelBuildName: buildName},
		},
	}
}

// testNewSimpleK8sClient returns a new fake k8s client
func testNewSimpleK8sClient() client.Client {
	cb := fake.NewClientBuilder()
	return cb.WithIndex(&mpsv1alpha1.GameServer{}, statusSessionId, func(rawObj client.Object) []string {
		gs := rawObj.(*mpsv1alpha1.GameServer)
		return []string{gs.Status.SessionID}
	}).WithIndex(&mpsv1alpha1.GameServerBuild{}, specBuildId, func(rawObj client.Object) []string {
		gsb := rawObj.(*mpsv1alpha1.GameServerBuild)
		return []string{gsb.Spec.BuildID}
	}).Build()
}

// testCreateGameServerAndBuild creates a GameServer and GameServerBuild with the given name and ID.
func testCreateGameServerAndBuild(client client.Client, gameServerName, buildName, buildID, sessionID string, state mpsv1alpha1.GameServerState) (*mpsv1alpha1.GameServer, error) {
	buildNamespace := "default"
	gsb := mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: buildNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID: buildID,
		},
	}
	err := client.Create(context.Background(), &gsb)
	if err != nil {
		return nil, err
	}
	gs := mpsv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gameServerName,
			Namespace: buildNamespace,
			Labels: map[string]string{
				LabelBuildID:   buildID,
				LabelBuildName: buildName,
			},
		},
		Status: mpsv1alpha1.GameServerStatus{
			SessionID: sessionID,
			State:     state,
		},
	}
	err = client.Create(context.Background(), &gs)
	if err != nil {
		return nil, err
	}
	return &gs, err
}

// testCreatePod creates a Pod with the given name
func testCreatePod(client client.Client, gsName string) error {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: gsName,
			Labels: map[string]string{
				LabelOwningGameServer: gsName,
			},
		},
	}
	err := client.Create(context.Background(), &pod)
	if err != nil {
		return err
	}
	return nil
}

// testVerifyEnv verifies that environment variable exists in the array and has the expected value
func testVerifyEnv(envs []corev1.EnvVar, env corev1.EnvVar) bool {
	for _, e := range envs {
		if e.Name == env.Name {
			return e.Value == env.Value
		}
	}
	return false
}
