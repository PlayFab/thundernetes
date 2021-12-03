package main

import (
	"context"
	"encoding/json"
	"fmt"

	"net/http"
	"os"
	"regexp"

	hm "github.com/cornelk/hashmap"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	gameserverGVR = schema.GroupVersionResource{
		Group:    "mps.playfab.com",
		Version:  "v1alpha1",
		Resource: "gameservers",
	}

	gameserverDetailGVR = schema.GroupVersionResource{
		Group:    "mps.playfab.com",
		Version:  "v1alpha1",
		Resource: "gameserverdetails",
	}

	gameServerMap = &hm.HashMap{}
	typedClient   *kubernetes.Clientset
	dynamicClient dynamic.Interface
)

const (
	logEveryHeartbeat   = true
	GameServerName      = "GameServerName"
	GameServerNamespace = "GameServerNamespace"
)

func main() {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatalf("NODE_NAME environment variable must be set")
	}

	nodeInternalIp := os.Getenv("NODE_INTERNAL_IP")
	if nodeInternalIp == "" {
		log.Fatalf("NODE_INTERNAL_IP environment variable must be set")
	}

	var err error
	typedClient, dynamicClient, err = initializeKubernetesClient()
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("Watching for Pods")
	lo := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	}
	watcher, err := typedClient.CoreV1().Pods(v1.NamespaceAll).Watch(context.Background(), lo)

	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for event := range watcher.ResultChan() {
			pod := event.Object.(*v1.Pod)
			switch event.Type {
			case watch.Added:
				podAdded(pod)
			case watch.Modified:
				podUpdated(pod)
			case watch.Deleted:
				podDeleted(pod)
			}
		}
	}()

	log.Debug("Starting HTTP server")
	http.HandleFunc("/v1/sessionHosts/", heartbeatHandler)
	err = http.ListenAndServe(fmt.Sprintf(":%d", 56001), nil)
	if err != nil {
		log.Fatal(err)
	}
}

func podAdded(pod *v1.Pod) {
	log.WithFields(log.Fields{
		GameServerName:      pod.ObjectMeta.Name,
		GameServerNamespace: pod.ObjectMeta.Namespace,
	}).Debugf("Pod %s/%s added", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

	gameServerMap.Insert(pod.ObjectMeta.Name, &GameServerDetails{
		GameServerNamespace: pod.ObjectMeta.Namespace,
	})
}

func podUpdated(pod *v1.Pod) {
	log.WithFields(log.Fields{
		GameServerName:      pod.ObjectMeta.Name,
		GameServerNamespace: pod.ObjectMeta.Namespace,
	}).Debugf("Pod %s/%s updated", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
}

func podDeleted(pod *v1.Pod) {
	log.WithFields(log.Fields{
		GameServerName:      pod.ObjectMeta.Name,
		GameServerNamespace: pod.ObjectMeta.Namespace,
	}).Debugf("Pod %s/%s deleted", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

	gameServerMap.Del(pod.ObjectMeta.Name)
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	re := regexp.MustCompile(`.*/v1/sessionHosts\/(.*?)(/heartbeats|$)`)
	match := re.FindStringSubmatch(r.RequestURI)

	gameServerName := match[1]

	var hb HeartbeatRequest
	err := json.NewDecoder(r.Body).Decode(&hb)
	if err != nil {
		badRequest(w, err, "cannot deserialize json")
		return
	}

	gsdi, exists := gameServerMap.Get(gameServerName)
	if !exists {
		internalServerError(w, fmt.Errorf("game server %s not found", gameServerName), "gameserver not found")
		return
	}
	gsd := gsdi.(*GameServerDetails)

	if logEveryHeartbeat {
		log.WithFields(log.Fields{
			GameServerName:      gameServerName,
			GameServerNamespace: gsd.GameServerNamespace,
		}).Debugf("heartbeat received from sessionHostId %s, data %#v", gameServerName, hb)
	}

	if err := validateHeartbeatRequestArgs(&hb); err != nil {
		log.WithFields(log.Fields{
			GameServerName:      gameServerName,
			GameServerNamespace: gsd.GameServerNamespace,
		}).Warnf("error validating heartbeat request %s", err.Error())
		badRequest(w, err, "invalid heartbeat request")
		return
	}

	if err := updateHealthAndStateIfNeeded(ctx, &hb, gameServerName, gsd); err != nil {
		log.WithFields(log.Fields{
			GameServerName:      gameServerName,
			GameServerNamespace: gsd.GameServerNamespace,
		}).Errorf("error updating health %s", err.Error())
		internalServerError(w, err, "error updating health")
		return
	}
}

// updateHealthAndStateIfNeeded updates both the health and state of the GameServer if any one of them has changed
func updateHealthAndStateIfNeeded(ctx context.Context, hb *HeartbeatRequest, gameServerName string, gsd *GameServerDetails) error {
	if gsd.CurrentHealth != hb.CurrentGameHealth || gsd.CurrentState != hb.CurrentGameState {
		log.WithFields(log.Fields{
			GameServerName:      gameServerName,
			GameServerNamespace: gsd.GameServerNamespace,
		}).Infof("Health or state is different than before, updating. Old health %s, new health %s, old state %s, new state %s", gsd.CurrentHealth, hb.CurrentGameHealth, gsd.CurrentHealth, hb.CurrentGameState)
		payload := fmt.Sprintf("{\"status\":{\"health\":\"%s\",\"state\":\"%s\"}}", hb.CurrentGameHealth, hb.CurrentGameState)
		payloadBytes := []byte(payload)
		_, err := dynamicClient.Resource(gameserverGVR).Namespace(gsd.GameServerNamespace).Patch(ctx, gameServerName, types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")

		if err != nil {
			return err
		}
		gsd.CurrentHealth = hb.CurrentGameHealth
		gsd.CurrentState = hb.CurrentGameState
	}
	return nil
}
