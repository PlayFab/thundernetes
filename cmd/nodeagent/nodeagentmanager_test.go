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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	scheme2 "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	testGameServerName      = "testgs"
	testGameServerNamespace = "default"
	testNodeName            = "testnode"
	testBuildName           = "testBuild"
	numberOfAttemps         = 3
)

// most tests here are marked as Flakey because of https://github.com/PlayFab/thundernetes/issues/238

var _ = Describe("nodeagent tests", func() {
	It("heartbeat with empty body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessionHosts/sessionHostID", nil)
		w := httptest.NewRecorder()
		n := NewNodeAgentManager(newDynamicInterface(), testNodeName, false, time.Now)
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
		n := NewNodeAgentManager(newDynamicInterface(), testNodeName, false, time.Now)
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

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		n.gameServerMap.Store(testGameServerName, &GameServerInfo{
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
	It("should transition properly from standingBy to Active", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		var gsinfo interface{}
		Eventually(func() bool {
			var ok bool
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// simulate subsequent updates by GSDK
		gsinfo.(*GameServerInfo).PreviousGameState = GameStateStandingBy
		gsinfo.(*GameServerInfo).PreviousGameHealth = "Healthy"

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
			tempgs.(*GameServerInfo).Mutex.RLock()
			gsd := *tempgs.(*GameServerInfo)
			tempgs.(*GameServerInfo).Mutex.RUnlock()
			return gsd.IsActive && gsd.PreviousGameState == GameStateStandingBy
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

		// next heartbeat response should be continue
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
		Expect(hbr.Operation).To(Equal(GameOperationContinue))
	})
	It("should not create a GameServerDetail if the server is not Active", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		Eventually(func() bool {
			var ok bool
			_, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// simulate 5 standingBy heartbeats
		for i := 0; i < 5; i++ {
			hb := &HeartbeatRequest{
				CurrentGameState:  GameStateStandingBy,
				CurrentGameHealth: "Healthy",
			}
			b, _ := json.Marshal(hb)
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", testGameServerName), bytes.NewReader(b))
			w := httptest.NewRecorder()

			n.heartbeatHandler(w, req)
			res := w.Result()
			defer res.Body.Close()
			Expect(res.StatusCode).To(Equal(http.StatusOK))
			resBody, err := ioutil.ReadAll(res.Body)
			Expect(err).ToNot(HaveOccurred())
			hbr := HeartbeatResponse{}
			_ = json.Unmarshal(resBody, &hbr)
			Expect(hbr.Operation).To(Equal(GameOperationContinue))
		}

		_, err = dynamicClient.Resource(gameserverDetailGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

	})
	It("should delete the GameServer from the cache when it's deleted from Kubernetes", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		Eventually(func() bool {
			var ok bool
			_, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		err = dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Delete(context.Background(), gs.GetName(), metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			var ok bool
			_, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())
	})
	It("should create a GameServerDetail when the GameServer transitions to Active", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		var gsinfo interface{}
		Eventually(func() bool {
			var ok bool
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// simulate subsequent updates by GSDK
		gsinfo.(*GameServerInfo).PreviousGameState = GameStateStandingBy
		gsinfo.(*GameServerInfo).PreviousGameHealth = "Healthy"

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
			tempgs.(*GameServerInfo).Mutex.RLock()
			gsd := *tempgs.(*GameServerInfo)
			tempgs.(*GameServerInfo).Mutex.RUnlock()
			return gsd.IsActive && gsd.PreviousGameState == GameStateStandingBy
		}).Should(BeTrue())

		// wait till the GameServerDetail CR has been created
		Eventually(func(g Gomega) {
			u, err := dynamicClient.Resource(gameserverDetailGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(u.GetName()).To(Equal(gs.GetName()))
		}).Should(Succeed())
	})
	It("should not create a GameServerDetail when an Unhealthy GameServer transitions to Active", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		var gsinfo interface{}
		Eventually(func() bool {
			var ok bool
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// simulate subsequent updates by GSDK
		gsinfo.(*GameServerInfo).PreviousGameState = GameStateStandingBy
		gsinfo.(*GameServerInfo).PreviousGameHealth = "Healthy"

		// update GameServer CR to active and unhealthy
		gs.Object["status"].(map[string]interface{})["state"] = "Active"
		gs.Object["status"].(map[string]interface{})["health"] = "Unhealthy"
		_, err = dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Update(context.Background(), gs, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the update trigger on the watch
		// making sure that the Active signal is not sent to the GameServer
		Eventually(func() bool {
			tempgs, ok := n.gameServerMap.Load(testGameServerName)
			if !ok {
				return false
			}
			tempgs.(*GameServerInfo).Mutex.RLock()
			gsd := *tempgs.(*GameServerInfo)
			tempgs.(*GameServerInfo).Mutex.RUnlock()
			return gsd.IsActive == false && gsd.PreviousGameState == GameStateStandingBy
		}).Should(BeTrue())

		// and making sure that GameServerDetail has not been created
		Consistently(func(g Gomega) {
			_, err = dynamicClient.Resource(gameserverDetailGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).To(HaveOccurred())
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}).Should(Succeed())
	})
	It("should create a GameServerDetail on subsequent heartbeats, if it fails on the first time", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		var gsinfo interface{}
		Eventually(func() bool {
			var ok bool
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// simulate subsequent updates by GSDK
		gsinfo.(*GameServerInfo).PreviousGameState = GameStateStandingBy
		gsinfo.(*GameServerInfo).PreviousGameHealth = "Healthy"

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
			tempgs.(*GameServerInfo).Mutex.RLock()
			gsd := *tempgs.(*GameServerInfo)
			tempgs.(*GameServerInfo).Mutex.RUnlock()
			return gsd.IsActive && gsd.PreviousGameState == GameStateStandingBy
		}).Should(BeTrue())

		// wait till the GameServerDetail CR has been created
		Eventually(func(g Gomega) {
			u, err := dynamicClient.Resource(gameserverDetailGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(u.GetName()).To(Equal(gs.GetName()))
		}).Should(Succeed())

		// delete the GameServerDetail CR to simulate failure in creating
		Eventually(func(g Gomega) {
			err := dynamicClient.Resource(gameserverDetailGVR).Namespace(testGameServerNamespace).Delete(context.Background(), gs.GetName(), metav1.DeleteOptions{})
			g.Expect(err).ToNot(HaveOccurred())
		}).Should(Succeed())

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

		// next heartbeat response should be continue
		// at the same time, game is adding some connected players
		hb = &HeartbeatRequest{
			CurrentGameState:  GameStateActive, // heartbeat is now active
			CurrentGameHealth: "Healthy",
			CurrentPlayers:    getTestConnectedPlayers(), // adding connected players
		}
		b, _ = json.Marshal(hb)
		req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", testGameServerName), bytes.NewReader(b))
		w = httptest.NewRecorder()
		n.heartbeatHandler(w, req)
		res = w.Result()
		defer res.Body.Close()
		// first heartbeat after Active should fail since the GameServerDetail is missing
		Expect(res.StatusCode).To(Equal(http.StatusInternalServerError))

		// next heartbeat should succeed
		hb = &HeartbeatRequest{
			CurrentGameState:  GameStateActive, // heartbeat is now active
			CurrentGameHealth: "Healthy",
			CurrentPlayers:    getTestConnectedPlayers(),
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
		Expect(hbr.Operation).To(Equal(GameOperationContinue))

		// make sure the GameServerDetail CR has been created
		Eventually(func(g Gomega) {
			u, err := dynamicClient.Resource(gameserverDetailGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(u.GetName()).To(Equal(gs.GetName()))
		}).Should(Succeed())
	})
	It("should handle a lot of simultaneous heartbeats from different game servers", FlakeAttempts(numberOfAttemps), func() {
		rand.Seed(time.Now().UnixNano())

		var wg sync.WaitGroup
		dynamicClient := newDynamicInterface()
		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		for i := 0; i < 100; i++ {
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
				gsdetails.(*GameServerInfo).PreviousGameState = GameStateStandingBy
				gsdetails.(*GameServerInfo).PreviousGameHealth = "Healthy"

				// update GameServer CR to active
				gs.Object["status"].(map[string]interface{})["state"] = "Active"
				gs.Object["status"].(map[string]interface{})["health"] = "Healthy"
				gs.Object["status"].(map[string]interface{})["sessionCookie"] = "cookie123"
				gs.Object["status"].(map[string]interface{})["initialPlayers"] = []interface{}{"player1", "player2"}
				_, err = dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Update(context.Background(), gs, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				// wait for the update trigger on the watch
				Eventually(func() bool {
					tempgs, ok := n.gameServerMap.Load(randomGameServerName)
					if !ok {
						return false
					}
					tempgs.(*GameServerInfo).Mutex.RLock()
					gsd := *tempgs.(*GameServerInfo)
					tempgs.(*GameServerInfo).Mutex.RUnlock()
					return gsd.IsActive && tempgs.(*GameServerInfo).PreviousGameState == GameStateStandingBy
				}).Should(BeTrue())

				// wait till the GameServerDetail CR has been created
				Eventually(func(g Gomega) {
					u, err := dynamicClient.Resource(gameserverDetailGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(u.GetName()).To(Equal(gs.GetName()))
				}).Should(Succeed())

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

				// next heartbeat response should be continue
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
				Expect(hbr.Operation).To(Equal(GameOperationContinue))

			}(randStringRunes(10))
			wg.Wait()
		}
	})
	It("should set CreationTime value when registering a new game server", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		var gsinfo interface{}
		Eventually(func() bool {
			var ok bool
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// the CreationTime value should be initialized
		Expect(gsinfo.(*GameServerInfo).CreationTime).ToNot(Equal(int64(0)))
	})
	It("should set LastHeartbeatTime value when receiving a heartbeat from a game server", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()

		n := NewNodeAgentManager(dynamicClient, testNodeName, false, time.Now)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		var gsinfo interface{}
		Eventually(func() bool {
			var ok bool
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// the LastHeartbeatTime value is uninitialized
		Expect(gsinfo.(*GameServerInfo).LastHeartbeatTime).To(Equal(int64(0)))

		// we send a heartbeat
		hb := &HeartbeatRequest{
			CurrentGameState:  GameStateStandingBy,
			CurrentGameHealth: "Healthy",
		}
		b, _ := json.Marshal(hb)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", testGameServerName), bytes.NewReader(b))
		w := httptest.NewRecorder()

		n.heartbeatHandler(w, req)

		// now the LastHeartbeatTime value should be eventually initialized
		Eventually(func() int64 {
			var ok bool
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			if !ok {
				return 0
			}
			return gsinfo.(*GameServerInfo).LastHeartbeatTime
		}).ShouldNot(Equal(int64(0)))
	})
	It("should mark the game server as unhealthy due to CreationTime", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()
		// set initial time
		customNow := func() time.Time {
			layout := "Mon, 02 Jan 2006 15:04:05 MST"
			value, _ := time.Parse(layout, "Tue, 26 Apr 2022 10:00:00 PST")
			return value
		}
		n := NewNodeAgentManager(dynamicClient, testNodeName, false, customNow)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		// create the game server
		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		Eventually(func() bool {
			_, ok := n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// change time to be 1 min and 1 sec later
		customNow = func() time.Time {
			layout := "Mon, 02 Jan 2006 15:04:05 MST"
			value, _ := time.Parse(layout, "Tue, 26 Apr 2022 10:01:01 PST")
			return value
		}
		n.nowFunc = customNow

		// we run the check
		n.HeartbeatTimeChecker()

		// the game server health status should eventually be Unhealthy
		Eventually(func(g Gomega) {
			u, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(u.GetName()).To(Equal(gs.GetName()))
			_, gameServerHealth, err := parseStateHealth(u)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gameServerHealth).To(Equal("Unhealthy"))
		}).Should(Succeed())
	})
	It("should mark the game server as unhealthy due to LastHeartbeatTime", FlakeAttempts(numberOfAttemps), func() {
		dynamicClient := newDynamicInterface()
		// set initial time
		customNow := func() time.Time {
			layout := "Mon, 02 Jan 2006 15:04:05 MST"
			value, _ := time.Parse(layout, "Tue, 26 Apr 2022 10:00:00 PST")
			return value
		}
		n := NewNodeAgentManager(dynamicClient, testNodeName, false, customNow)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		// create the game server
		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		Eventually(func() bool {
			_, ok := n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// we send a heartbeat
		hb := &HeartbeatRequest{
			CurrentGameState:  GameStateStandingBy,
			CurrentGameHealth: "Healthy",
		}
		b, _ := json.Marshal(hb)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", testGameServerName), bytes.NewReader(b))
		w := httptest.NewRecorder()

		n.heartbeatHandler(w, req)

		// wait for LastHeartbeatTime value to be eventually initialized
		Eventually(func() int64 {
			var ok bool
			var gsinfo interface{}
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			if !ok {
				return 0
			}
			return gsinfo.(*GameServerInfo).LastHeartbeatTime
		}).ShouldNot(Equal(int64(0)))

		// change time to be 6 seconds later
		customNow = func() time.Time {
			layout := "Mon, 02 Jan 2006 15:04:05 MST"
			value, _ := time.Parse(layout, "Tue, 26 Apr 2022 10:00:06 PST")
			return value
		}
		n.nowFunc = customNow

		// we run the check
		n.HeartbeatTimeChecker()

		// the game server health status should eventually be Unhealthy
		Eventually(func(g Gomega) {
			u, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(u.GetName()).To(Equal(gs.GetName()))
			_, gameServerHealth, err := parseStateHealth(u)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gameServerHealth).To(Equal("Unhealthy"))
		}).Should(Succeed())
	})
	It("should not mark the game server as Unhealthy more than once", FlakeAttempts(numberOfAttemps), func() {
		// this test is a bit hacky because if more than one patch to mark Unhealthy are sent
		// the behavior doesn't really change and the code still works, to be able to test
		// this we change an Unhealthy game server back to Healthy and expect that it doesn't
		// go back to Unhealthy
		dynamicClient := newDynamicInterface()
		// set initial time
		customNow := func() time.Time {
			layout := "Mon, 02 Jan 2006 15:04:05 MST"
			value, _ := time.Parse(layout, "Tue, 26 Apr 2022 10:00:00 PST")
			return value
		}
		n := NewNodeAgentManager(dynamicClient, testNodeName, false, customNow)
		gs := createUnstructuredTestGameServer(testGameServerName, testGameServerNamespace)

		// create the game server
		_, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Create(context.Background(), gs, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// wait for the create trigger on the watch
		Eventually(func() bool {
			_, ok := n.gameServerMap.Load(testGameServerName)
			return ok
		}).Should(BeTrue())

		// we send a heartbeat
		hb := &HeartbeatRequest{
			CurrentGameState:  GameStateStandingBy,
			CurrentGameHealth: "Healthy",
		}
		b, _ := json.Marshal(hb)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/sessionHosts/%s", testGameServerName), bytes.NewReader(b))
		w := httptest.NewRecorder()

		n.heartbeatHandler(w, req)

		// wait for LastHeartbeatTime value to be eventually initialized
		Eventually(func() int64 {
			var ok bool
			var gsinfo interface{}
			gsinfo, ok = n.gameServerMap.Load(testGameServerName)
			if !ok {
				return 0
			}
			return gsinfo.(*GameServerInfo).LastHeartbeatTime
		}).ShouldNot(Equal(int64(0)))

		// change time to be 6 seconds later
		customNow = func() time.Time {
			layout := "Mon, 02 Jan 2006 15:04:05 MST"
			value, _ := time.Parse(layout, "Tue, 26 Apr 2022 10:00:06 PST")
			return value
		}
		n.nowFunc = customNow

		// we run the check
		n.HeartbeatTimeChecker()

		// the game server health status should eventually be Unhealthy
		Eventually(func(g Gomega) {
			u, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(u.GetName()).To(Equal(gs.GetName()))
			_, gameServerHealth, err := parseStateHealth(u)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gameServerHealth).To(Equal("Unhealthy"))
		}).Should(Succeed())

		// we patch the game server to be Healthy again
		patch := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": mpsv1alpha1.GameServerStatus{
					Health: mpsv1alpha1.GameServerHealth(healthyStatus),
				},
			},
		}
		payloadBytes, err := json.Marshal(patch)
		Expect(err).ToNot(HaveOccurred())
		_, err = dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Patch(context.Background(), gs.GetName(), types.MergePatchType, payloadBytes, metav1.PatchOptions{}, "status")
		Expect(err).ToNot(HaveOccurred())

		// the game server health status should eventually be Healthy
		Eventually(func(g Gomega) {
			u, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(u.GetName()).To(Equal(gs.GetName()))
			_, gameServerHealth, err := parseStateHealth(u)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gameServerHealth).To(Equal("Healthy"))
		}).Should(Succeed())

		// change time to be 6 seconds later again
		customNow = func() time.Time {
			layout := "Mon, 02 Jan 2006 15:04:05 MST"
			value, _ := time.Parse(layout, "Tue, 26 Apr 2022 10:00:06 PST")
			return value
		}
		n.nowFunc = customNow

		// we run the check again
		n.HeartbeatTimeChecker()

		// the game server health status should stay Healthy
		Consistently(func(g Gomega) {
			u, err := dynamicClient.Resource(gameserverGVR).Namespace(testGameServerNamespace).Get(context.Background(), gs.GetName(), metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(u.GetName()).To(Equal(gs.GetName()))
			_, gameServerHealth, err := parseStateHealth(u)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gameServerHealth).To(Equal("Healthy"))
		}, "1s").Should(Succeed())
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
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"NodeName":  testNodeName,
				"BuildName": testBuildName,
			}},
		"spec": map[string]interface{}{
			"titleID": "testTitleID",
			"buildID": "testBuildID",
			"portsToExpose": []interface{}{
				"80",
			},
		},
		"status": map[string]interface{}{
			"health": "",
			"state":  "",
		},
	}
	return &unstructured.Unstructured{Object: g}
}

func getTestConnectedPlayers() []ConnectedPlayer {
	return []ConnectedPlayer{
		{
			PlayerId: "player1",
		},
		{
			PlayerId: "player2",
		},
	}
}

func TestNodeAgent(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "NodeAgent suite")
}
