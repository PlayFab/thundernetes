package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
)

const (
	buildName1     string = "testbuild"
	buildNamespace string = "default"
	buildID1       string = "acb84898-cf73-46e2-8057-314ac557d85d"
	sessionID1     string = "d5f075a4-517b-4bf4-8123-dfa0021aa169"
	gsName         string = "testgs"
)

var _ = Describe("allocation API service input validation tests", func() {
	It("empty body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", nil)
		w := httptest.NewRecorder()
		h := NewAllocationApiServer(nil, nil, nil)
		h.handleAllocationRequest(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("GET method should return error", func() {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/allocate", nil)
		w := httptest.NewRecorder()
		h := NewAllocationApiServer(nil, nil, nil)
		h.handleAllocationRequest(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("bad body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString("{\"foo\":\"bar\"}"))
		w := httptest.NewRecorder()
		h := NewAllocationApiServer(nil, nil, nil)
		h.handleAllocationRequest(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("buildID should be a GUID", func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString("{\"buildID\":\"NOT_A_GUID\",\"sessionID\":\"9bb3bbb2-5031-42fd-8982-5a3f76ef2c8a\"}"))
		w := httptest.NewRecorder()
		h := NewAllocationApiServer(nil, nil, nil)
		h.handleAllocationRequest(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should return NotFound on an empty list", func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString("{\"sessionID\":\"9bb3bbb2-5031-42fd-8982-5a3f76ef2c8a\",\"buildID\":\"9bb3bbb2-5031-42fd-8982-5a3f76ef2c8a\"}"))
		w := httptest.NewRecorder()
		h := NewAllocationApiServer(nil, nil, testNewSimpleK8sClient())
		h.handleAllocationRequest(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should return existing game server when given an existing sessionID", func() {
		client := testNewSimpleK8sClient()
		err := testCreateGameServerAndBuild(client, gsName, buildName1, buildID1, sessionID1, mpsv1alpha1.GameServerStateActive)
		Expect(err).ToNot(HaveOccurred())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString(fmt.Sprintf("{\"sessionID\":\"%s\",\"buildID\":\"%s\"}", sessionID1, buildID1)))
		w := httptest.NewRecorder()
		h := NewAllocationApiServer(nil, nil, client)
		h.handleAllocationRequest(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var rm = RequestMultiplayerServerResponse{}
		err = json.Unmarshal(body, &rm)
		Expect(err).ToNot(HaveOccurred())
		Expect(rm.SessionID).To(Equal(sessionID1))
	})
	It("should allocate a game server", func() {
		client := testNewSimpleK8sClient()
		err := testCreateGameServerAndBuild(client, gsName, buildName1, buildID1, "", mpsv1alpha1.GameServerStateStandingBy)
		Expect(err).ToNot(HaveOccurred())
		err = testCreatePod(client, gsName)
		Expect(err).ToNot(HaveOccurred())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString(fmt.Sprintf("{\"sessionID\":\"%s\",\"buildID\":\"%s\"}", sessionID1, buildID1)))
		w := httptest.NewRecorder()
		h := NewAllocationApiServer(nil, nil, client)
		h.handleAllocationRequest(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var rm = RequestMultiplayerServerResponse{}
		err = json.Unmarshal(body, &rm)
		Expect(err).ToNot(HaveOccurred())
		Expect(rm.SessionID).To(Equal(sessionID1))
	})
})

var _ = Describe("allocation API service queue tests", func() {
	It("should update queue properly during allocations", func() {
		// create a GameServerBuild with 2 standingBy servers
		gsb := testGenerateGameServerBuild(buildName1, buildNamespace, buildID1, 2, 4, false)
		Expect(testk8sClient.Create(context.Background(), &gsb)).Should(Succeed())
		testVerifyTotalGameServerCount(context.Background(), buildID1, 2)
		testUpdateInitializingGameServersToStandingBy(context.Background(), buildID1)
		testVerifyInitializingStandingByActiveByCount(context.Background(), buildID1, 0, 2, 0)
		// verify that references exist in queue
		Eventually(func(g Gomega) {
			testAllocationApiServer.gameServerQueue.mutex.RLock()
			defer testAllocationApiServer.gameServerQueue.mutex.RUnlock()
			_, exists := testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1]
			g.Expect(exists).To(BeTrue())
			g.Expect(len(*testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1].queue)).To(Equal(2))
		}).Should(Succeed())

		// allocate a game server
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString(fmt.Sprintf("{\"sessionID\":\"%s\",\"buildID\":\"%s\"}", uuid.New(), buildID1)))
		w := httptest.NewRecorder()
		testAllocationApiServer.handleAllocationRequest(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))

		// validate that 2 game servers are in the queue (since 1 standingBy was created)
		Eventually(func(g Gomega) {
			testUpdateInitializingGameServersToStandingBy(context.Background(), buildID1)
			testAllocationApiServer.gameServerQueue.mutex.RLock()
			defer testAllocationApiServer.gameServerQueue.mutex.RUnlock()
			_, exists := testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1]
			g.Expect(exists).To(BeTrue())
			g.Expect(len(*testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1].queue)).To(Equal(2))
		}).Should(Succeed())

		// do another allocation
		req = httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString(fmt.Sprintf("{\"sessionID\":\"%s\",\"buildID\":\"%s\"}", uuid.New(), buildID1)))
		w = httptest.NewRecorder()
		testAllocationApiServer.handleAllocationRequest(w, req)
		res = w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))

		// validate that there are two game servers in the queue
		Eventually(func(g Gomega) {
			testUpdateInitializingGameServersToStandingBy(context.Background(), buildID1)
			testAllocationApiServer.gameServerQueue.mutex.RLock()
			defer testAllocationApiServer.gameServerQueue.mutex.RUnlock()
			_, exists := testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1]
			g.Expect(exists).To(BeTrue())
			g.Expect(len(*testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1].queue)).To(Equal(2))
		}).Should(Succeed())

		// downscale the Build to one standingBy
		Eventually(func(g Gomega) {
			testVerifyInitializingStandingByActiveByCount(context.Background(), buildID1, 0, 2, 2)
			testUpdateGameServerBuild(context.Background(), 1, 4, buildName1)
			testVerifyInitializingStandingByActiveByCount(context.Background(), buildID1, 0, 1, 2)
		}).Should(Succeed())

		// validate that there is one game server in the queue
		Eventually(func(g Gomega) {
			testUpdateInitializingGameServersToStandingBy(context.Background(), buildID1)
			testAllocationApiServer.gameServerQueue.mutex.RLock()
			defer testAllocationApiServer.gameServerQueue.mutex.RUnlock()
			_, exists := testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1]
			g.Expect(exists).To(BeTrue())
			g.Expect(len(*testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1].queue)).To(Equal(1))
		}).Should(Succeed())

		// downscale the Build to zero standingBy
		Eventually(func(g Gomega) {
			testUpdateGameServerBuild(context.Background(), 0, 4, buildName1)
			testVerifyInitializingStandingByActiveByCount(context.Background(), buildID1, 0, 0, 2)
		}).Should(Succeed())

		// validate that there are no more game servers in the queue
		Eventually(func(g Gomega) {
			testUpdateInitializingGameServersToStandingBy(context.Background(), buildID1)
			testAllocationApiServer.gameServerQueue.mutex.RLock()
			defer testAllocationApiServer.gameServerQueue.mutex.RUnlock()
			_, exists := testAllocationApiServer.gameServerQueue.queuesPerBuilds[buildID1]
			g.Expect(exists).To(BeFalse())
		}).Should(Succeed())

		// allocate a game server, make sure we get a 429
		req = httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString(fmt.Sprintf("{\"sessionID\":\"%s\",\"buildID\":\"%s\"}", uuid.New(), buildID1)))
		w = httptest.NewRecorder()
		testAllocationApiServer.handleAllocationRequest(w, req)
		res = w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusTooManyRequests))
	})
})
