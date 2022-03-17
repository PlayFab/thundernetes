/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(Fail)

	RunSpecs(t, "GameServerBuild Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = mpsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	// generate a port registry for the tests
	portRegistry, err := NewPortRegistry(k8sClient, &mpsv1alpha1.GameServerList{}, 20000, 20100, 1, false, ctrl.Log)
	Expect(err).ToNot(HaveOccurred())

	err = portRegistry.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&GameServerBuildReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		PortRegistry: portRegistry,
		Recorder:     k8sManager.GetEventRecorderFor("GameServerBuildReconciler"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&GameServerReconciler{
		Client:                     k8sManager.GetClient(),
		Scheme:                     k8sManager.GetScheme(),
		PortRegistry:               portRegistry,
		Recorder:                   k8sManager.GetEventRecorderFor("GameServerReconciler"),
		GetPublicIpForNodeProvider: func(_ context.Context, _ client.Reader, _ string) (string, error) { return "testPublicIP", nil },
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	// nasty workaround because of this issue: https://github.com/kubernetes-sigs/controller-runtime/issues/1571
	// alternatives would be
	// 1. set the K8s env test version to 1.20
	// 2. use the solution here https://github.com/kubernetes-sigs/kubebuilder/pull/2302/files#diff-9c68eed99ac3d414e720ba8a0c38b489e359c99da0b50b203a12ebe5a57d5fbfL143
	if !strings.Contains(err.Error(), "timeout waiting for process kube-apiserver to stop") {
		Fail(err.Error())
	}
})
