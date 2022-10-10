package main

import (
	"context"
	"os"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	img                  string
	certFile             string
	keyFile              string
	fakeCertFile         string
	fakeKeyFile          string
	allocationApiSvcPort string
)

func TestEndToEnd(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(Fail)

	RunSpecs(t, "End To End Suite")
}

var _ = BeforeSuite(func() {
	img = os.Getenv("IMG")
	Expect(img).NotTo(BeEmpty())
	certFile = os.Getenv("TLS_PUBLIC")
	Expect(certFile).ToNot(BeEmpty())
	keyFile = os.Getenv("TLS_PRIVATE")
	Expect(keyFile).ToNot(BeEmpty())
	fakeCertFile = os.Getenv("FAKE_TLS_PUBLIC")
	Expect(fakeCertFile).ToNot(BeEmpty())
	fakeKeyFile = os.Getenv("FAKE_TLS_PRIVATE")
	Expect(fakeKeyFile).ToNot(BeEmpty())
	allocationApiSvcPort = GetAllocationApiSvcPort()
	Expect(allocationApiSvcPort).ToNot(BeEmpty())
})

// Function to pull allocation api svc port from the controller
func GetAllocationApiSvcPort() string {
	kubeConfig := ctrl.GetConfigOrDie()
	kubeClient, err := createKubeClient(kubeConfig)
	Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()
	svc := corev1.Service{}
	err = kubeClient.Get(ctx, types.NamespacedName{Namespace: "thundernetes-system", Name: "thundernetes-controller-manager"}, &svc)
	Expect(err).ToNot(HaveOccurred())
	return strconv.Itoa(int(svc.Spec.Ports[0].Port))
}
