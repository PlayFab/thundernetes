package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	port        = 5001
	host        = "localhost"
	contentType = "application/json"
)

var _ = Describe("GameServerAPI tests", func() {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	testNamespace := "gameserverapi"
	url := fmt.Sprintf("http://%s:%d/api/v1", host, port)

	buildName := "test-build-gameserverapi"
	buildNameNoImage := "test-build-gameserverapi-no-image"

	It("should return an error when creating a GameServerBuild that does not have a name", func() {
		build := createGameServerBuild(buildNameNoImage, testNamespace, img)
		build.Name = ""
		b, err := json.Marshal(build)
		Expect(err).ToNot(HaveOccurred())
		req, err := client.Post(url+"/gameserverbuilds", contentType, bytes.NewReader(b))
		Expect(err).ToNot(HaveOccurred())
		Expect(req.StatusCode).To(Equal(http.StatusInternalServerError))
	})

	It("should return an error when deleting a non-existent GameServerBuild", func() {
		req, err := http.NewRequest("DELETE", url+"/gameserverbuilds/nonexistentbuild", nil)
		Expect(err).ToNot(HaveOccurred())
		res, err := client.Do(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("should return an error when getting an non-existing GameServerBuild", func() {
		r, err := client.Get(fmt.Sprintf("%s/gameserverbuilds/%s/%s", url, testNamespace, "nonexistentbuild"))
		Expect(err).ToNot(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("should return an error when getting an non-existing GameServer", func() {
		r, err := client.Get(fmt.Sprintf("%s/gameservers/%s/%s", url, testNamespace, "nonexistentgameserver"))
		Expect(err).ToNot(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("should create a new GameServerBuild, list it and then delete it", func() {
		build := createGameServerBuild(buildName, testNamespace, img)
		b, err := json.Marshal(build)
		Expect(err).ToNot(HaveOccurred())
		req, err := client.Post(url+"/gameserverbuilds", contentType, bytes.NewReader(b))
		Expect(err).ToNot(HaveOccurred())
		Expect(req.StatusCode).To(Equal(http.StatusCreated))

		// list all the GameServerBuilds and see if the one we created is there
		r, err := client.Get(url + "/gameserverbuilds")
		Expect(err).ToNot(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusOK))
		defer r.Body.Close()
		var l mpsv1alpha1.GameServerBuildList
		body, err := ioutil.ReadAll(r.Body)
		Expect(err).ToNot(HaveOccurred())
		err = json.Unmarshal(body, &l)
		Expect(err).ToNot(HaveOccurred())
		var found bool
		for _, b := range l.Items {
			if b.Name == buildName {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue())

		// get the specific GameServerBuild
		r, err = client.Get(fmt.Sprintf("%s/gameserverbuilds/%s/%s", url, testNamespace, buildName))
		Expect(err).ToNot(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusOK))
		var bu mpsv1alpha1.GameServerBuild
		body, err = ioutil.ReadAll(r.Body)
		Expect(err).ToNot(HaveOccurred())
		err = json.Unmarshal(body, &bu)
		Expect(err).ToNot(HaveOccurred())
		Expect(bu.Name).To(Equal(buildName))

		// list GameServers for GameServerBuild
		var gsList mpsv1alpha1.GameServerList
		Eventually(func(g Gomega) {
			r, err = client.Get(fmt.Sprintf("%s/gameserverbuilds/%s/%s/gameservers", url, testNamespace, buildName))
			g.Expect(err).ToNot(HaveOccurred())
			body, err = ioutil.ReadAll(r.Body)
			g.Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal(body, &gsList)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(len(gsList.Items)).To(Equal(2))
		}, timeout, interval).Should(Succeed())

		gsName := gsList.Items[0].Name
		// get a GameServer
		r, err = client.Get(fmt.Sprintf("%s/gameservers/%s/%s", url, testNamespace, gsName))
		Expect(err).ToNot(HaveOccurred())
		var gs mpsv1alpha1.GameServer
		body, err = ioutil.ReadAll(r.Body)
		Expect(err).ToNot(HaveOccurred())
		err = json.Unmarshal(body, &gs)
		Expect(err).ToNot(HaveOccurred())
		Expect(gs.Name).To(Equal(gsName))

		// delete this GameServer
		req1, err := http.NewRequest("DELETE", fmt.Sprintf("%s/gameservers/%s/%s", url, testNamespace, gsName), nil)
		Expect(err).ToNot(HaveOccurred())
		res, err := client.Do(req1)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.StatusCode).To(Equal(http.StatusOK))

		// make sure this GameServer is not returned any more
		// a finalizer runs so it will not disappear at once
		Eventually(func() int {
			r, err = client.Get(fmt.Sprintf("%s/gameservers/%s/%s", url, testNamespace, gsName))
			Expect(err).ToNot(HaveOccurred())
			return r.StatusCode
		}, timeout, interval).Should(Equal(http.StatusNotFound))

		// make sure controller creates an extra GameServer
		Eventually(func() int {
			r, err = client.Get(fmt.Sprintf("%s/gameserverbuilds/%s/%s/gameservers", url, testNamespace, buildName))
			Expect(err).ToNot(HaveOccurred())
			var gsList mpsv1alpha1.GameServerList
			body, err = ioutil.ReadAll(r.Body)
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal(body, &gsList)
			Expect(err).ToNot(HaveOccurred())
			return len(gsList.Items)
		}, timeout, interval).Should(Equal(2))

		// TODO: allocate so GameServerDetails can be created
		// TODO: list GameServerDetails for GameServerBuild
		// TODO: get a GameServerDetail

		// update the GameServerBuild to 3 standingBy and 6 max
		patchBody := map[string]interface{}{
			"standingBy": 3,
			"max":        6,
		}
		pb, err := json.Marshal(patchBody)
		Expect(err).ToNot(HaveOccurred())
		req2, err := http.NewRequest("PATCH", fmt.Sprintf("%s/gameserverbuilds/%s/%s", url, testNamespace, buildName), bytes.NewReader(pb))
		Expect(err).ToNot(HaveOccurred())
		res, err = client.Do(req2)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.StatusCode).To(Equal(http.StatusOK))

		// get the specific GameServerBuild again and make sure the values were updated
		r, err = client.Get(fmt.Sprintf("%s/gameserverbuilds/%s/%s", url, testNamespace, buildName))
		Expect(err).ToNot(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusOK))
		body, err = ioutil.ReadAll(r.Body)
		Expect(err).ToNot(HaveOccurred())
		err = json.Unmarshal(body, &bu)
		Expect(err).ToNot(HaveOccurred())
		Expect(bu.Spec.StandingBy).To(Equal(3))
		Expect(bu.Spec.Max).To(Equal(6))

		// delete the GameServerBuild
		req3, err := http.NewRequest("DELETE", fmt.Sprintf("%s/gameserverbuilds/%s/%s", url, testNamespace, buildName), nil)
		Expect(err).ToNot(HaveOccurred())
		res, err = client.Do(req3)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.StatusCode).To(Equal(http.StatusOK))

		// make sure the GameServerBuild is gone
		r, err = client.Get(fmt.Sprintf("%s/gameserverbuilds/%s/%s", url, testNamespace, buildName))
		Expect(err).ToNot(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusNotFound))
	})
})

func createGameServerBuild(name, namespace, img string) mpsv1alpha1.GameServerBuild {
	return mpsv1alpha1.GameServerBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: mpsv1alpha1.GameServerBuildSpec{
			BuildID:       uuid.New().String(),
			TitleID:       "1E03",
			PortsToExpose: []mpsv1alpha1.PortToExpose{{ContainerName: "netcore-sample", PortName: "gameport"}},
			BuildMetadata: []mpsv1alpha1.BuildMetadataItem{
				{Key: "metadatakey1", Value: "metadatavalue1"},
				{Key: "metadatakey2", Value: "metadatavalue2"},
				{Key: "metadatakey3", Value: "metadatavalue3"},
			},
			StandingBy: 2,
			Max:        4,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           img,
							Name:            "netcore-sample",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "gameport",
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}
