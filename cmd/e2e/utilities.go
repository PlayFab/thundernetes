package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	safeToEvictPodAttribute string = "cluster-autoscaler.kubernetes.io/safe-to-evict"
)

// executeRemoteCommand executes a remote shell command on the given pod
// returns the output from stdout and stderr
func executeRemoteCommand(coreClient *kubernetes.Clientset, pod *v1.Pod, cfg *rest.Config, args ...string) (string, string, error) {
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
		VersionedParams(&v1.PodExecOptions{
			Container: pod.Spec.Containers[0].Name,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", request.URL())
	if err != nil {
		return "", "", err
	}

	// was removed since it failed on kind on GitHub Action
	// Put the terminal into raw mode to prevent it echoing characters twice.
	// oldState, err := terminal.MakeRaw(0)
	// if err != nil {
	// 	panic(err)
	// }
	// defer terminal.Restore(0, oldState)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", errors.Wrapf(err, "failed executing command %s on %v/%v", command, pod.Namespace, pod.Name)
	}

	return buf.String(), errBuf.String(), nil
}

func getContainerLogs(ctx context.Context, coreClient *kubernetes.Clientset, podName, containerName, podNamespace string) (string, error) {
	podLogOpts := v1.PodLogOptions{
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

func handleError(err error) {
	log.Panic(err)
}

func loopCheck(fn func(context.Context, string, string, buildState) error, ctx context.Context, state buildState, loopTimes int) error {
	var err error
	for times := 0; times < loopTimes; times++ {
		err = fn(ctx, state.buildID, state.buildName, state)
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(delayInSecondsForLoopTest) * time.Second)
	}
	return err
}

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

func validateBuildState(ctx context.Context, state buildState) {
	var loopTimes int
	if state.loopTimesOverride > 0 {
		loopTimes = state.loopTimesOverride
	} else {
		loopTimes = loopTimesConst
	}

	fmt.Printf("    Verifying that %d pods are in state %s for build %s\n", state.podRunningCount, v1.PodRunning, state.buildName)
	err := loopCheck(verifyPods, ctx, state, loopTimes)
	if err != nil {
		handleError(err)
	}

	fmt.Printf("    Verifying that we have %d initializing, %d standingBy, %d active gameservers for build %s\n", state.initializingCount, state.standingByCount, state.activeCount, state.buildName)
	err = loopCheck(verifyGameServers, ctx, state, loopTimes)
	if err != nil {
		handleError(err)
	}

	fmt.Printf("    Verifying build %s .Status with %d standingBy gameservers, %d active gameservers and %d crashes\n", state.buildName, state.standingByCount, state.activeCount, state.crashesCount)
	err = loopCheck(verifyGameServerBuild, ctx, state, loopTimes)
	if err != nil {
		handleError(err)
	}
}

func verifyGameServerBuild(ctx context.Context, buildID, buildName string, state buildState) error {
	gameServerBuild := mpsv1alpha1.GameServerBuild{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, &gameServerBuild); err != nil {
		panic(err)
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

	// 5 is the default, we should parameterize that
	if gameServerBuild.Status.CrashesCount >= 5 && gameServerBuild.Status.Health != "Unhealthy" {
		return fmt.Errorf("expected %s status, got %s", "Unhealthy", gameServerBuild.Status.Health)
	} else if gameServerBuild.Status.CrashesCount < 5 && gameServerBuild.Status.Health != "Healthy" {
		return fmt.Errorf("expected %s status, got %s", "Healthy", gameServerBuild.Status.Health)
	}

	return nil
}

func verifyGameServers(ctx context.Context, buildID, buildName string, state buildState) error {
	gameServers := mpsv1alpha1.GameServerList{}
	if err := kubeClient.List(ctx, &gameServers, client.InNamespace(testNamespace)); err != nil {
		panic(err)
	}

	var observedStandingByCount, observedActiveCount int
	for _, gameServer := range gameServers.Items {
		if gameServer.Labels[LabelBuildID] != buildID {
			continue
		}

		if gameServer.OwnerReferences == nil {
			return fmt.Errorf("GameServer %s has no OwnerReferences", gameServer.Name)
		}
		if gameServer.OwnerReferences[0].Name != buildName {
			return fmt.Errorf(fmt.Sprintf("GameServer %s has incorrect OwnerReferences: %s", gameServer.Name, gameServer.OwnerReferences[0].Name))
		}
		if gameServer.Status.State == mpsv1alpha1.GameServerStateStandingBy {
			observedStandingByCount++
			if err := verifyGameServerPodEvictionAnnotation(ctx, gameServer, "true"); err != nil {
				return err
			}

		} else if gameServer.Status.State == mpsv1alpha1.GameServerStateActive {
			observedActiveCount++
			if err := verifyGameServerPodEvictionAnnotation(ctx, gameServer, "false"); err != nil {
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

func verifyGameServerPodEvictionAnnotation(ctx context.Context, gameserver mpsv1alpha1.GameServer, safeToEvict string) error {
	var pod v1.Pod
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: gameserver.Namespace, Name: gameserver.Name}, &pod); err != nil {
		return err
	}
	annotations := pod.GetAnnotations()

	if strings.ToLower(annotations[safeToEvictPodAttribute]) != safeToEvict {
		return fmt.Errorf("expected gameserver %s pod %s %s attribute to be marked %s. Got %s", gameserver.Name, pod.Name, safeToEvictPodAttribute, safeToEvict, annotations[safeToEvictPodAttribute])
	}

	return nil
}

func verifyPods(ctx context.Context, buildID, buildName string, state buildState) error {
	pods := v1.PodList{}

	if err := kubeClient.List(ctx, &pods, client.InNamespace(testNamespace)); err != nil {
		return err
	}

	var observedCount int
	for _, pod := range pods.Items {
		if pod.Labels[LabelBuildID] != buildID {
			continue
		}
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		if pod.OwnerReferences == nil {
			return fmt.Errorf("pod %s has no OwnerReferences", pod.Name)
		}
		if !strings.HasPrefix(pod.OwnerReferences[0].Name, buildName) {
			return fmt.Errorf("pod %s has incorrect OwnerReferences: %s", pod.Name, pod.OwnerReferences[0].Name)
		}
		observedCount++
	}

	if observedCount == state.podRunningCount {
		return nil
	}
	return fmt.Errorf("pods not OK, expecting running %d, got running %d", state.podRunningCount, observedCount)
}

func verifyGameServerDetail(ctx context.Context, gameServerDetailName string, expectedConnectedPlayersCount int, expectedConnectedPlayers []string) error {
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
