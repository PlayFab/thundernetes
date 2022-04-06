package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var connectedPlayers = []string{"Amie", "Ken", "Dimitris"} // this should the same as in the netcore sample

const (
	testNamespace                      = "e2e"
	connectedPlayersCount              = 3 // this should the same as in the netcore sample
	LabelBuildID                       = "BuildID"
	invalidStatusCode           string = "invalid status code"
	containerName               string = "netcore-sample" // this must be the same as the GameServer name
	nodeAgentName               string = "nodeagent"
	portKey                     string = "gameport"
	safeToEvictPodAttribute     string = "cluster-autoscaler.kubernetes.io/safe-to-evict"
	timeout                            = time.Second * 30
	interval                           = time.Millisecond * 250
	thundernetesSystemNamespace        = "thundernetes-system"
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
	gsbHealth         mpsv1alpha1.GameServerBuildHealth
}

// createKubeClient creates a new kubernetes client
func createKubeClient(kubeConfig *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = mpsv1alpha1.AddToScheme(scheme)

	return client.New(kubeConfig, client.Options{Scheme: scheme})
}

// validateThatAllocatedServersHaveReadyForPlayersUnblocked checks the logs for Pods whose parent GameServers are Active
// the sample netcore application outputs a special string to the output when it's becoming Active
// so this function
// 1. checks that the string is present in the logs
// 2. checks for other logs (like sessionCookie)
// 3. checks for the presence of the GameServerDetails CR
func validateThatAllocatedServersHaveReadyForPlayersUnblocked(ctx context.Context, kubeClient client.Client, coreClient *kubernetes.Clientset, buildID string, activesCount int) error {
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

	if len(activeGameServers) != activesCount {
		return errors.Errorf("expected %d active game servers, but got %d", activesCount, len(activeGameServers))
	}

	var nodeAgentPodList corev1.PodList
	if err := kubeClient.List(ctx, &nodeAgentPodList, client.MatchingLabels{"name": nodeAgentName}); err != nil {
		return err
	}

	if len(nodeAgentPodList.Items) != 1 {
		return fmt.Errorf("expected 1 node agent pod, got %d", len(nodeAgentPodList.Items))
	}
	nodeAgentPod := nodeAgentPodList.Items[0]

	nodeAgentLogs, err := getContainerLogs(ctx, coreClient, nodeAgentPod.Name, nodeAgentName, thundernetesSystemNamespace)
	if err != nil {
		return err
	}

	for _, gameServer := range activeGameServers {
		Eventually(func() error {
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
			if err := verifyGameServerDetail(ctx, kubeClient, gameServer.Name, connectedPlayersCount, connectedPlayers); err != nil {
				return err
			}

			return nil
		}, timeout, interval).Should(Succeed())
	}
	return nil
}

// stopActiveGameServer find the child Pod for a GameServer and executes a "kill 1" command
// which essentially kills the root process of container, hence the entire container
func stopActiveGameServer(ctx context.Context, kubeClient client.Client, coreClient *kubernetes.Clientset, kubeConfig *rest.Config, buildID string) error {
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

// executeRemoteCommand executes a remote shell command on the given pod
// returns the output from stdout and stderr
func executeRemoteCommand(coreClient *kubernetes.Clientset, pod *corev1.Pod, cfg *rest.Config, args ...string) (string, string, error) {
	command := []string{"/bin/sh", "-c"}
	command = append(command, args...)

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := coreClient.CoreV1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pod.Spec.Containers[0].Name,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, clientgoscheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", request.URL())
	if err != nil {
		return "", "", err
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", errors.Wrapf(err, "failed executing command %s on %v/%v", command, pod.Namespace, pod.Name)
	}

	return buf.String(), errBuf.String(), nil
}

// getContainerLogs returns the logs for a running container
func getContainerLogs(ctx context.Context, coreClient *kubernetes.Clientset, podName, containerName, podNamespace string) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		Container: containerName,
	}
	req := coreClient.CoreV1().Pods(podNamespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	str := buf.String()
	return str, nil
}

// allocate sends a POST request for allocation to the Thundernetes allocation service
func allocate(buildID, sessionID string, cert tls.Certificate) error {
	// curl --key ~/private.pem --cert ~/public.pem --insecure -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5","sessionID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"}' https://${IP}:5000/api/v1/allocate
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	postBody, _ := json.Marshal(map[string]interface{}{
		"buildID":        buildID,
		"sessionID":      sessionID,
		"sessionCookie":  "randomCookie",
		"initialPlayers": []string{"player1", "player2"},
	})
	postBodyBytes := bytes.NewBuffer(postBody)
	resp, err := client.Post("https://localhost:5000/api/v1/allocate", "application/json", postBodyBytes)
	//Handle Error
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %d", invalidStatusCode, resp.StatusCode)
	}
	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	ar := &AllocationResult{}
	json.Unmarshal(body, ar)

	if ar.IPV4Address == "" {
		return fmt.Errorf("invalid IPV4Address %s", ar.IPV4Address)
	}

	if ar.SessionID != sessionID {
		return fmt.Errorf("invalid SessionID %s", ar.SessionID)
	}

	return nil
}

// verifyGameServerBuildOverall checks the state of the Pods, GameServers and the GameServerBuild
// returns error if the state is different than the expected
func verifyGameServerBuildOverall(ctx context.Context, kubeClient client.Client, state buildState) error {
	if err := verifyPods(ctx, kubeClient, state); err != nil {
		return err
	}
	if err := verifyGameServers(ctx, kubeClient, state); err != nil {
		return err
	}
	if err := verifyGameServerBuild(ctx, kubeClient, state); err != nil {
		return err
	}
	return nil
}

// verifyPodsInHostNetwork checks if the Pods are in hostNetwork and if the containerPort is equal to hostPort
// for the ports that we want exposed
func verifyPodsInHostNetwork(ctx context.Context, kubeClient client.Client, gsb *mpsv1alpha1.GameServerBuild, state buildState) error {
	var pods = corev1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(gsb.Namespace),
		client.MatchingLabels{"BuildName": gsb.Name, "BuildID": gsb.Spec.BuildID},
	}

	if err := kubeClient.List(ctx, &pods, opts...); err != nil {
		return err
	}

	if len(pods.Items) != state.podRunningCount {
		return fmt.Errorf("pod count is not equal to the expected %d", state.podRunningCount)
	}

	// for all Pods
	for _, pod := range pods.Items {
		if !pod.Spec.HostNetwork {
			return fmt.Errorf("pod %s is not in host network", pod.Name)
		}
		// for all containers in this Pod
		for _, container := range pod.Spec.Containers {
			containerName := container.Name
			for _, portToExpose := range gsb.Spec.PortsToExpose {
				for _, containerPortMapping := range container.Ports {
					// found a port
					if containerPortMapping.ContainerPort == portToExpose {
						// let's make sure that hostPort is the same as containerPort (work done by the controller)
						if containerPortMapping.HostPort != containerPortMapping.ContainerPort {
							return fmt.Errorf("hostPort != containerPort for hostNetwork pod %s container %s, port %d", pod.Name, containerName, portToExpose)
						}
					}

				}
			}

		}
	}
	return nil
}

// verifyGameServerBuild checks if the GameServerBuild current state is equal to the expected
func verifyGameServerBuild(ctx context.Context, kubeClient client.Client, state buildState) error {
	gameServerBuild := mpsv1alpha1.GameServerBuild{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: state.buildName, Namespace: testNamespace}, &gameServerBuild); err != nil {
		return err
	}

	if gameServerBuild.Status.CurrentInitializing != state.initializingCount {
		return fmt.Errorf("expected %d initializing, got %d", state.initializingCount, gameServerBuild.Status.CurrentInitializing)
	}
	if gameServerBuild.Status.CurrentStandingBy != state.standingByCount {
		return fmt.Errorf("expected %d standingBy, got %d", state.standingByCount, gameServerBuild.Status.CurrentStandingBy)
	}
	if gameServerBuild.Status.CurrentActive != state.activeCount {
		return fmt.Errorf("expected %d active, got %d", state.activeCount, gameServerBuild.Status.CurrentActive)
	}
	if gameServerBuild.Status.CrashesCount < state.crashesCount {
		return fmt.Errorf("expected >=%d crashes, got %d", state.crashesCount, gameServerBuild.Status.CrashesCount)
	}

	if gameServerBuild.Status.Health != state.gsbHealth {
		return fmt.Errorf("expected %s status, got %s", state.gsbHealth, gameServerBuild.Status.Health)
	}

	return nil
}

// verifyGameServers checks if the state of the GameServers is equal to the expected
func verifyGameServers(ctx context.Context, kubeClient client.Client, state buildState) error {
	gameServers := mpsv1alpha1.GameServerList{}
	if err := kubeClient.List(ctx, &gameServers, client.InNamespace(testNamespace)); err != nil {
		return err
	}

	var observedStandingByCount, observedActiveCount int
	for _, gameServer := range gameServers.Items {
		if gameServer.Labels[LabelBuildID] != state.buildID {
			continue
		}

		if gameServer.OwnerReferences == nil {
			return fmt.Errorf("GameServer %s has no OwnerReferences", gameServer.Name)
		}
		if gameServer.OwnerReferences[0].Name != state.buildName {
			return fmt.Errorf(fmt.Sprintf("GameServer %s has incorrect OwnerReferences: %s", gameServer.Name, gameServer.OwnerReferences[0].Name))
		}
		if gameServer.Status.State == mpsv1alpha1.GameServerStateStandingBy {
			observedStandingByCount++
			if err := verifyGameServerPodEvictionAnnotation(ctx, kubeClient, gameServer, "true"); err != nil {
				return err
			}

		} else if gameServer.Status.State == mpsv1alpha1.GameServerStateActive {
			observedActiveCount++
			if err := verifyGameServerPodEvictionAnnotation(ctx, kubeClient, gameServer, "false"); err != nil {
				return err
			}
		}
	}

	if observedStandingByCount != state.standingByCount {
		return fmt.Errorf(fmt.Sprintf("Expected %d gameservers in standingBy, got %d", state.standingByCount, observedStandingByCount))
	}
	if observedActiveCount != state.activeCount {
		return fmt.Errorf(fmt.Sprintf("Expected %d gameservers in active, got %d", state.activeCount, observedActiveCount))
	}
	return nil
}

// verifyGameServerPodEvictionAnnotation checks if the GameServer Pods have the safeToEvict annotation (used by the cluster autoscaler) set appropriately
func verifyGameServerPodEvictionAnnotation(ctx context.Context, kubeClient client.Client, gameserver mpsv1alpha1.GameServer, safeToEvict string) error {
	var pod corev1.Pod
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: gameserver.Namespace, Name: gameserver.Name}, &pod); err != nil {
		return err
	}
	annotations := pod.GetAnnotations()

	if strings.ToLower(annotations[safeToEvictPodAttribute]) != safeToEvict {
		return fmt.Errorf("expected gameserver %s pod %s %s attribute to be marked %s. Got %s", gameserver.Name, pod.Name, safeToEvictPodAttribute, safeToEvict, annotations[safeToEvictPodAttribute])
	}

	return nil
}

// verifyPods checks if the Pod state is equal to the expected
func verifyPods(ctx context.Context, kubeClient client.Client, state buildState) error {
	pods := corev1.PodList{}

	if err := kubeClient.List(ctx, &pods, client.InNamespace(testNamespace)); err != nil {
		return err
	}

	var observedCount int
	for _, pod := range pods.Items {
		if pod.Labels[LabelBuildID] != state.buildID {
			continue
		}
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if pod.OwnerReferences == nil {
			return fmt.Errorf("pod %s has no OwnerReferences", pod.Name)
		}
		if !strings.HasPrefix(pod.OwnerReferences[0].Name, state.buildName) {
			return fmt.Errorf("pod %s has incorrect OwnerReferences: %s", pod.Name, pod.OwnerReferences[0].Name)
		}
		observedCount++
	}

	if observedCount == state.podRunningCount {
		return nil
	}
	return fmt.Errorf("Expecting %d Pods in state Running, got %d", state.podRunningCount, observedCount)
}

// verifyGameServerDetail checks if the GameServerDetail CR is valid and has the appropriate state
func verifyGameServerDetail(ctx context.Context, kubeClient client.Client, gameServerDetailName string, expectedConnectedPlayersCount int, expectedConnectedPlayers []string) error {
	gameServerDetail := mpsv1alpha1.GameServerDetail{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: gameServerDetailName, Namespace: testNamespace}, &gameServerDetail); err != nil {
		return err
	}
	if gameServerDetail.Spec.ConnectedPlayersCount != expectedConnectedPlayersCount {
		return fmt.Errorf("expected %d connected players, got %d", expectedConnectedPlayersCount, gameServerDetail.Spec.ConnectedPlayersCount)
	}
	if len(gameServerDetail.Spec.ConnectedPlayers) != len(expectedConnectedPlayers) {
		return fmt.Errorf("expected %d connected players, got %d", len(expectedConnectedPlayers), len(gameServerDetail.Spec.ConnectedPlayers))
	}

	for i := 0; i < len(gameServerDetail.Spec.ConnectedPlayers); i++ {
		if gameServerDetail.Spec.ConnectedPlayers[i] != expectedConnectedPlayers[i] {
			return fmt.Errorf("expected connected player %s, got %s", expectedConnectedPlayers[i], gameServerDetail.Spec.ConnectedPlayers[i])
		}
	}

	return nil
}

// createTestBuild creates a GameServerBuild with the given name and ID.
func createTestBuild(buildName, buildID, img string) *mpsv1alpha1.GameServerBuild {
	return &mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: testNamespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       buildID,
			TitleID:       "1E03",
			PortsToExpose: []int32{80},
			StandingBy:    2,
			Max:           4,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           img,
							Name:            containerName,
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
}
