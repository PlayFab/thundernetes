package controllers

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
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

var _ = Describe("Port registry tests", Ordered, func() {
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
			Expect(actualPort).To(Equal(peekPort), fmt.Sprintf("Wrong port returned, peekPort:%d, actualPort:%d", peekPort, actualPort))
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
		Expect(err).To(HaveOccurred())

		_, err = portRegistry.GetNewPort()
		Expect(err).To(HaveOccurred())

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
		Expect(actualPort).To(Equal(peekPort), fmt.Sprintf("Wrong port returned, peekPort:%d,actualPort:%d", peekPort, actualPort))
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

var _ = Describe("Port registry with one thousand ports", func() {
	rand.Seed(time.Now().UnixNano())
	log := logr.FromContextOrDiscard(context.Background())
	min := int32(20000)
	max := int32(20999)
	portRegistry, err := NewPortRegistry(mpsv1alpha1.GameServerList{}, min, max, log)
	Expect(err).ToNot(HaveOccurred())

	assignedPorts := sync.Map{}
	It("should work with allocating and deallocating ports", func() {
		go portRegistry.portProducer()

		// allocate all 1000 ports
		var wg sync.WaitGroup
		for i := 0; i < int(max-min+1); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				port, err := portRegistry.GetNewPort()
				Expect(err).ToNot(HaveOccurred())
				_, ok := assignedPorts.Load(port)
				if ok {
					Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
				}
				assignedPorts.Store(port, true)
			}()
		}
		wg.Wait()

		if displayPortRegistryVariablesDuringTesting {
			portRegistry.displayRegistry()
		}

		m := syncMapToMapInt32Bool(&assignedPorts)
		verifyAssignedHostPorts(portRegistry, m)
		verifyUnassignedHostPorts(portRegistry, m)

		// trying to get another port should fail, since we've allocated every available port
		_, err := portRegistry.GetNewPort()
		Expect(err).To(HaveOccurred())

		//deallocate the 500 even ports
		for i := 0; i < int(max-min+1)/2; i++ {
			wg.Add(1)
			go func(portToDeallocate int) {
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				err := portRegistry.DeregisterServerPorts([]int32{int32(portToDeallocate)})
				Expect(err).ToNot(HaveOccurred())
				assignedPorts.Delete(int32(portToDeallocate))
			}(i*2 + int(min))
		}
		wg.Wait()

		if displayPortRegistryVariablesDuringTesting {
			portRegistry.displayRegistry()
		}

		m = syncMapToMapInt32Bool(&assignedPorts)
		verifyAssignedHostPorts(portRegistry, m)
		verifyUnassignedHostPorts(portRegistry, m)

		// allocate 500 ports
		for i := 0; i < int(max-min+1)/2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				port, err := portRegistry.GetNewPort()
				Expect(err).ToNot(HaveOccurred())
				_, ok := assignedPorts.Load(port)
				if ok {
					Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
				}
				assignedPorts.Store(port, true)
			}()
		}
		wg.Wait()

		verifyAssignedHostPorts(portRegistry, syncMapToMapInt32Bool(&assignedPorts))
		verifyUnassignedHostPorts(portRegistry, syncMapToMapInt32Bool(&assignedPorts))

		// trying to get another port should fail, since we've allocated every available port
		_, err = portRegistry.GetNewPort()
		Expect(err).To(HaveOccurred())
	})
})

// syncMapToMapInt32Bool converts a sync.Map to a map[int32]bool
// useful as part of our test uses the sync.Map instead of the slice
func syncMapToMapInt32Bool(sm *sync.Map) map[int32]bool {
	m := make(map[int32]bool)
	sm.Range(func(key, value interface{}) bool {
		m[key.(int32)] = value.(bool)
		return true
	})
	return m
}

func verifyAssignedHostPorts(portRegistry *PortRegistry, assignedHostPorts map[int32]bool) {
	Expect(len(portRegistry.HostPorts)).Should(Equal(int(portRegistry.Max - portRegistry.Min + 1)))
	for hostPort := range assignedHostPorts {
		val, ok := portRegistry.HostPorts[hostPort]
		Expect(ok).Should(BeTrue(), fmt.Sprintf("There should be an entry about hostPort %d", hostPort))
		Expect(val).Should(BeTrue(), fmt.Sprintf("HostPort %d should be registered", hostPort))
	}
}

func verifyUnassignedHostPorts(portRegistry *PortRegistry, assignedHostPorts map[int32]bool) {
	Expect(len(portRegistry.HostPorts)).Should(Equal(int(portRegistry.Max - portRegistry.Min + 1)))
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
