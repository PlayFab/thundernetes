package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"github.com/playfab/thundernetes/pkg/operator/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	buildName1 string = "testBuild"
	buildID1   string = "acb84898-cf73-46e2-8057-314ac557d85d"
	sessionID1 string = "d5f075a4-517b-4bf4-8123-dfa0021aa169"
	gsName     string = "testgs"
)

var _ = Describe("allocation API service tests", func() {
	It("empty body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", nil)
		w := httptest.NewRecorder()
		h := &allocateHandler{}
		h.handle(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("GET method should return error", func() {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/allocate", nil)
		w := httptest.NewRecorder()
		h := &allocateHandler{}
		h.handle(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("bad body should return error", func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString("{\"foo\":\"bar\"}"))
		w := httptest.NewRecorder()
		h := &allocateHandler{}
		h.handle(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("buildID should be a GUID", func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString("{\"buildID\":\"NOT_A_GUID\",\"sessionID\":\"9bb3bbb2-5031-42fd-8982-5a3f76ef2c8a\"}"))
		w := httptest.NewRecorder()
		h := &allocateHandler{}
		h.handle(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should return NotFound on an empty list", func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString("{\"sessionID\":\"9bb3bbb2-5031-42fd-8982-5a3f76ef2c8a\",\"buildID\":\"9bb3bbb2-5031-42fd-8982-5a3f76ef2c8a\"}"))
		w := httptest.NewRecorder()
		h := &allocateHandler{
			client: newTestSimpleK8s(),
		}
		h.handle(w, req)
		res := w.Result()
		defer res.Body.Close()
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
		_, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should return existing game server when given an existing sessionID", func() {
		client := newTestSimpleK8s()
		err := createTestGameServerAndBuild(client, gsName, buildName1, buildID1, sessionID1, mpsv1alpha1.GameServerStateActive)
		Expect(err).ToNot(HaveOccurred())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString(fmt.Sprintf("{\"sessionID\":\"%s\",\"buildID\":\"%s\"}", sessionID1, buildID1)))
		w := httptest.NewRecorder()
		h := &allocateHandler{
			client: client,
		}
		h.handle(w, req)
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
		client := newTestSimpleK8s()
		err := createTestGameServerAndBuild(client, gsName, buildName1, buildID1, "", mpsv1alpha1.GameServerStateStandingBy)
		Expect(err).ToNot(HaveOccurred())
		err = createTestPod(client, gsName)
		Expect(err).ToNot(HaveOccurred())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString(fmt.Sprintf("{\"sessionID\":\"%s\",\"buildID\":\"%s\"}", sessionID1, buildID1)))
		w := httptest.NewRecorder()
		h := &allocateHandler{
			client: client,
		}
		h.handle(w, req)
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
	// this is commented out as the fake client does not implement field selector indexing yet
	//https://github.com/kubernetes-sigs/controller-runtime/issues/1376
	// It("should return 429 when there are no more servers to allocate", func() {
	// 	client := newTestSimpleK8s()
	// 	err := createTestGameServerAndBuild(client, gsName, buildName1, buildID1, sessionID1, mpsv1alpha1.GameServerStateActive)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	err = createTestPod(client, gsName)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	req := httptest.NewRequest(http.MethodPost, "/api/v1/allocate", bytes.NewBufferString(fmt.Sprintf("{\"sessionID\":\"%s\",\"buildID\":\"%s\"}", sessionID2, buildID1)))
	// 	w := httptest.NewRecorder()
	// 	h := &allocateHandler{
	// 		client: client,
	// 		changeStatusInternalProvider: func(podIP, state, sessionCookie, sessionId string, initialPlayers []string) error {
	// 			return nil
	// 		},
	// 	}
	// 	h.handle(w, req)
	// 	res := w.Result()
	// 	defer res.Body.Close()
	// 	_, err = ioutil.ReadAll(res.Body)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	Expect(res.StatusCode).To(Equal(http.StatusTooManyRequests))
	// })
})

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Allocation API Service Suite")
}

func newTestSimpleK8s() client.Client {
	cb := fake.NewClientBuilder()
	err := mpsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())
	return cb.Build()
}

func createTestGameServerAndBuild(client client.Client, gameServerName, buildName, buildID, sessionID string, state mpsv1alpha1.GameServerState) error {
	gsb := mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: "default",
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID: buildID,
		},
	}
	err := client.Create(context.Background(), &gsb)
	if err != nil {
		return err
	}
	gs := mpsv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gameServerName,
			Namespace: "default",
			Labels: map[string]string{
				controllers.LabelBuildID:   buildID,
				controllers.LabelBuildName: buildName,
			},
		},
		Status: mpsv1alpha1.GameServerStatus{
			SessionID: sessionID,
			State:     state,
		},
	}
	err = client.Create(context.Background(), &gs)
	if err != nil {
		return err
	}
	return nil
}

func createTestPod(client client.Client, gsName string) error {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: gsName,
			Labels: map[string]string{
				controllers.LabelOwningGameServer: gsName,
			},
		},
	}
	err := client.Create(context.Background(), &pod)
	if err != nil {
		return err
	}
	return nil
}
