package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
)

const (
	displayPortRegistryVariablesDuringTesting = false
	testnamespace                             = "default"
	timeout                                   = time.Second * 10
	duration                                  = time.Second * 10
	interval                                  = time.Millisecond * 250
)

var _ = Describe("Port registry tests", func() {
	log := logr.FromContextOrDiscard(context.Background())
	portRegistry, err := NewPortRegistry(mpsv1alpha1.GameServerList{}, 20000, 20010, log)
	Expect(err).ToNot(HaveOccurred())
	registeredPorts := make([]int32, 7)
	assignedPorts := make(map[int32]bool)

	It("should allocate hostPorts when creating game servers", func() {
		// get 4 ports
		for i := 0; i < 4; i++ {
			port, err := portRegistry.GetNewPort()
			Expect(err).ToNot(HaveOccurred())
			if _, ok := assignedPorts[port]; ok {
				Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
			}
			assignedPorts[port] = true
		}

		verifyAssignedHostPorts(portRegistry, assignedPorts)
		verifyUnassignedHostPorts(portRegistry, assignedPorts)

		if displayPortRegistryVariablesDuringTesting {
			portRegistry.displayRegistry()
		}

		go portRegistry.portProducer()
		// end of initialization
	})
	It("should allocate more ports", func() {
		for i := 0; i < 7; i++ {
			peekPort, err := peekNextPort(portRegistry)
			actualPort, _ := portRegistry.GetNewPort()
			Expect(actualPort).To(BeIdenticalTo(peekPort), fmt.Sprintf("Wrong port returned, peekPort:%d, actualPort:%d", peekPort, actualPort))
			Expect(err).ToNot(HaveOccurred())

			registeredPorts[i] = actualPort

			assignedPorts[actualPort] = true

			verifyAssignedHostPorts(portRegistry, assignedPorts)
			verifyUnassignedHostPorts(portRegistry, assignedPorts)
		}

		if displayPortRegistryVariablesDuringTesting {
			portRegistry.displayRegistry()
		}
	})

	It("should return an error when we have exceeded the number of allocated ports", func() {
		_, err := peekNextPort(portRegistry)
		if err == nil {
			Expect(err).To(HaveOccurred())
		}

		_, err = portRegistry.GetNewPort()
		if err == nil {
			Expect(err).To(HaveOccurred())
		}

		if displayPortRegistryVariablesDuringTesting {
			portRegistry.displayRegistry()
		}
	})
	It("should successfully deallocate ports", func() {
		portRegistry.DeregisterServerPorts(registeredPorts)

		for _, val := range registeredPorts {
			delete(assignedPorts, val)
		}

		if displayPortRegistryVariablesDuringTesting {
			portRegistry.displayRegistry()
		}

		verifyAssignedHostPorts(portRegistry, assignedPorts)
		verifyUnassignedHostPorts(portRegistry, assignedPorts)
	})
	It("should return another port", func() {

		peekPort, err := peekNextPort(portRegistry)
		actualPort, _ := portRegistry.GetNewPort()
		Expect(actualPort).To(BeNumerically("==", peekPort), fmt.Sprintf("Wrong port returned, peekPort:%d,actualPort:%d", peekPort, actualPort))
		Expect(err).ToNot(HaveOccurred())

		assignedPorts[actualPort] = true

		verifyAssignedHostPorts(portRegistry, assignedPorts)
		verifyUnassignedHostPorts(portRegistry, assignedPorts)

		if displayPortRegistryVariablesDuringTesting {
			portRegistry.displayRegistry()
		}

		portRegistry.Stop()
	})
})

func verifyAssignedHostPorts(portRegistry *PortRegistry, assignedHostPorts map[int32]bool) {
	for hostPort := range assignedHostPorts {
		val, ok := portRegistry.HostPorts[hostPort]
		Expect(ok).Should(BeTrue(), fmt.Sprintf("There should be an entry about hostPort %d", hostPort))
		Expect(val).Should(BeTrue(), fmt.Sprintf("HostPort %d should be registered", hostPort))
	}

}

func verifyUnassignedHostPorts(portRegistry *PortRegistry, assignedHostPorts map[int32]bool) {
	Expect(len(portRegistry.HostPorts)).Should(BeNumerically("==", int(portRegistry.Max-portRegistry.Min+1)))
	for hostPort, ok := range portRegistry.HostPorts {
		if ok { //ignore the assigned ones
			continue
		}
		exists := assignedHostPorts[hostPort]
		Expect(exists).To(BeFalse())
	}
}

func peekNextPort(pr *PortRegistry) (int32, error) {
	tempPointer := pr.NextFreePortIndex
	tempPointerCopy := tempPointer
	for {
		if !pr.HostPorts[pr.Indexes[tempPointer]] { //port not taken
			return pr.Indexes[tempPointer], nil
		}

		tempPointer++
		if tempPointer == (pr.Max - pr.Min + 1) {
			tempPointer = 0
		}

		if tempPointer == tempPointerCopy {
			//we've done a full circle, no ports available
			return 0, errors.New("No ports available")
		}
	}
}
