package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fetchMetricsViaPortForward uses Kubernetes port-forward to fetch /metrics from the controller pod.
// This works with distroless containers since it doesn't require any tools inside the container.
func fetchMetricsViaPortForward(kubeConfig *rest.Config, coreClient *kubernetes.Clientset, pod *corev1.Pod) (string, error) {
	transport, upgrader, err := spdy.RoundTripperFor(kubeConfig)
	if err != nil {
		return "", err
	}

	reqURL := coreClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward").
		URL()

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, reqURL)

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	fw, err := portforward.New(dialer, []string{"0:8080"}, stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		return "", err
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- fw.ForwardPorts()
	}()

	select {
	case <-readyChan:
	case err := <-errChan:
		return "", fmt.Errorf("port-forward failed: %w", err)
	}
	defer close(stopChan)

	ports, err := fw.GetPorts()
	if err != nil {
		return "", err
	}
	if len(ports) == 0 {
		return "", fmt.Errorf("no ports forwarded")
	}
	localPort := ports[0].Local

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", localPort))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// This test verifies that Prometheus metrics are correctly tracked during game server lifecycle operations.
// It creates a build, performs allocations, and verifies that the relevant metrics counters are incremented.
var _ = Describe("Prometheus metrics validation", func() {
	testBuildMetricsName := "metricstest"
	testBuildMetricsID := "a2ffe8da-c82f-4035-86c5-9d2b5f42d6a2"
	It("should track metrics for game server operations", func() {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		kubeConfig := ctrl.GetConfigOrDie()
		kubeClient, err := createKubeClient(kubeConfig)
		Expect(err).ToNot(HaveOccurred())
		coreClient, err := kubernetes.NewForConfig(kubeConfig)
		Expect(err).ToNot(HaveOccurred())

		// create the build
		gsb := createTestBuild(testBuildMetricsName, testBuildMetricsID, img)
		gsb.Spec.StandingBy = 2
		gsb.Spec.Max = 4
		err = kubeClient.Create(ctx, gsb)
		Expect(err).ToNot(HaveOccurred())

		// wait for standingBy
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildMetricsName,
				buildID:         testBuildMetricsID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// allocate a server
		sessionID := uuid.New().String()
		Expect(allocate(testBuildMetricsID, sessionID, cert)).To(Succeed())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testBuildMetricsName,
				buildID:         testBuildMetricsID,
				standingByCount: 2,
				activeCount:     1,
				podRunningCount: 3,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// try to allocate with a non-existent BuildID to get a 404
		nonExistentID := uuid.New().String()
		_ = allocate(nonExistentID, uuid.New().String(), cert)

		// try to allocate with an exhausted pool for a 429
		// first, max out the build by allocating all remaining servers
		for i := 0; i < 3; i++ {
			_ = allocate(testBuildMetricsID, uuid.New().String(), cert)
		}
		// now try one more that should fail with 429
		_ = allocate(testBuildMetricsID, uuid.New().String(), cert)

		// get the controller pod
		var podList corev1.PodList
		err = kubeClient.List(ctx, &podList, client.InNamespace(thundernetesSystemNamespace), client.MatchingLabels{
			"control-plane": "controller-manager",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(1))

		controllerPod := podList.Items[0]

		// fetch metrics via port-forward (works with distroless containers)
		Eventually(func(g Gomega) {
			metricsOutput, err := fetchMetricsViaPortForward(kubeConfig, coreClient, &controllerPod)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(metricsOutput).ToNot(BeEmpty())

			// verify key metrics exist with correct labels
			g.Expect(strings.Contains(metricsOutput, "thundernetes_gameservers_created_total")).To(BeTrue(),
				"expected gameservers_created_total metric")
			g.Expect(strings.Contains(metricsOutput, "thundernetes_gameservers_current_state_per_build")).To(BeTrue(),
				"expected gameservers_current_state_per_build metric")
			g.Expect(strings.Contains(metricsOutput, "thundernetes_allocations_total")).To(BeTrue(),
				"expected allocations_total metric")

			// verify the metrics include our build name
			g.Expect(strings.Contains(metricsOutput, fmt.Sprintf(`BuildName="%s"`, testBuildMetricsName))).To(BeTrue(),
				"expected metrics to include our build name, got: %s", metricsOutput)

			// verify allocations counter was incremented
			g.Expect(strings.Contains(metricsOutput,
				fmt.Sprintf(`thundernetes_allocations_total{BuildName="%s"}`, testBuildMetricsName))).To(BeTrue(),
				"expected allocations_total with our build name")
		}, timeout, interval).Should(Succeed())
	})
})
