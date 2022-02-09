package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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
	loopTimesConst            int    = 15
	delayInSecondsForLoopTest int    = 1
	LabelBuildID                     = "BuildID"
	invalidStatusCode         string = "invalid status code"
	containerName             string = "netcore-sample" // this must be the same as the GameServer name
	nodeAgentName             string = "nodeagent"
	portKey                   string = "gameport"
)

type AllocationResult struct {
	IPV4Address string `json:"IPv4Address"`
	SessionID   string `json:"SessionID"`
}

type buildState struct {
	initializingCount int
	activeCount       int
	standingByCount   int
	podRunningCount   int
	buildID           string
	buildName         string
	crashesCount      int
	loopTimesOverride int // in case we want to override the global const loop times
}

const (
	testNamespace                           = "mynamespace"
	testBuild1Name                          = "testbuild"
	testBuildSleepBeforeReadyForPlayersName = "sleepbeforereadyforplayers"
	testBuildCrashingName                   = "crashing"
	testBuildWithoutReadyForPlayers         = "withoutreadyforplayers"
	test1BuildID                            = "85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"
	testBuildSleepBeforeReadyForPlayersID   = "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6"
	testCrashingBuildID                     = "85ffe8da-c82f-4035-86c5-9d2b5f42d6f7"
	testWithoutReadyForPlayersBuildID       = "85ffe8da-c82f-4035-86c5-9d2b5f42d6f8"
	connectedPlayersCount                   = 3 // this should the same as in the netcore sample
)

var connectedPlayers = []string{"Amie", "Ken", "Dimitris"} // this should the same as in the netcore sample

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

	// get certificates to authenticate to operator allocation API service
	certFile := os.Getenv("TLS_PUBLIC")
	keyFile := os.Getenv("TLS_PRIVATE")
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		handleError(err)
	}

	// build1 is just a normal build
	build1 := &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBuild1Name,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       test1BuildID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: "netcore-sample", PortName: portKey}},
			BuildMetadata: []mpsv1alpha1.BuildMetadataItem{
				{Key: "metadatakey1", Value: "metadatavalue1"},
				{Key: "metadatakey2", Value: "metadatavalue2"},
				{Key: "metadatakey3", Value: "metadatavalue3"},
			},
			StandingBy: 2,
			Max:        4,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           imgName,
							Name:            "netcore-sample",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          portKey,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	// game server process in this build will sleep for a while before it calls ReadyForPlayers
	buildSleepBeforeReadyForPlayers := &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBuildSleepBeforeReadyForPlayersName,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       testBuildSleepBeforeReadyForPlayersID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: "netcore-sample", PortName: portKey}},
			StandingBy:    2,
			Max:           4,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           imgName,
							Name:            "netcore-sample",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          portKey,
									ContainerPort: 80,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "SLEEP_BEFORE_READY_FOR_PLAYERS",
									Value: "true",
								},
							},
						},
					},
				},
			},
		},
	}

	// game servers in this build will crash on start
	buildWithCrashingGameServers := &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBuildCrashingName,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       testCrashingBuildID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: "netcore-sample", PortName: portKey}},
			StandingBy:    2,
			Max:           4,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           imgName,
							Name:            "netcore-sample",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sh", "-c", "sleep 2 && command_that_does_not_exist"},
							Ports: []corev1.ContainerPort{
								{
									Name:          portKey,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	// game server process in this build does not call ReadyForPlayers
	buildWithoutReadyForPlayers := &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBuildWithoutReadyForPlayers,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       testWithoutReadyForPlayersBuildID,
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: "netcore-sample", PortName: portKey}},
			StandingBy:    2,
			Max:           4,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           imgName,
							Name:            "netcore-sample",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          portKey,
									ContainerPort: 80,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "SKIP_READY_FOR_PLAYERS",
									Value: "true",
								},
							},
						},
					},
				},
			},
		},
	}

	// if we are not running in the default namespace, we need to create the namespace
	// plus the service account and role binding
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
	}

	fmt.Println("Creating 2 GameServerBuilds")
	if err := kubeClient.Create(ctx, build1); err != nil {
		handleError(err)
	}

	if err := kubeClient.Create(ctx, buildSleepBeforeReadyForPlayers); err != nil {
		handleError(err)
	}

	fmt.Println("Checking that both Builds have 2 standingBy servers and 2 Pods")
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 0, podRunningCount: 2})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 2, activeCount: 0, podRunningCount: 2})

	// -------------- Scaling tests start --------------

	fmt.Printf("Updating build %s with 4 standingBy\n", testBuild1Name)
	var gsb mpsv1alpha1.GameServerBuild
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.StandingBy = 4
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 4, activeCount: 0, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 2, activeCount: 0, podRunningCount: 2})

	fmt.Printf("Updating build %s with 3 standingBy\n", testBuildSleepBeforeReadyForPlayersName)
	gsb = mpsv1alpha1.GameServerBuild{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuildSleepBeforeReadyForPlayersName, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.StandingBy = 3
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 4, activeCount: 0, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 0, podRunningCount: 3})

	fmt.Printf("Updating build %s with 3 standingBy - 1 standingBy should be removed\n", testBuild1Name)
	gsb = mpsv1alpha1.GameServerBuild{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.StandingBy = 3
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 3, activeCount: 0, podRunningCount: 3})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 0, podRunningCount: 3})

	// -------------- Scaling tests end --------------

	// -------------- Allocation tests start --------------

	fmt.Printf("Allocating on Build %s\n", testBuild1Name)
	sessionID1 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 3, activeCount: 1, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 0, podRunningCount: 3})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test1BuildID)

	fmt.Printf("Allocating on Build %s with same sessionID - should not convert another standingBy to active\n", testBuild1Name)
	if err := allocate(test1BuildID, sessionID1, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 3, activeCount: 1, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 0, podRunningCount: 3})

	fmt.Printf("Allocating on Build %s with a new sessionID\n", testBuild1Name)
	sessionID1_1 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1_1, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 2, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 0, podRunningCount: 3})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test1BuildID)

	fmt.Printf("Allocating on build %s\n", testBuildSleepBeforeReadyForPlayersName)
	sessionID2 := uuid.New().String()
	if err := allocate(testBuildSleepBeforeReadyForPlayersID, sessionID2, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 2, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, testBuildSleepBeforeReadyForPlayersID)

	fmt.Printf("Allocating on build %s with a new sessionID\n", testBuild1Name)
	sessionID1_2 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1_2, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 3, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test1BuildID)

	fmt.Printf("Allocating on Build %s with a new sessionID\n", testBuild1Name)
	sessionID1_3 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1_3, cert); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 0, activeCount: 4, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})
	validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx, test1BuildID)

	fmt.Printf("Allocating on Build %s with a new sessionID, expecting 429\n", testBuild1Name)
	sessionID1_4 := uuid.New().String()
	if err := allocate(test1BuildID, sessionID1_4, cert); err.Error() != fmt.Sprintf("%s 429", invalidStatusCode) {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 0, activeCount: 4, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	// -------------- Allocation tests end --------------

	// -------------- More scaling tests start --------------
	fmt.Printf("Updating build %s with 3 max - since we have 4 actives, none should be removed\n", testBuild1Name)
	gsb = mpsv1alpha1.GameServerBuild{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.Max = 3
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 0, activeCount: 4, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	fmt.Printf("Updating build %s with 5 max - we should have 1 standingBy\n", testBuild1Name)
	gsb = mpsv1alpha1.GameServerBuild{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.Max = 5
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podRunningCount: 5})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	fmt.Printf("Updating build %s with 4 max and 2 standingBy - since max is 4, we should have 0 standingBy\n", testBuild1Name)
	gsb = mpsv1alpha1.GameServerBuild{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.StandingBy = 2
	gsb.Spec.Max = 4
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 0, activeCount: 4, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	fmt.Printf("Updating build %s with 5 max and 2 standingBy - since max is 5, we should have 1 standingBy\n", testBuild1Name)
	gsb = mpsv1alpha1.GameServerBuild{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: testBuild1Name, Namespace: testNamespace}, &gsb); err != nil {
		handleError(err)
	}
	gsb.Spec.StandingBy = 2
	gsb.Spec.Max = 5
	if err := kubeClient.Update(ctx, &gsb); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podRunningCount: 5})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	// -------------- More scaling tests end --------------

	// -------------- HTTP Server validation start --------------

	fmt.Printf("Allocating on Build %s with a non-Guid sessionID, expecting 400\n", testBuild1Name)
	sessionID1_5 := "notAGuid"
	if err := allocate(test1BuildID, sessionID1_5, cert); err.Error() != fmt.Sprintf("%s 400", invalidStatusCode) {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podRunningCount: 5})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	fmt.Printf("Allocating on Build %s with a non-Guid BuildID, expecting 400\n", testBuild1Name)
	sessionID1_6 := uuid.New().String()
	if err := allocate("notAGuid", sessionID1_6, cert); err.Error() != fmt.Sprintf("%s 400", invalidStatusCode) {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podRunningCount: 5})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	fmt.Printf("Allocating on Build %s with a non existent BuildID, expecting 404\n", testBuild1Name)
	sessionID1_7 := uuid.New().String()
	if err := allocate(uuid.New().String(), sessionID1_7, cert); err.Error() != fmt.Sprintf("%s 404", invalidStatusCode) {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 1, activeCount: 4, podRunningCount: 5})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	// // -------------- HTTP Server validation end --------------

	// -------------- GameServer process exiting gracefully start --------------

	fmt.Printf("Killing an active gameserver from Build %s\n", testBuild1Name)
	if err := stopActiveGameServer(ctx, test1BuildID, kubeConfig); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 3, podRunningCount: 5})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 1, podRunningCount: 4})

	fmt.Printf("Killing an active gameserver from %s\n", testBuildSleepBeforeReadyForPlayersName)
	if err := stopActiveGameServer(ctx, testBuildSleepBeforeReadyForPlayersID, kubeConfig); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 3, podRunningCount: 5})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 0, podRunningCount: 3})

	fmt.Printf("Killing another active gameserver from Build %s\n", testBuild1Name)
	if err := stopActiveGameServer(ctx, test1BuildID, kubeConfig); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: test1BuildID, buildName: testBuild1Name, standingByCount: 2, activeCount: 2, podRunningCount: 4})
	validateBuildState(ctx, buildState{buildID: testBuildSleepBeforeReadyForPlayersID, buildName: testBuildSleepBeforeReadyForPlayersName, standingByCount: 3, activeCount: 0, podRunningCount: 3})

	// -------------- GameServer process exiting gracefully end --------------

	// -------------- GameServer process exiting with a non-zero code start --------------

	fmt.Println("Creating a Build with failing gameServers")
	if err := kubeClient.Create(ctx, buildWithCrashingGameServers); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: testCrashingBuildID, buildName: testBuildCrashingName, standingByCount: 0, activeCount: 0, crashesCount: 5, loopTimesOverride: 30})

	// -------------- GameServer process exiting with a non-zero code end --------------

	// -------------- Validating a Build that does not call ReadyForPlayers stays at Initializing state start --------------

	fmt.Println("Creating a Build with gameServers that do not call ReadyForPlayers")
	if err := kubeClient.Create(ctx, buildWithoutReadyForPlayers); err != nil {
		handleError(err)
	}
	validateBuildState(ctx, buildState{buildID: testWithoutReadyForPlayersBuildID, buildName: testBuildWithoutReadyForPlayers, standingByCount: 0, activeCount: 0, initializingCount: 0, podRunningCount: 2})

	// -------------- Validating a Build that does not call ReadyForPlayers stays at Initializing state end --------------
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

func validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx context.Context, buildID string) {
	var gameServers mpsv1alpha1.GameServerList
	if err := kubeClient.List(ctx, &gameServers, client.MatchingLabels{LabelBuildID: buildID}); err != nil {
		handleError(err)
	}

	activeGameServers := make([]mpsv1alpha1.GameServer, 0)
	for _, gameServer := range gameServers.Items {
		if gameServer.Status.State == mpsv1alpha1.GameServerStateActive {
			activeGameServers = append(activeGameServers, gameServer)
		}
	}

	var nodeAgentPodList corev1.PodList
	if err := kubeClient.List(ctx, &nodeAgentPodList, client.MatchingLabels{"name": nodeAgentName}); err != nil {
		handleError(err)
	}

	if len(nodeAgentPodList.Items) != 1 {
		handleError(fmt.Errorf("expected 1 node agent pod, got %d", len(nodeAgentPodList.Items)))
	}
	nodeAgentPod := nodeAgentPodList.Items[0]

	nodeAgentLogs, err := getContainerLogs(ctx, coreClient, nodeAgentPod.Name, nodeAgentName, "thundernetes-system")
	if err != nil {
		handleError(err)
	}

	for _, gameServer := range activeGameServers {
		err := retry(loopTimesConst, time.Duration(delayInSecondsForLoopTest)*time.Second, func() error {
			if !strings.Contains(nodeAgentLogs, "sessionCookie:randomCookie") {
				return fmt.Errorf("expected to find 'sessionCookie:randomCookie' in nodeAgent logs, got %s", nodeAgentLogs)
			}

			containerLogs, err := getContainerLogs(ctx, coreClient, gameServer.Name, containerName, gameServer.Namespace)

			if err != nil {
				return err
			}
			if !strings.Contains(containerLogs, "After ReadyForPlayers") { // this string must be the same as the one logged on netcore-sample
				return fmt.Errorf("ReadyForPlayers still blocked for %s, the GSDK was not notified of the GameServer transitioning to Active", gameServer.Name)
			}
			if !strings.Contains(containerLogs, "Config with key sessionId has value") {
				return fmt.Errorf("sessionId was not set on %s", gameServer.Name)
			}
			if !strings.Contains(containerLogs, "Config with key sessionCookie has value randomCookie") {
				return fmt.Errorf("sessionCookie was not set on %s", gameServer.Name)
			}
			if !strings.Contains(containerLogs, "Initial Players: player1-player2") {
				return fmt.Errorf("initial Players was not logged for %s", gameServer.Name)
			}

			// check GameServerDetails
			if err := verifyGameServerDetail(ctx, gameServer.Name, connectedPlayersCount, connectedPlayers); err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			handleError(err)
		}
	}
}

func retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}
		if i >= (attempts - 1) {
			break
		}
		time.Sleep(sleep)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}
