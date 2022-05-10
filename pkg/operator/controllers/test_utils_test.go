package controllers

import (
	"context"
	"os"

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

func verifyGameServersForBuildAreInitializing(ctx context.Context, buildID string, count int) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		var initializingCount int
		for i := 0; i < len(gameServers.Items); i++ {
			gs := gameServers.Items[i]
			if gs.Status.State == "" || gs.Status.State == mpsv1alpha1.GameServerStateInitializing {
				initializingCount++
			}
		}
		return initializingCount == count
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

// verifyTotalGameServerCount verifies the total number of game servers
func verifyTotalGameServerCount(ctx context.Context, buildID string, total int) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		return len(gameServers.Items) == total
	}, timeout, interval).Should(BeTrue())
}

// verifyStandindByActiveCount verifies that the number of standingBy and active game servers is equal to the expected
func verifyStandingByActiveByCount(ctx context.Context, buildID string, standingByCount, activeCount int) {
	Eventually(func() bool {
		var gameServers mpsv1alpha1.GameServerList
		err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
		Expect(err).ToNot(HaveOccurred())
		var currentActive, currentStandingBy int
		for i := 0; i < len(gameServers.Items); i++ {
			gs := gameServers.Items[i]
			if gs.Status.State == mpsv1alpha1.GameServerStateActive {
				currentActive++
			} else if gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
				currentStandingBy++
			}
		}
		return currentActive == activeCount && currentStandingBy == standingByCount
	}, timeout, interval).Should(BeTrue())
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

// updateInitializingGameServersToStandingBy sets all initializing game servers to standing by
func updateInitializingGameServersToStandingBy(ctx context.Context, buildID string) {
	var gameServers mpsv1alpha1.GameServerList
	err := testk8sClient.List(ctx, &gameServers, client.InNamespace(testnamespace), client.MatchingLabels{LabelBuildID: buildID})
	Expect(err).ToNot(HaveOccurred())
	for _, gameServer := range gameServers.Items {
		if gameServer.Status.State == "" || gameServer.Status.State == mpsv1alpha1.GameServerStateInitializing {
			gs := getGameServer(ctx, gameServer.Name) // getting the latest updated GameServer object
			patch := client.MergeFrom(gs.DeepCopy())
			gs.Status.State = mpsv1alpha1.GameServerStateStandingBy
			gs.Status.Health = mpsv1alpha1.Healthy
			err = testk8sClient.Status().Patch(ctx, &gs, patch)
			Expect(err).ToNot(HaveOccurred())
		}
	}
}

// testGenerateGameServerBuild creates a GameServerBuild with the given name and ID.
func testGenerateGameServerBuild(buildName, buildNamespace, buildID string, standingBy, max int, hostNetwork bool) mpsv1alpha1.GameServerBuild {
	return mpsv1alpha1.GameServerBuild{
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:    buildID,
			StandingBy: standingBy,
			Max:        max,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testcontainer",
							Image: os.Getenv("THUNDERNETES_SAMPLE_IMAGE"),
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

func generateGameServer(buildName, buildID, gsNamespace, gsName string) *mpsv1alpha1.GameServer {
	return &mpsv1alpha1.GameServer{
		Spec: mpsv1alpha1.GameServerSpec{
			BuildID: buildID,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"label1": "value1", "label2": "value2"},
					Annotations: map[string]string{"annotation1": "value1", "annotation2": "value2"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testcontainer",
							Image: os.Getenv("THUNDERNETES_SAMPLE_IMAGE"),
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

func newTestSimpleK8s() client.Client {
	cb := fake.NewClientBuilder()
	return cb.Build()
}

func testCreateGameServerAndBuild(client client.Client, gameServerName, buildName, buildID, sessionID string, state mpsv1alpha1.GameServerState) error {
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
		return err
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
		return err
	}
	return nil
}

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
