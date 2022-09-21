package controllers

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
)

const (
	displayPortRegistryVariablesDuringTesting = false
	testnamespace                             = "default"
	testGsName                                = "testgs"
	testGsName2                               = "testgs2"
	timeout                                   = time.Second * 10
	duration                                  = time.Second * 10
	interval                                  = time.Millisecond * 250
	prefix                                    = "testprefix"
)

var _ = Describe("Port registry tests", func() {
	const testMinPort = 20000
	const testMaxPort = 20009
	It("should allocate hostPorts when creating game servers", func() {
		portRegistry, _ := getPortRegistryKubeClientForTesting(testMinPort, testMaxPort)
		Expect(portRegistry.Min).To(Equal(int32(testMinPort)))
		Expect(portRegistry.Max).To(Equal(int32(testMaxPort)))
		Expect(portRegistry.FreePortsCount).To(Equal(testMaxPort - testMinPort + 1))
		assignedPorts := make(map[int32]int)
		// get 4 ports
		ports, err := portRegistry.GetNewPorts(testnamespace, testGsName, 4)
		Expect(err).ToNot(HaveOccurred())
		for i := 0; i < 4; i++ {
			port := ports[i]
			validatePort(port, testMinPort, testMaxPort)
			Expect(port).To(Equal(int32(testMinPort + i)))
			if _, ok := assignedPorts[port]; ok {
				Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
			}
			assignedPorts[port] = assignedPorts[port] + 1
		}

		verifyExpectedHostPorts(portRegistry, assignedPorts, 4)
	})
	It("should fail to allocate more ports than the maximum", func() {
		portRegistry, _ := getPortRegistryKubeClientForTesting(testMinPort, testMaxPort)
		assignedPorts := make(map[int32]int)
		ports, err := portRegistry.GetNewPorts(testnamespace, testGsName, testMaxPort-testMinPort+1)
		Expect(err).To(Not(HaveOccurred()))
		for i := 0; i < testMaxPort-testMinPort+1; i++ {
			port := ports[i]
			validatePort(port, testMinPort, testMaxPort)
			if _, ok := assignedPorts[port]; ok {
				Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
			}
			assignedPorts[port] = assignedPorts[port] + 1
		}
		verifyExpectedHostPorts(portRegistry, assignedPorts, 10)

		// this one should fail
		_, err = portRegistry.GetNewPorts(testnamespace, testGsName2, 1)
		Expect(err).To(HaveOccurred())
	})

	It("should increase/decrease NodeCount when we add/delete Nodes from the cluster", func() {
		portRegistry, kubeClient := getPortRegistryKubeClientForTesting(testMinPort, testMaxPort)
		node := getNewNodeForTest("node2")
		err := kubeClient.Create(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		// do a manual reconcile since we haven't added the controller to the manager
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
		Eventually(func() error {
			return verifyHostPortsUsage(portRegistry, 2, 2*(testMaxPort-testMinPort+1))
		}).Should(Succeed())

		err = kubeClient.Delete(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
		Eventually(func() error {
			return verifyHostPortsUsage(portRegistry, 1, 1*(testMaxPort-testMinPort+1))
		}).Should(Succeed())
	})

	It("should successfully allocate ports on two Nodes", func() {
		portRegistry, kubeClient := getPortRegistryKubeClientForTesting(testMinPort, testMaxPort)
		node := getNewNodeForTest("node2")
		err := kubeClient.Create(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		// do a manual reconcile since we haven't added the controller to the manager
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
		Eventually(func() error {
			return verifyHostPortsUsage(portRegistry, 2, 2*(testMaxPort-testMinPort+1))
		}).Should(Succeed())

		assignedPorts := make(map[int32]int)
		// get 15 ports
		ports, err := portRegistry.GetNewPorts(testnamespace, testGsName, 15)
		Expect(err).ToNot(HaveOccurred())
		for i := 0; i < 15; i++ {
			port := ports[i]
			validatePort(port, testMinPort, testMaxPort)
			assignedPorts[port] = assignedPorts[port] + 1
		}

		verifyExpectedHostPorts(portRegistry, assignedPorts, 15)
	})

	It("should successfully deallocate ports - one Node scenario", func() {
		portRegistry, _ := getPortRegistryKubeClientForTesting(testMinPort, testMaxPort)
		Expect(portRegistry.Min).To(Equal(int32(testMinPort)))
		Expect(portRegistry.Max).To(Equal(int32(testMaxPort)))
		assignedPorts := make(map[int32]int)
		// allocate 10 ports
		ports := make([]int32, 10)
		for i := 0; i < 10; i++ {
			newPorts, err := portRegistry.GetNewPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, i), 1)
			ports[i] = newPorts[0]
			Expect(err).To(Not(HaveOccurred()))
		}

		for i := 0; i < 10; i++ {
			port := ports[i]
			validatePort(port, testMinPort, testMaxPort)
			if _, ok := assignedPorts[port]; ok {
				Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
			}
			assignedPorts[port] = assignedPorts[port] + 1
		}

		verifyExpectedHostPorts(portRegistry, assignedPorts, 10)
		// deallocate two ports
		ports, err := portRegistry.DeregisterPorts(testnamespace, testGsName+"1")
		Expect(err).ToNot(HaveOccurred())
		Expect(ports[0]).To(Equal(int32(testMinPort + 1)))
		delete(assignedPorts, testMinPort+1)
		ports, err = portRegistry.DeregisterPorts(testnamespace, testGsName+"3")
		Expect(ports[0]).To(Equal(int32(testMinPort + 3)))
		Expect(err).ToNot(HaveOccurred())
		delete(assignedPorts, testMinPort+3)
		verifyExpectedHostPorts(portRegistry, assignedPorts, 8) // 10 minus two
	})

	It("should successfully deallocate ports - two Nodes to one scenario", func() {
		portRegistry, kubeClient := getPortRegistryKubeClientForTesting(testMinPort, testMaxPort)
		Expect(portRegistry.Min).To(Equal(int32(testMinPort)))
		Expect(portRegistry.Max).To(Equal(int32(testMaxPort)))
		assignedPorts := make(map[int32]int)
		// allocate 10 ports
		ports := make([]int32, 10)
		for i := 0; i < 10; i++ {
			newPorts, err := portRegistry.GetNewPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, i), 1)
			ports[i] = newPorts[0]
			Expect(err).To(Not(HaveOccurred()))
		}
		for i := 0; i < 10; i++ {
			port := ports[i]
			validatePort(port, testMinPort, testMaxPort)
			if _, ok := assignedPorts[port]; ok {
				Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
			}
			assignedPorts[port] = assignedPorts[port] + 1
		}
		verifyExpectedHostPorts(portRegistry, assignedPorts, 10)

		// deallocate two ports
		ports, err := portRegistry.DeregisterPorts(testnamespace, testGsName+"1")
		Expect(err).ToNot(HaveOccurred())
		Expect(ports[0]).To(Equal(int32(testMinPort + 1)))
		delete(assignedPorts, testMinPort+1)
		ports, err = portRegistry.DeregisterPorts(testnamespace, testGsName+"3")
		Expect(ports[0]).To(Equal(int32(testMinPort + 3)))
		Expect(err).ToNot(HaveOccurred())
		delete(assignedPorts, testMinPort+3)
		verifyExpectedHostPorts(portRegistry, assignedPorts, 8) // 10 minus two

		// add a second Node
		node := getNewNodeForTest("node2")
		err = kubeClient.Create(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		// do a manual reconcile since we haven't added the controller to the manager
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
		Eventually(func() error {
			return verifyHostPortsUsage(portRegistry, 2, 2*(testMaxPort-testMinPort+1)-8) // ten minus two
		}).Should(Succeed())

		// get 8 more ports, we have 16 in total
		ports = make([]int32, 8)
		for i := 0; i < 8; i++ {
			newPorts, err := portRegistry.GetNewPorts(testnamespace, fmt.Sprintf("%s%d", testGsName2, i), 1)
			Expect(err).To(Not(HaveOccurred()))
			ports[i] = newPorts[0]
		}
		for i := 0; i < 8; i++ {
			port := ports[i]
			validatePort(port, testMinPort, testMaxPort)
			assignedPorts[port] = assignedPorts[port] + 1
		}
		verifyExpectedHostPorts(portRegistry, assignedPorts, 16)

		// deallocate six ports that exist on the second Node
		for i := 0; i < 6; i++ {
			ports := portRegistry.HostPortsPerGameServer[getNamespacedName(testnamespace, fmt.Sprintf("%s%d", testGsName2, i))]
			releasedPorts, err := portRegistry.DeregisterPorts(testnamespace, fmt.Sprintf("%s%d", testGsName2, i))
			Expect(err).ToNot(HaveOccurred())
			Expect(releasedPorts[0]).To(Equal(ports[0]))
			portToDelete := ports[0]
			assignedPorts[portToDelete] = assignedPorts[portToDelete] - 1
			if assignedPorts[portToDelete] == 0 { // we're removing these entries to facilitate verification
				delete(assignedPorts, portToDelete)
			}
		}

		verifyExpectedHostPorts(portRegistry, assignedPorts, 10) // 16 minus 6

		// now delete the second node
		err = kubeClient.Delete(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
		Eventually(func() error {
			return verifyHostPortsUsage(portRegistry, 1, 1*(testMaxPort-testMinPort+1)-10) // 16 minus 6
		}).Should(Succeed())
		verifyExpectedHostPorts(portRegistry, assignedPorts, 10)

		// deallocate three ports (we're doing 7..9 since 1 and 3 have already been deleted)
		for i := 7; i <= 9; i++ {
			releasedPorts, err := portRegistry.DeregisterPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, i))
			Expect(err).ToNot(HaveOccurred())
			Expect(releasedPorts[0]).To(Equal(int32(testMinPort + i)))
			portToDelete := testMinPort + int32(i)
			assignedPorts[portToDelete] = assignedPorts[portToDelete] - 1
			if assignedPorts[portToDelete] == 0 { // we're removing these entries to facilitate verification
				delete(assignedPorts, portToDelete)
			}
		}

		verifyExpectedHostPorts(portRegistry, assignedPorts, 7) // 10 minus three
	})

})

// This test allocates ports from the PortRegistry using predictable GameServer names
// useful to test ordered GameServer creation that leads to port allocation
var _ = Describe("Ordered port registration on port registry with two thousand ports, five hundred on four nodes", func() {
	rand.Seed(time.Now().UnixNano())
	min := int32(20000)
	max := int32(20499)

	portRegistry, kubeClient := getPortRegistryKubeClientForTesting(min, max)
	Expect(portRegistry.Min).To(Equal(min))
	Expect(portRegistry.Max).To(Equal(max))
	// map[port, timesallocated] keeps track of how many times each port has been allocated. Used for assertions
	assignedPorts := sync.Map{}

	// add three more nodes
	for i := 0; i < 3; i++ {
		node := getNewNodeForTest(fmt.Sprintf("node%d", i+2))
		err := kubeClient.Create(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		// we manually reconcile so the extra ports are added to the port registry
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
	}

	Eventually(func() error {
		return verifyHostPortsUsage(portRegistry, 4, 4*int(max-min+1))
	}).Should(Succeed())

	It("should allocate and deallocate ports", func() {
		// allocate all 2000 ports in parallel, one at a time
		var wg sync.WaitGroup
		for i := 0; i < int(max-min+1)*4; i++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				ports, err := portRegistry.GetNewPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, j), 1)
				Expect(err).ToNot(HaveOccurred())
				p := ports[0]                    // we allocate only one port
				val, ok := assignedPorts.Load(p) // check if this is the first time we allocate this port
				if !ok {
					assignedPorts.Store(p, 1)
				} else {
					assignedPorts.Store(p, val.(int)+1)
				}
			}(i)
		}
		wg.Wait()
		m := syncMapToMapInt32Int(&assignedPorts)
		verifyExpectedHostPorts(portRegistry, m, int(max-min+1)*4)

		// trying to get another port should fail, since we've allocated every available port
		_, err := portRegistry.GetNewPorts(testnamespace, "willfail", 1)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorNotEnoughFreePorts))

		//deallocate 1000 ports in parallel from GameServers that end in 0..999
		for i := 0; i < int(max-min+1)*2; i++ {
			wg.Add(1)
			go func(j int) {
				defer GinkgoRecover()
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				releasedPorts, err := portRegistry.DeregisterPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, j))
				Expect(err).ToNot(HaveOccurred())
				Expect(len(releasedPorts)).To(Equal(1))
				val, ok := assignedPorts.Load(releasedPorts[0])
				if !ok {
					Fail(fmt.Sprintf("port %d was not found in the map", j))
				}
				assignedPorts.Store(releasedPorts[0], val.(int)-1)
			}(i)
		}
		wg.Wait()

		m = syncMapToMapInt32Int(&assignedPorts)
		verifyExpectedHostPorts(portRegistry, m, int(max-min+1)*2) // 1000 ports allocated

		// trying to re-register an existing GameServer will fail (GameServer ending in 1000 is already registered)
		_, err = portRegistry.GetNewPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, 1000), 1)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorPortsAlreadyAssignedForThisGameServer))

		// allocate 500 ports in parallel (GameServers ending in 0..499)
		for i := 0; i < int(max-min+1); i++ {
			wg.Add(1)
			go func(j int) {
				defer GinkgoRecover()
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				ports, err := portRegistry.GetNewPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, j), 1)
				Expect(err).ToNot(HaveOccurred())
				port := ports[0]
				val, ok := assignedPorts.Load(port)
				if !ok {
					assignedPorts.Store(port, 1)
				} else {
					assignedPorts.Store(port, val.(int)+1)
				}
			}(i)
		}
		wg.Wait()

		m = syncMapToMapInt32Int(&assignedPorts)
		verifyExpectedHostPorts(portRegistry, m, int(max-min+1)*3) // 1500 ports allocated

		// trying to re-register an existing GameServer will fail (GameServer ending in 0 is already registered)
		_, err = portRegistry.GetNewPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, 0), 1)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorPortsAlreadyAssignedForThisGameServer))

		//allocate the last 500 ports in parallel (GameServers ending in 500..999)
		for i := max - min + 1; i < int32(max-min+1)*2; i++ {
			wg.Add(1)
			go func(j int) {
				defer GinkgoRecover()
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				ports, err := portRegistry.GetNewPorts(testnamespace, fmt.Sprintf("%s%d", testGsName, j), 1)
				Expect(err).ToNot(HaveOccurred())
				port := ports[0]
				val, ok := assignedPorts.Load(port)
				if !ok {
					assignedPorts.Store(port, 1)
				} else {
					assignedPorts.Store(port, val.(int)+1)
				}
			}(int(i))
		}
		wg.Wait()

		m = syncMapToMapInt32Int(&assignedPorts)
		verifyExpectedHostPorts(portRegistry, m, int(max-min+1)*4)

		// trying to get another port should fail, since we've allocated every available port
		_, err = portRegistry.GetNewPorts(testnamespace, "willfail2", 1)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorNotEnoughFreePorts))
	})
})

// this test verifies that the port registry can handle concurrent requests to allocate ports
// from random GameServer names
var _ = Describe("Random port registration on port registry with two thousand ports, five hundred on four nodes", func() {
	rand.Seed(time.Now().UnixNano())
	min := int32(20000)
	max := int32(20499)

	portRegistry, kubeClient := getPortRegistryKubeClientForTesting(min, max)
	Expect(portRegistry.Min).To(Equal(min))
	Expect(portRegistry.Max).To(Equal(max))
	// map[GameServerName, port] to save the GameServer/Port tuples that we have registered
	gameServerNamesAndPorts := sync.Map{}

	// add three more nodes
	for i := 0; i < 3; i++ {
		node := getNewNodeForTest(fmt.Sprintf("node%d", i+2))
		err := kubeClient.Create(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
	}

	Eventually(func() error {
		return verifyHostPortsUsage(portRegistry, 4, 4*int(max-min+1))
	}).Should(Succeed())

	It("should work with allocating and deallocating ports", func() {
		// allocate all 2000 ports in parallel, one at a time
		var wg sync.WaitGroup
		for i := 0; i < int(max-min+1)*4; i++ {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				var gameServerName string
				for { // make sure we don't register the same GameServer twice
					gameServerName = generateName(prefix)
					if _, ok := gameServerNamesAndPorts.Load(gameServerName); !ok {
						break
					}
				}
				ports, err := portRegistry.GetNewPorts(testnamespace, gameServerName, 1)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(ports)).To(Equal(1))
				gameServerNamesAndPorts.Store(gameServerName, ports[0])
			}()
		}
		wg.Wait()
		Expect(len(portRegistry.HostPortsPerGameServer)).To(Equal(int(max-min+1) * 4))

		// make sure the ports are stored properly in the portRegistry
		gameServerNamesAndPorts.Range(func(key, value interface{}) bool {
			gameServerName := key.(string)
			port := value.(int32)
			Expect(portRegistry.HostPortsPerGameServer[getNamespacedName(testnamespace, gameServerName)]).To(Equal([]int32{port}))
			return true
		})

		// make sure all ports have been registered 4 times
		for k, v := range portRegistry.HostPortsUsage {
			Expect(v).To(Equal(4))
			validatePort(k, min, max)
		}

		// trying to get another port should fail, since we've allocated every available port
		_, err := portRegistry.GetNewPorts(testnamespace, "willfail", 1)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorNotEnoughFreePorts))

		// deallocate 1000 ports in parallel
		i := 0
		gameServerNamesAndPorts.Range(func(key, value interface{}) bool {
			ports, err := portRegistry.DeregisterPorts(testnamespace, key.(string))
			Expect(err).ToNot(HaveOccurred())
			Expect(len(ports)).To(Equal(1))
			i++
			gameServerNamesAndPorts.Delete(key)
			return i != 1000 // exit the loop when it has run 1000 times
		})

		Expect(len(portRegistry.HostPortsPerGameServer)).To(Equal(int(max-min+1) * 2)) // 1000 ports allocated

		// make sure the ports are stored properly in the portRegistry
		gameServerNamesAndPorts.Range(func(key, value interface{}) bool {
			gameServerName := key.(string)
			port := value.(int32)
			Expect(portRegistry.HostPortsPerGameServer[getNamespacedName(testnamespace, gameServerName)]).To(Equal([]int32{port}))
			return true
		})

		// allocate the rest 1000 ports in parallel
		for i := 0; i < int(max-min+1)*2; i++ {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				gameServerName := generateName(prefix)
				ports, err := portRegistry.GetNewPorts(testnamespace, gameServerName, 1)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(ports)).To(Equal(1))
				gameServerNamesAndPorts.Store(gameServerName, ports[0])
			}()
		}
		wg.Wait()

		Expect(len(portRegistry.HostPortsPerGameServer)).To(Equal(int(max-min+1) * 4)) // 2000 ports allocated

		// make sure the ports are stored properly in the portRegistry
		gameServerNamesAndPorts.Range(func(key, value interface{}) bool {
			gameServerName := key.(string)
			port := value.(int32)
			Expect(portRegistry.HostPortsPerGameServer[getNamespacedName(testnamespace, gameServerName)]).To(Equal([]int32{port}))
			return true
		})

		// trying to get another port should fail, since we've allocated every available port
		_, err = portRegistry.GetNewPorts(testnamespace, "willfailagain", 1)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorNotEnoughFreePorts))
	})
})

// validatePort checks if the port is in the range testMinPort<=port<=testMaxPort
func validatePort(port, testMinPort, testMaxPort int32) {
	Expect(port).Should(BeNumerically(">=", testMinPort))
	Expect(port).Should(BeNumerically("<=", testMaxPort))
}

// verifyExpectedHostPorts compares the HostPortsUsage map on the PortRegistry to the expectedHostPorts map
func verifyExpectedHostPorts(portRegistry *PortRegistry, expectedHostPorts map[int32]int, expectedTotalHostPortsCount int) {
	actualHostPorts := make(map[int32]int)
	actualTotalHostPortsCount := 0
	for port, count := range portRegistry.HostPortsUsage {
		actualHostPorts[port] = count
		actualTotalHostPortsCount += count
	}
	for port := portRegistry.Min; port <= portRegistry.Max; port++ {
		if _, ok := expectedHostPorts[port]; !ok {
			expectedHostPorts[port] = 0
		}
	}

	Expect(len(actualHostPorts)).To(Equal(len(expectedHostPorts)))
	Expect(reflect.DeepEqual(actualHostPorts, expectedHostPorts)).To(BeTrue())
	Expect(actualTotalHostPortsCount).To(Equal(expectedTotalHostPortsCount))
}

// verifyHostPortsUsage verifies that the HostPortsUsage map on the PortRegistry has the proper length
// and its item has the correct length as well
func verifyHostPortsUsage(portRegistry *PortRegistry, expectedNodeCount, expectedFreePortsCount int) error {
	if portRegistry.NodeCount != expectedNodeCount {
		return fmt.Errorf("NodeCount is not %d, it is %d", expectedNodeCount, portRegistry.NodeCount)
	}
	if portRegistry.FreePortsCount != expectedFreePortsCount {
		return fmt.Errorf("FreePortsCount is not %d, it is %d", expectedFreePortsCount, portRegistry.FreePortsCount)
	}
	return nil
}

// getPortRegistryKubeClientForTesting returns a PortRegistry and a fake Kubernetes client for testing
func getPortRegistryKubeClientForTesting(min, max int32) (*PortRegistry, client.Client) {
	log := logr.FromContextOrDiscard(context.Background())
	node := getNewNodeForTest("node1")
	clientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(node)
	kubeClient := clientBuilder.Build()
	Expect(kubeClient).NotTo(BeNil())
	portRegistry, err := NewPortRegistry(kubeClient, &mpsv1alpha1.GameServerList{}, min, max, 1, false, log)
	Expect(err).ToNot(HaveOccurred())
	return portRegistry, kubeClient
}

// syncMapToMapInt32Bool converts a sync.Map to a map[int32]int
// useful as part of our test uses the sync.Map instead of the slice
func syncMapToMapInt32Int(sm *sync.Map) map[int32]int {
	m := make(map[int32]int)
	sm.Range(func(key, value interface{}) bool {
		m[key.(int32)] = value.(int)
		return true
	})
	return m
}

// getNewNodeForTest returns a new Node for testing
func getNewNodeForTest(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
}
