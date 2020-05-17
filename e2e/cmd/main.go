package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/google/uuid"
	mpsv1alpha1 "github.com/playfab/thundernetes/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	kubeClient client.Client
	coreClient *kubernetes.Clientset // we need the client-go kubernetes client so we can do i) remote exec to the Pods and ii) capture pod logs
	kubeConfig *rest.Config
)

const (
	loopTimes                 int    = 20
	delayInSecondsForLoopTest int    = 5
	delayInSeconds            int    = 1
	LabelBuildID                     = "BuildID"
	invalidStatusCode         string = "invalid status code"
)

type AllocationResult struct {
	IPV4Address string `json:"IPv4Address"`
	SessionID   string `json:"SessionID"`
}

type buildState struct {
	activeCount     int
	standingByCount int
	podCount        int
	buildID         string
	buildName       string
	crashesCount    int
}

const (
	testNamespace  = "mynamespace"
	testBuild1Name = "testbuild1"
	testBuild2Name = "testbuild2"
	testBuild3Name = "crashing"
	test1BuildID   = "85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"
	test2BuildID   = "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6"
	test3BuildID   = "85ffe8da-c82f-4035-86c5-9d2b5f42d6f7"
)

func main() {
	if len(os.Args) < 1 {
		panic("Image name required")
	}

	imgName := os.Args[1]

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = mpsv1alpha1.AddToScheme(scheme)

	ctx := context.Background()

	var err error
	kubeConfig = ctrl.GetConfigOrDie()
	kubeClient, err = client.New(kubeConfig, client.Options{Scheme: scheme})
	if err != nil {
		handleError(err)
	}

	coreClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		handleError(err)
	}

	// get certificates to authenticate to operator API server
	certFile := os.Getenv("TLS_PUBLIC")
	keyFile := os.Getenv("TLS_PRIVATE")
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		handleError(err)
	}

	build1 := &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBuild1Name,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       test1BuildID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: "netcore-sample", PortName: "myport"}},
			BuildMetadata: []mpsv1alpha1.BuildMetadataItem{
				{Key: "metadatakey1", Value: "metadatavalue1"},
				{Key: "metadatakey2", Value: "metadatavalue2"},
				{Key: "metadatakey3", Value: "metadatavalue3"},
			},
			StandingBy: 2,
			Max:        4,
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Image:           imgName,
						Name:            "netcore-sample",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name:          "myport",
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	}

	build2 := &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBuild2Name,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       test2BuildID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: "netcore-sample", PortName: "myport"}},
			StandingBy:    2,
			Max:           4,
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Image:           imgName,
						Name:            "netcore-sample",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name:          "myport",
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	}

	build3 := &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBuild3Name,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       test3BuildID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: "netcore-sample", PortName: "myport"}},
			StandingBy:    2,
			Max:           4,
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Image:           imgName,
						Name:            "netcore-sample",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/sh", "-c", "sleep 2 && command_that_does_not_exist"},
						Ports: []corev1.ContainerPort{
							{
								Name:          "myport",
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	}

	if testNamespace != "default" {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		fmt.Printf("Creating namespace %s\n", testNamespace)
		if err := kubeClient.Create(ctx, ns); err != nil {
			handleError(err)
		}

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "thundernetes-gameserver-editor",
				Namespace: testNamespace,
			},
		}
		fmt.Println("Creating service account gameserver-editor")
		if err := kubeClient.Create(ctx, sa); err != nil {
			handleError(err)
		}

		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "thundernetes-gameserver-editor-rolebinding",
				Namespace: testNamespace,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "thundernetes-gameserver-editor-role",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "thundernetes-gameserver-editor",
					Namespace: testNamespace,
				},
			},
		}
		fmt.Println("Creating role binding account gameserver-editor-rolebinding")
		if err := kubeClient.Create(ctx, rb); err != nil {
			handleError(err)
		}
	}

	fmt.Println("Creating 2 GameServerBuilds")
	if err := kubeClient.Create(ctx, build1); err != nil {
		handleError(err)
	}
	if err := kubeClient.Create(ctx, build2); err != nil {
		handleError(err)
	}

	fmt.Println("Checking that both Builds have 2 standingBy servers and 2 Pods")
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 0, podCount: 2})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 2, activeCount: 0, podCount: 2})

	// -------------- Scaling tests start --------------

	fmt.Println("Updating build1 with 4 standingBy")
	var gsb mpsv1alpha1.GameServerBuild
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.StandingBy = 4
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 4, activeCount: 0, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 2, activeCount: 0, podCount: 2})

	fmt.Println("Updating build2 with 3 standingBy")
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild2Name, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.StandingBy = 3
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 4, activeCount: 0, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 0, podCount: 3})

	fmt.Println("Updating build1 with 3 standingBy - 1 standingBy should be removed")
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		panic(err)
	}
	gsb.Spec.StandingBy = 3
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 3, activeCount: 0, podCount: 3})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 0, podCount: 3})

	// -------------- Scaling tests end --------------

	// -------------- Allocation tests start --------------

	fmt.Println("Allocating on Build1")
	sessionID1 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 3, activeCount: 1, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 0, podCount: 3})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test1BuildID)

	fmt.Println("Allocating on Build1 with same sessionID - should not convert another standingBy to active")
	if err := allocate(test1BuildID, sessionID1, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 3, activeCount: 1, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 0, podCount: 3})

	fmt.Println("Allocating on Build1 with a new sessionID")
	sessionID1_1 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1_1, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 2, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 0, podCount: 3})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test1BuildID)

	fmt.Println("Allocating on Build2")
	sessionID2 := uuid.New().String()
	if err := allocate(test2BuildID, sessionID2, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 2, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test2BuildID)

	fmt.Println("Allocating on Build1 with a new sessionID")
	sessionID1_2 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1_2, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 3, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test1BuildID)

	fmt.Println("Allocating on Build1 with a new sessionID")
	sessionID1_3 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1_3, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 0, activeCount: 4, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test1BuildID)

	fmt.Println("Allocating on Build1 with a new sessionID, expecting 429")
	sessionID1_4 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1_4, cert); err.Error() != fmt.Sprintf("%s 429", invalidStatusCode) {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 0, activeCount: 4, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})

	// -------------- Allocation tests end --------------

	// -------------- More scaling tests start --------------
	fmt.Println("Updating build1 with 3 max - since we have 4 actives, noone should be removed")
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		panic(err)
	}
	gsb.Spec.Max = 3
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 0, activeCount: 4, podCount: 4})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})

	fmt.Println("Updating build1 with 5 max - we should have 1 standingBy")
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		panic(err)
	}
	gsb.Spec.Max = 5
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podCount: 5})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})
	// -------------- More scaling tests end --------------

	// -------------- HTTP Server validation start --------------

	fmt.Println("Allocating on Build1 with a non-Guid sessionID, expecting 400")
	sessionID1_5 := "notAGuid"
	if err := allocate(test1BuildID, sessionID1_5, cert); err.Error() != fmt.Sprintf("%s 400", invalidStatusCode) {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podCount: 5})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})

	fmt.Println("Allocating with a non-Guid BuildID, expecting 400")
	sessionID1_6 := uuid.New().String()
	if err := allocate("notAGuid", sessionID1_6, cert); err.Error() != fmt.Sprintf("%s 400", invalidStatusCode) {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podCount: 5})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})

	fmt.Println("Allocating with a non existent BuildID, expecting 404")
	sessionID1_7 := uuid.New().String()
	if err := allocate(uuid.New().String(), sessionID1_7, cert); err.Error() != fmt.Sprintf("%s 404", invalidStatusCode) {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podCount: 5})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})

	// // -------------- HTTP Server validation end --------------

	// -------------- GameServer process exiting gracefully start --------------

	fmt.Println("Killing an active gameserver from Build1")
	if err := stopActiveGameServer(ctx, test1BuildID, kubeConfig); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 3, podCount: 5})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 1, podCount: 4})

	fmt.Println("Killing an active gameserver from Build2")
	if err := stopActiveGameServer(ctx, test2BuildID, kubeConfig); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 3, podCount: 5})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 0, podCount: 3})

	fmt.Println("Killing another active gameserver from Build1")
	if err := stopActiveGameServer(ctx, test1BuildID, kubeConfig); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 3, activeCount: 2, podCount: 5})
	validateBuildState(ctx, buildState{buildID: test2BuildID, buildName: testBuild2Name, standingByCount: 3, activeCount: 0, podCount: 3})

	// -------------- GameServer process exiting gracefully end --------------

	// -------------- GameServer process exiting with a non-zero code start --------------

	fmt.Println("Creating a Build with failing gameServers")
	if err := kubeClient.Create(ctx, build3); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test3BuildID, buildName: testBuild3Name, standingByCount: 0, activeCount: 0, crashesCount: 5})

	// -------------- GameServer process exiting with a non-zero code end --------------
}

func stopActiveGameServer(ctx context.Context, buildID string, cfg *rest.Config) error {
	var gameServers mpsv1alpha1.GameServerList
	if err := kubeClient.List(ctx, &gameServers, client.MatchingLabels{LabelBuildID: buildID}); err != nil {
		return err
	}

	activeGameServers := make([]mpsv1alpha1.GameServer, 0)
	for _, gameServer := range gameServers.Items {
		if gameServer.Status.State == mpsv1alpha1.GameServerStateActive {
			activeGameServers = append(activeGameServers, gameServer)
		}
	}

	randomGameServer := activeGameServers[rand.Intn(len(activeGameServers))]

	var pods corev1.PodList
	if err := kubeClient.List(ctx, &pods, client.MatchingLabels{LabelBuildID: buildID}); err != nil {
		return err
	}

	gameServerPods := make([]corev1.Pod, 0)
	for _, pod := range pods.Items {
		if pod.OwnerReferences != nil {
			if pod.OwnerReferences[0].Name == randomGameServer.Name {
				gameServerPods = append(gameServerPods, pod)
			}
		}
	}

	if len(gameServerPods) != 1 {
		return fmt.Errorf("expected 1 pod, got %d", len(gameServerPods))
	}

	pod := gameServerPods[0]

	_, _, err := executeRemoteCommand(coreClient, &pod, kubeConfig, "kill 1")

	if err != nil {
		return err
	}

	return nil
}

func validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx context.Context, buildID string) error {
	var gameServers mpsv1alpha1.GameServerList
	if err := kubeClient.List(ctx, &gameServers, client.MatchingLabels{LabelBuildID: buildID}); err != nil {
		return err
	}

	activeGameServers := make([]mpsv1alpha1.GameServer, 0)
	for _, gameServer := range gameServers.Items {
		if gameServer.Status.State == mpsv1alpha1.GameServerStateActive {
			activeGameServers = append(activeGameServers, gameServer)
		}
	}

	for _, gameServer := range activeGameServers {
		logs, err := getPodLogs(ctx, coreClient, gameServer.Name, gameServer.Namespace)
		if err != nil {
			return err
		}
		if !strings.Contains(logs, "After ReadyForPlayers") { // this string must be the same as the one logged on netcore-sample
			return fmt.Errorf("ReadyForPlayers still blocked for %s, the GSDK was not notified of the GameServer transitioning to Active", gameServer.Name)
		}
	}
	return nil
}
