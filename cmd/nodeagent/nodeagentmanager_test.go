package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
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
	testNodeName            = "testnode"
)

var _ = Describe("nodeagent tests", func() {
	It("heartbeat with empty body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessionHosts/sessionHostID", nil)
		w := httptest.NewRecorder()
		n := NewNodeAgentManager(newDynamicInterface(), testNodeName)
		n.heartbeatHandler(w, req)
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
		n := NewNodeAgentManager(newDynamicInterface(), testNodeName)
		n.heartbeatHandler(w, req)
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
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		n.gameServerMap.Store(testGameServerName, &GameServerDetails{
			GameServerNamespace: testGameServerNamespace,
			Mutex:               &sync.RWMutex{},
		})

		_, ok := n.gameServerMap.Load(testGameServerName)
		Expect(ok).To(BeTrue())

		n.heartbeatHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		resBody, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		hbr := HeartbeatResponse{}
		_ = json.Unmarshal(resBody, &hbr)
		Expect(hbr.Operation).To(Equal(GameOperationContinue))
	})
	It("should transition properly from standingBy to Active", func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		var gsdetails interface{}
		Eventually(func() bool {
			var ok bool
			gsdetails, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// simulate subsequent updates by GSDK
		gsdetails.(*GameServerDetails).PreviousGameState = GameStateStandingBy
		gsdetails.(*GameServerDetails).PreviousGameHealth = "Healthy"

		// update GameServer CR to active
		gs.Object["status"].(map[string]interface{})["state"] = "Active"
		gs.Object["status"].(map[string]interface{})["health"] = "Healthy"
		_, err = dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Update(context.Background(), gs, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the update trigger on the watch
		Eventually(func() bool {
			tempgs, ok := n.gameServerMap.Load(testGameServerName)
			if !ok {
				return false
			}
			return tempgs.(*GameServerDetails).WasActivated && tempgs.(*GameServerDetails).PreviousGameState == GameStateStandingBy
		}).Should(BeTrue())

		// heartbeat from the game is still StandingBy
		hb := &HeartbeatRequest{
			CurrentGameState:  GameStateStandingBy,
			CurrentGameHealth: "Healthy",
		}
		b, _ := json.Marshal(hb)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", testGameServerName), bytes.NewReader(b))
		w := httptest.NewRecorder()

		// but the response from NodeAgent should be active
		n.heartbeatHandler(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		resBody, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		hbr := HeartbeatResponse{}
		_ = json.Unmarshal(resBody, &hbr)
		Expect(hbr.Operation).To(Equal(GameOperationActive))

		// next heartbeat response should be active as well
		hb = &HeartbeatRequest{
			CurrentGameState:  GameStateActive, // heartbeat is now active
			CurrentGameHealth: "Healthy",
		}
		b, _ = json.Marshal(hb)
		req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", testGameServerName), bytes.NewReader(b))
		w = httptest.NewRecorder()
		n.heartbeatHandler(w, req)
		res = w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		resBody, err = ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		hbr = HeartbeatResponse{}
		err = json.Unmarshal(resBody, &hbr)
		Expect(err).ToNot(HaveOccurred())
		Expect(hbr.Operation).To(Equal(GameOperationActive))
	})
	It("should handle a lot of simultaneous heartbeats from different game servers", func() {
		rand.Seed(time.Now().UnixNano())

		var wg sync.WaitGroup
		dynamicClient := newDynamicInterface()
		n := NewNodeAgentManager(dynamicClient, testNodeName)
		for i := 0; i < 500; i++ {
			wg.Add(1)
			go func(randomGameServerName string) {
				defer GinkgoRecover()
				defer wg.Done()

				gs := createUnstructuredTestGameServer(randomGameServerName, testGameServerNamespace)

				_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				// wait for the create trigger on the watch
				var gsdetails interface{}
				Eventually(func() bool {
					var ok bool
					gsdetails, ok = n.gameServerMap.Load(randomGameServerName)
					return ok
				}).Should(BeTrue())

				// simulate subsequent updates by GSDK
				gsdetails.(*GameServerDetails).PreviousGameState = GameStateStandingBy
				gsdetails.(*GameServerDetails).PreviousGameHealth = "Healthy"

				// update GameServer CR to active
				gs.Object["status"].(map[string]interface{})["state"] = "Active"
				gs.Object["status"].(map[string]interface{})["health"] = "Healthy"
				gs.Object["status"].(map[string]interface{})["sessiocCookie"] = "cookie123"
				_, err = dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Update(context.Background(), gs, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				// wait for the update trigger on the watch
				Eventually(func() bool {
					tempgs, ok := n.gameServerMap.Load(randomGameServerName)
					if !ok {
						return false
					}
					return tempgs.(*GameServerDetails).WasActivated && tempgs.(*GameServerDetails).PreviousGameState == GameStateStandingBy
				}).Should(BeTrue())

				// heartbeat from the game is still StandingBy
				hb := &HeartbeatRequest{
					CurrentGameState:  GameStateStandingBy,
					CurrentGameHealth: "Healthy",
				}
				b, _ := json.Marshal(hb)
				req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", randomGameServerName), bytes.NewReader(b))
				w := httptest.NewRecorder()

				// but the response should be active
				n.heartbeatHandler(w, req)
				res := w.Result()
				defer res.Body.Close()
				Expect(res.StatusCode).To(Equal(http.StatusOK))
				resBody, err := ioutil.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())
				hbr := HeartbeatResponse{}
				_ = json.Unmarshal(resBody, &hbr)
				Expect(hbr.Operation).To(Equal(GameOperationActive))

				// next heartbeat response should be active as well
				hb = &HeartbeatRequest{
					CurrentGameState:  GameStateActive, // heartbeat is now active
					CurrentGameHealth: "Healthy",
				}
				b, _ = json.Marshal(hb)
				req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", randomGameServerName), bytes.NewReader(b))
				w = httptest.NewRecorder()
				n.heartbeatHandler(w, req)
				res = w.Result()
				defer res.Body.Close()
				Expect(res.StatusCode).To(Equal(http.StatusOK))
				resBody, err = ioutil.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())
				hbr = HeartbeatResponse{}
				err = json.Unmarshal(resBody, &hbr)
				Expect(err).ToNot(HaveOccurred())
				Expect(hbr.Operation).To(Equal(GameOperationActive))

			}(randStringRunes(10))
			wg.Wait()
		}
	})
})

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

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
		"metadata":   map[string]interface{}{"name": name, "namespace": namespace, "labels": map[string]interface{}{"NodeName": testNodeName}},
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

func TestNodeAgent(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "NodeAgent suite")
}
