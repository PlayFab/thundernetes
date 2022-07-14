package main

import (
	"context"
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These *Ordered* tests have to be run without any other tests running in parallel
// This is because they modify the number of Nodes in the cluster
// and this could have an impact in the other tests, since taking down a Node might take down active GameServers
// Moreover, we should NOT change the string in the Describe method below, since it is used by the test framework to designate that this test should run by itself
// Bear in mind that in production environments, active GameServers should not normally go down by the cluster autoscaler since we add the "safe-to-evict:false" Annotation to the Pods
var _ = Describe("Cluster with variable number of Nodes", Ordered, func() {
	testClusterScalingName := "scaling"
	testClusterScalingID := "85ffe8da-a7e1-a1c3-86c5-9d2b5f42a12e"
	thundernetesSystemNamespace := "thundernetes-system"
	kubeConfig := ctrl.GetConfigOrDie()
	kubeClient, err := createKubeClient(kubeConfig)
	Expect(err).ToNot(HaveOccurred())
	coreClient, err := kubernetes.NewForConfig(kubeConfig)
	Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()
	worker3 := "kind-worker3"

	It("should be able to create a Build", func() {
		err = kubeClient.Create(ctx, createTestBuild(testClusterScalingName, testClusterScalingID, img))
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testClusterScalingName,
				buildID:         testClusterScalingID,
				standingByCount: 2,
				podRunningCount: 2,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
	It("should scale this Build to 10 standingBy", func() {
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testClusterScalingName, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 10
		gsb.Spec.Max = 20
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testClusterScalingName,
				buildID:         testClusterScalingID,
				standingByCount: 10,
				podRunningCount: 10,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
	It("should mark node 3 as Unschedulable", func() {
		node3 := &corev1.Node{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: worker3}, node3)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(node3.DeepCopy())
		node3.Spec.Unschedulable = true
		err = kubeClient.Patch(ctx, node3, patch)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should evict all Pods on Node 3", func() {
		pods := &corev1.PodList{}
		err = kubeClient.List(ctx, pods, client.InNamespace(testNamespace))
		Expect(err).ToNot(HaveOccurred())
		evictedPodsNames := []string{}
		evictedPodsPorts := []int32{}
		for _, pod := range pods.Items {
			if pod.Spec.NodeName == worker3 {
				evictedPodsNames = append(evictedPodsNames, pod.Name)
				evictedPodsPorts = append(evictedPodsPorts, pod.Spec.Containers[0].Ports[0].HostPort)
				coreClient.CoreV1().Pods(testNamespace).Evict(ctx, &policyv1beta1.Eviction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					},
				})
			}
		}
		// make sure that all evicted Pods have been deleted
		for _, pod := range evictedPodsNames {
			Eventually(func(g Gomega) {
				tempPod := &corev1.Pod{}
				err = kubeClient.Get(ctx, client.ObjectKey{Name: pod, Namespace: testNamespace}, tempPod)
				g.Expect(err).To(HaveOccurred())
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
		}
		// get controller pod name
		podList := &corev1.PodList{}
		err = kubeClient.List(ctx, podList, client.InNamespace(thundernetesSystemNamespace), client.MatchingLabels(map[string]string{
			"control-plane": "controller-manager",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(1))
		controllerPodName := podList.Items[0].Name
		// make sure that the HostPorts have been released
		Eventually(func(g Gomega) {
			controllerLogs, err := getContainerLogs(ctx, coreClient, controllerPodName, "manager", thundernetesSystemNamespace)
			g.Expect(err).ToNot(HaveOccurred())
			for _, port := range evictedPodsPorts {
				// log entries are in this format
				// 2022-07-02T23:27:55Z    DEBUG   portregistry    Deregistering port      {"port": 10010}
				matched, err := regexp.MatchString(fmt.Sprintf("Deregistering port\\s+{\"port\": %d,", port), controllerLogs)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(matched).To(BeTrue())
			}
		}, timeout, interval).Should(Succeed())
	})
	It("should have 10 standingBy after the cluster scale down", func() {
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testClusterScalingName,
				buildID:         testClusterScalingID,
				standingByCount: 10,
				podRunningCount: 10,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
	It("should mark node 3 as Schedulable again", func() {
		node3 := &corev1.Node{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: worker3}, node3)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(node3.DeepCopy())
		node3.Spec.Unschedulable = false
		err = kubeClient.Patch(ctx, node3, patch)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should scale to 20 standingBy", func() {
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testClusterScalingName, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 20
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testClusterScalingName,
				buildID:         testClusterScalingID,
				standingByCount: 20,
				podRunningCount: 20,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
	It("should have at least one Pod on Node 3", func() {
		// for this test, we're relying on the kube scheduler trying to balance the load among the Nodes
		// so it will definitely schedule a Pod on Node 3
		pods := &corev1.PodList{}
		err = kubeClient.List(ctx, pods, client.InNamespace(testNamespace))
		Expect(err).ToNot(HaveOccurred())
		podExistsOnNode3 := false
		for _, pod := range pods.Items {
			if pod.Spec.NodeName == worker3 {
				podExistsOnNode3 = true
				break
			}
		}
		Expect(podExistsOnNode3).To(BeTrue())
	})
	It("should scale to 5 standingBy to decrease load on the cluster for the rest of the tests", func() {
		gsb := &mpsv1alpha1.GameServerBuild{}
		err = kubeClient.Get(ctx, client.ObjectKey{Name: testClusterScalingName, Namespace: testNamespace}, gsb)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(gsb.DeepCopy())
		gsb.Spec.StandingBy = 5
		err = kubeClient.Patch(ctx, gsb, patch)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func(g Gomega) {
			state := buildState{
				buildName:       testClusterScalingName,
				buildID:         testClusterScalingID,
				standingByCount: 5,
				podRunningCount: 5,
				gsbHealth:       mpsv1alpha1.BuildHealthy,
			}
			g.Expect(verifyGameServerBuildOverall(ctx, kubeClient, state)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})
})
