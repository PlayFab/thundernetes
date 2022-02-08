package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEndToEnd(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(Fail)

	RunSpecs(t, "End To End Suite")
}
