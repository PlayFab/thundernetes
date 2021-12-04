package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	scheme2 "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	testGameServerName      = "testgs"
	testGameServerNamespace = "default"
)

var _ = Describe("allocation API service tests", func() {
	It("heartbeat with empty body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessionHosts/sessionHostID", nil)
		w := httptest.NewRecorder()
		heartbeatHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("heartbeat with empty fields should return error", func() {
		hb := &HeartbeatRequest{}
		b, _ := json.Marshal(hb)
		req := httptest.NewRequest(http.MethodPost, "/v1/sessionHosts/sessionHostID", bytes.NewReader(b))
		w := httptest.NewRecorder()
		heartbeatHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("heartbeat with body should work", func() {
		hb := &HeartbeatRequest{
			CurrentGameState:  GameStateStandingBy,
			CurrentGameHealth: "Healthy",
		}
		b, _ := json.Marshal(hb)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", testGameServerName), bytes.NewReader(b))
		w := httptest.NewRecorder()

		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		// TODO: highjacking global variable, we should use DI here
		dynamicClient = newDynamicInterface()

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		gameServerMap.Insert(testGameServerName, &GameServerDetails{
			GameServerNamespace: testGameServerNamespace,
			Mutex:               &sync.RWMutex{},
		})
		Expect(err).ToNot(HaveOccurred())
		heartbeatHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		resBody, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		hbr := HeartbeatResponse{}
		_ = json.Unmarshal(resBody, &hbr)
		Expect(hbr.Operation).To(Equal(GameOperationContinue))
	})
})

func newDynamicInterface() dynamic.Interface {
	SchemeBuilder := &scheme.Builder{GroupVersion: gameserverGVR.GroupVersion()}
	SchemeBuilder.AddToScheme(scheme2.Scheme)
	gvrMap := make(map[schema.GroupVersionResource]string)
	gvrMap[gameserverGVR] = "GameServerList"
	return fake.NewSimpleDynamicClientWithCustomListKinds(scheme2.Scheme, gvrMap)
}

func createUnstructuredTestGameServer(name, namespace string) *unstructured.Unstructured {
	g := map[string]interface{}{
		"apiVersion": "mps.playfab.com/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]interface{}{"name": name, "namespace": namespace},
		"spec": map[string]interface{}{
			"titleID": "testTitleID",
			"buildID": "testBuildID",
			"portsToExpose": []interface{}{
				map[string]interface{}{
					"containerName": "containerName",
					"portName":      "portName",
				},
			},
		},
		"status": map[string]interface{}{
			"health": "",
			"state":  "",
		},
	}
	return &unstructured.Unstructured{Object: g}
}

func TestSidecar(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "NodeAgent suite")
}
