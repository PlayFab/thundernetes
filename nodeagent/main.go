package main

import (
	"context"
	"encoding/json"
	"fmt"

	"net/http"
	"os"
	"regexp"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const logEveryHeartbeat = true

func main() {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatalf("NODE_NAME environment variable must be set")
	}

	typedClient, _, err := initializeKubernetesClient()
	if err != nil {
		log.Fatal(err)
	}

	lo := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	}
	watcher, err := typedClient.CoreV1().Pods(v1.NamespaceDefault).Watch(context.Background(), lo)

	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for event := range watcher.ResultChan() {
			svc := event.Object.(*v1.Pod)
			switch event.Type {
			case watch.Added:
				fmt.Printf("Pod %s/%s added\n\n", svc.ObjectMeta.Namespace, svc.ObjectMeta.Name)
			case watch.Modified:
				fmt.Printf("Pod %s/%s modified\n\n", svc.ObjectMeta.Namespace, svc.ObjectMeta.Name)
			case watch.Deleted:
				fmt.Printf("Pod %s/%s deleted\n\n", svc.ObjectMeta.Namespace, svc.ObjectMeta.Name)
			}
		}
	}()

	http.HandleFunc("/v1/sessionHosts/", heartbeatHandler)
	http.ListenAndServe(fmt.Sprintf("localhost:%d", 56001), nil)
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	//ctx := context.Background()
	re := regexp.MustCompile(`.*/v1/sessionHosts\/(.*?)(/heartbeats|$)`)
	match := re.FindStringSubmatch(r.RequestURI)

	sessionHostId := match[1]

	var hb HeartbeatRequest
	err := json.NewDecoder(r.Body).Decode(&hb)
	if err != nil {
		badRequest(w, err, "cannot deserialize json")
		return
	}

	if logEveryHeartbeat {
		log.Debugf("heartbeat received from sessionHostId %s, data %#v", sessionHostId, hb)
	}

	if err := validateHeartbeatRequestArgs(&hb); err != nil {
		log.Warnf("error validating heartbeat request %s", err.Error())
		badRequest(w, err, "invalid heartbeat request")
		return
	}

	// if err := sm.updateHealthAndStateIfNeeded(ctx, &hb); err != nil {
	// 	log.Errorf("error updating health %s", err.Error())
	// 	internalServerError(w, err, "error updating health")
	// 	return
	// }
}
