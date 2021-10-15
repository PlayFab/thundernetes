package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	scheme2 "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	gameServerName      = "testgs"
	gameServerNamespace = "default"
)

var _ = Describe("API server tests", func() {
	It("heartbeat with empty body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessionHosts/sessionHostID", nil)
		w := httptest.NewRecorder()
		h := NewHttpHandler(newDynamicInterface(), gameServerName, gameServerNamespace)
		h.heartbeatHandler(w, req)
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
		h := NewHttpHandler(newDynamicInterface(), gameServerName, gameServerNamespace)
		h.heartbeatHandler(w, req)
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
		req := httptest.NewRequest(http.MethodPost, "/v1/sessionHosts/sessionHostID", bytes.NewReader(b))
		w := httptest.NewRecorder()
		h := NewHttpHandler(newDynamicInterface(), gameServerName, gameServerNamespace)
		gs := createUnstructuredTestGameServer(gameServerName, gameServerNamespace)

		_, err := h.k8sClient.Resource(gameserverGVR).Namespace(gameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})

		Expect(err).ToNot(HaveOccurred())
		h.heartbeatHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		resBody, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		hbr := HeartbeatResponse{}
		_ = json.Unmarshal(resBody, &hbr)
		Expect(hbr.Operation).To(Equal(GameOperationContinue))
	})
	It("change state request with empty body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/changeState", nil)
		w := httptest.NewRecorder()
		h := NewHttpHandler(newDynamicInterface(), gameServerName, gameServerNamespace)
		h.changeStateHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("change state request with empty body should return error", func() {
		s := SessionDetails{}
		b, _ := json.Marshal(s)
		req := httptest.NewRequest(http.MethodPost, "/v1/changeState", bytes.NewReader(b))
		w := httptest.NewRecorder()
		h := NewHttpHandler(newDynamicInterface(), gameServerName, gameServerNamespace)
		h.changeStateHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("change state request with non-empty body should work", func() {
		s := SessionDetails{
			State:     "GameStateActive",
			SessionId: "sessionId",
		}
		b, _ := json.Marshal(s)
		req := httptest.NewRequest(http.MethodPost, "/v1/changeState", bytes.NewReader(b))
		w := httptest.NewRecorder()
		h := NewHttpHandler(newDynamicInterface(), gameServerName, gameServerNamespace)
		h.changeStateHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		Expect(h.userSetSessionDetails.SessionId).To(Equal("sessionId"))
		Expect(h.userSetSessionDetails.State).To(Equal("GameStateActive"))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
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

	RunSpecsWithDefaultAndCustomReporters(t,
		"Sidecar Suite",
		[]Reporter{printer.NewlineReporter{}})
}
