package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace = "unittestnamespace"
	url           = "/api/v1"
)

var _ = Describe("GameServer API service tests", func() {
	It("should create a GameServerBuild", func() {
		testBuild2 := mpsv1alpha1.GameServerBuild{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      "test-build2",
				Namespace: testNamespace,
			},
			Spec: mpsv1alpha1.GameServerBuildSpec{
				StandingBy: 1,
				Max:        2,
			},
		}
		b, err := json.Marshal(testBuild2)
		Expect(err).ToNot(HaveOccurred())
		r := setupRouter()
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/gameserverbuilds", url), bytes.NewReader(b))
		Expect(err).ToNot(HaveOccurred())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusCreated))
	})
	It("should return GameServerBuildList with two items", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameserverbuilds", url), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var l mpsv1alpha1.GameServerBuildList
		err = json.Unmarshal(body, &l)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(l.Items)).To(Equal(2))
		Expect(l.Items[0].Name).To(Equal("test-build"))
		Expect(l.Items[1].Name).To(Equal("test-build2"))
	})
	It("should update the second GameServerBuild", func() {
		patchBody := map[string]interface{}{
			"standingBy": 2,
			"max":        4,
		}
		pb, err := json.Marshal(patchBody)
		Expect(err).ToNot(HaveOccurred())
		r := setupRouter()
		req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/gameserverbuilds/%s/test-build2", url, testNamespace), bytes.NewReader(pb))
		Expect(err).ToNot(HaveOccurred())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var b mpsv1alpha1.GameServerBuild
		err = json.Unmarshal(body, &b)
		Expect(err).ToNot(HaveOccurred())
		Expect(b.Spec.StandingBy).To(Equal(2))
		Expect(b.Spec.Max).To(Equal(4))
	})
	It("should return bad request for bad arguments", func() {
		patchBody := map[string]interface{}{
			"standingByWrong": 2,
			"max":             4,
		}
		pb, err := json.Marshal(patchBody)
		Expect(err).ToNot(HaveOccurred())
		r := setupRouter()
		req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/gameserverbuilds/%s/test-build2", url, testNamespace), bytes.NewReader(pb))
		Expect(err).ToNot(HaveOccurred())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
	})
	It("should return bad request for bad arguments - take 2", func() {
		patchBody := map[string]interface{}{
			"standingBy": 2,
			"max":        "wrong",
		}
		pb, err := json.Marshal(patchBody)
		Expect(err).ToNot(HaveOccurred())
		r := setupRouter()
		req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/gameserverbuilds/%s/test-build2", url, testNamespace), bytes.NewReader(pb))
		Expect(err).ToNot(HaveOccurred())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
	})
	It("should return 404 when GameServerBuild does not exist", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameserverbuilds/%s/wrong-build", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
	})
	It("should return GameServerBuild with name test-build", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameserverbuilds/%s/test-build", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var b mpsv1alpha1.GameServerBuild
		err = json.Unmarshal(body, &b)
		Expect(err).ToNot(HaveOccurred())
		Expect(b.Name).To(Equal("test-build"))
	})
	It("should list GameServers for GameServerBuild", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameserverbuilds/%s/test-build/gameservers", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var l mpsv1alpha1.GameServerList
		err = json.Unmarshal(body, &l)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(l.Items)).To(Equal(1))
		Expect(l.Items[0].Name).To(Equal("test-gameserver"))
	})
	It("should get 404 on a non-existent GameServer", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameservers/%s/wrong-server", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
	})
	It("should get a GameServer", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameservers/%s/test-gameserver", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var g mpsv1alpha1.GameServer
		err = json.Unmarshal(body, &g)
		Expect(err).ToNot(HaveOccurred())
		Expect(g.Name).To(Equal("test-gameserver"))
	})
	It("should list GameServerDetails for Build", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameserverbuilds/%s/test-build/gameserverdetails", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var l mpsv1alpha1.GameServerDetailList
		err = json.Unmarshal(body, &l)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(l.Items)).To(Equal(1))
		Expect(l.Items[0].Name).To(Equal("test-gameserver"))
	})
	It("should get 404 on a non-existent GameServerDetail", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameserverdetails/%s/wrong-server", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
	})
	It("should get a GameServerDetail", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameserverdetails/%s/test-gameserver", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var g mpsv1alpha1.GameServerDetail
		err = json.Unmarshal(body, &g)
		Expect(err).ToNot(HaveOccurred())
		Expect(g.Name).To(Equal("test-gameserver"))
	})
	It("should list all GameServers", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/gameservers", url), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var l mpsv1alpha1.GameServerList
		err = json.Unmarshal(body, &l)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(l.Items)).To(BeNumerically(">=", 1))
	})
	It("should return bad request when patching with invalid JSON body", func() {
		r := setupRouter()
		req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/gameserverbuilds/%s/test-build", url, testNamespace), strings.NewReader("not valid json"))
		Expect(err).ToNot(HaveOccurred())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
	})
	It("should return 200 OK on healthz", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var m map[string]string
		err = json.Unmarshal(body, &m)
		Expect(err).ToNot(HaveOccurred())
		Expect(m["status"]).To(Equal("ok"))
	})
	It("should return CORS headers on OPTIONS request", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodOptions, fmt.Sprintf("%s/gameserverbuilds", url), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		Expect(res.Header.Get("Access-Control-Allow-Origin")).To(Equal("*"))
		Expect(res.Header.Get("Access-Control-Allow-Methods")).To(ContainSubstring("POST"))
	})
	It("should return bad request when standingBy > max", func() {
		patchBody := map[string]interface{}{
			"standingBy": 10,
			"max":        5,
		}
		pb, err := json.Marshal(patchBody)
		Expect(err).ToNot(HaveOccurred())
		r := setupRouter()
		req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/gameserverbuilds/%s/test-build", url, testNamespace), bytes.NewReader(pb))
		Expect(err).ToNot(HaveOccurred())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
	})
	It("should return bad request when creating GameServerBuild with bad JSON", func() {
		r := setupRouter()
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/gameserverbuilds", url), strings.NewReader("not valid json"))
		Expect(err).ToNot(HaveOccurred())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
	})
	It("should return 404 when patching non-existent GameServerBuild", func() {
		patchBody := map[string]interface{}{
			"standingBy": 1,
			"max":        2,
		}
		pb, err := json.Marshal(patchBody)
		Expect(err).ToNot(HaveOccurred())
		r := setupRouter()
		req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/gameserverbuilds/%s/non-existent-build", url, testNamespace), bytes.NewReader(pb))
		Expect(err).ToNot(HaveOccurred())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
	})
	It("should delete a GameServer", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/gameservers/%s/test-gameserver", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var m map[string]string
		err = json.Unmarshal(body, &m)
		Expect(err).ToNot(HaveOccurred())
		Expect(m["message"]).To(Equal("Game server deleted"))
	})
	It("should return 404 when deleting non-existent GameServer", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/gameservers/%s/non-existent-server", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
	})
	It("should delete a GameServerBuild", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/gameserverbuilds/%s/test-build", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		var m map[string]string
		err = json.Unmarshal(body, &m)
		Expect(err).ToNot(HaveOccurred())
		Expect(m["message"]).To(Equal("Game server build deleted"))
	})
	It("should return 404 when deleting non-existent GameServerBuild", func() {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/gameserverbuilds/%s/non-existent-build", url, testNamespace), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
	})
})

func TestGameServerAPI(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "GameServer API Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")

	testBuild := &mpsv1alpha1.GameServerBuild{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "test-build",
			Namespace: testNamespace,
		},
	}

	testGameServer := &mpsv1alpha1.GameServer{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "test-gameserver",
			Namespace: testNamespace,
			Labels: map[string]string{
				"BuildName": "test-build",
			},
		},
	}

	testGameServerDetail := &mpsv1alpha1.GameServerDetail{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "test-gameserver",
			Namespace: testNamespace,
			Labels: map[string]string{
				"BuildName": "test-build",
			},
		},
	}

	err := mpsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testBuild, testGameServer, testGameServerDetail)

	kubeClient = clientBuilder.Build()
	Expect(kubeClient).NotTo(BeNil())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
})
