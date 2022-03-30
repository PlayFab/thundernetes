package main

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	img          string
	certFile     string
	keyFile      string
	fakeCertFile string
	fakeKeyFile  string
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
})
