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
	timeout                                   = time.Second * 10
	duration                                  = time.Second * 10
	interval                                  = time.Millisecond * 250
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
		ports, err := portRegistry.GetNewPorts(4)
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
		ports, err := portRegistry.GetNewPorts(testMaxPort - testMinPort + 1)
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
		_, err = portRegistry.GetNewPorts(1)
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
			return verifyHostPortsPerNode(portRegistry, 2)
		}).Should(Succeed())

		err = kubeClient.Delete(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
		Eventually(func() error {
			return verifyHostPortsPerNode(portRegistry, 1)
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
			return verifyHostPortsPerNode(portRegistry, 2)
		}).Should(Succeed())

		assignedPorts := make(map[int32]int)
		// get 15 ports
		ports, err := portRegistry.GetNewPorts(15)
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
		// get 10 ports
		ports, err := portRegistry.GetNewPorts(10)
		Expect(err).To(Not(HaveOccurred()))
		for i := 0; i < 10; i++ {
			port := ports[i]
			Expect(err).ToNot(HaveOccurred())
			validatePort(port, testMinPort, testMaxPort)
			if _, ok := assignedPorts[port]; ok {
				Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
			}
			assignedPorts[port] = assignedPorts[port] + 1
		}

		verifyExpectedHostPorts(portRegistry, assignedPorts, 10)
		// deallocate two ports
		err = portRegistry.DeregisterServerPorts([]int32{testMinPort + 1, testMinPort + 3}, testGsName)
		Expect(err).ToNot(HaveOccurred())
		delete(assignedPorts, testMinPort+1)
		delete(assignedPorts, testMinPort+3)
		verifyExpectedHostPorts(portRegistry, assignedPorts, 8)
	})

	It("should successfully deallocate ports - two Nodes to one scenario", func() {
		portRegistry, kubeClient := getPortRegistryKubeClientForTesting(testMinPort, testMaxPort)
		Expect(portRegistry.Min).To(Equal(int32(testMinPort)))
		Expect(portRegistry.Max).To(Equal(int32(testMaxPort)))
		assignedPorts := make(map[int32]int)
		// get 10 ports
		ports, err := portRegistry.GetNewPorts(10)
		Expect(err).To(Not(HaveOccurred()))
		for i := 0; i < 10; i++ {
			port := ports[i]
			Expect(err).ToNot(HaveOccurred())
			validatePort(port, testMinPort, testMaxPort)
			if _, ok := assignedPorts[port]; ok {
				Fail(fmt.Sprintf("Port %d should not be in the assignedPorts map", port))
			}
			assignedPorts[port] = assignedPorts[port] + 1
		}
		verifyExpectedHostPorts(portRegistry, assignedPorts, 10)
		// deallocate two ports
		err = portRegistry.DeregisterServerPorts([]int32{testMinPort + 1, testMinPort + 3}, testGsName)
		Expect(err).ToNot(HaveOccurred())
		delete(assignedPorts, testMinPort+1)
		delete(assignedPorts, testMinPort+3)
		verifyExpectedHostPorts(portRegistry, assignedPorts, 8)

		// add a second Node
		node := getNewNodeForTest("node2")
		err = kubeClient.Create(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		// do a manual reconcile since we haven't added the controller to the manager
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
		Eventually(func() error {
			return verifyHostPortsPerNode(portRegistry, 2)
		}).Should(Succeed())

		// get 8 ports, we have 16 in total
		ports, err = portRegistry.GetNewPorts(8)
		Expect(err).To(Not(HaveOccurred()))
		for i := 0; i < 8; i++ {
			port := ports[i]
			validatePort(port, testMinPort, testMaxPort)
			assignedPorts[port] = assignedPorts[port] + 1
		}
		verifyExpectedHostPorts(portRegistry, assignedPorts, 16)

		// deallocate six ports that exist on the second Node
		deletedPortsCount := 0
		for port := portRegistry.Min; port <= portRegistry.Max; port++ {
			if portRegistry.HostPortsPerNode[port] == 2 {
				err := portRegistry.DeregisterServerPorts([]int32{port}, testGsName)
				assignedPorts[port] = assignedPorts[port] - 1
				Expect(err).ToNot(HaveOccurred())
				deletedPortsCount++
			}
		}
		Expect(deletedPortsCount).To(Equal(6))
		verifyExpectedHostPorts(portRegistry, assignedPorts, 10)

		// now delete the second node
		err = kubeClient.Delete(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
		Eventually(func() error {
			return verifyHostPortsPerNode(portRegistry, 1)
		}).Should(Succeed())
		verifyExpectedHostPorts(portRegistry, assignedPorts, 10)

		// deallocate three ports
		err = portRegistry.DeregisterServerPorts([]int32{testMinPort + 1, testMinPort + 2, testMinPort + 3}, testGsName)
		Expect(err).ToNot(HaveOccurred())
		delete(assignedPorts, testMinPort+1)
		delete(assignedPorts, testMinPort+2)
		delete(assignedPorts, testMinPort+3)
		verifyExpectedHostPorts(portRegistry, assignedPorts, 7)
	})

})

var _ = Describe("Port registry with two thousand ports, five hundred on four nodes", func() {
	rand.Seed(time.Now().UnixNano())
	min := int32(20000)
	max := int32(20499)

	portRegistry, kubeClient := getPortRegistryKubeClientForTesting(min, max)
	Expect(portRegistry.Min).To(Equal(min))
	Expect(portRegistry.Max).To(Equal(max))
	assignedPorts := sync.Map{}

	// add three nodes
	for i := 0; i < 3; i++ {
		node := getNewNodeForTest(fmt.Sprintf("node%d", i+2))
		err := kubeClient.Create(context.Background(), node)
		Expect(err).ToNot(HaveOccurred())
		portRegistry.Reconcile(context.Background(), reconcile.Request{})
	}

	Eventually(func() error {
		return verifyHostPortsPerNode(portRegistry, 4)
	}).Should(Succeed())

	It("should work with allocating and deallocating ports", func() {
		// allocate all 2000 ports, one at a time
		var wg sync.WaitGroup
		for i := 0; i < int(max-min+1)*4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				ports, err := portRegistry.GetNewPorts(1)
				Expect(err).ToNot(HaveOccurred())
				val, ok := assignedPorts.Load(ports[0])
				if !ok {
					assignedPorts.Store(ports[0], 1)
				} else {
					assignedPorts.Store(ports[0], val.(int)+1)
				}
			}()
		}
		wg.Wait()
		m := syncMapToMapInt32Int(&assignedPorts)
		verifyExpectedHostPorts(portRegistry, m, 2000)

		// trying to get another port should fail, since we've allocated every available port
		_, err := portRegistry.GetNewPorts(1)
		Expect(err).To(HaveOccurred())

		//deallocate 1000 ports
		for i := 0; i < int(max-min+1)*2; i++ {
			wg.Add(1)
			go func(portToDeallocate int32) {
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				err := portRegistry.DeregisterServerPorts([]int32{portToDeallocate}, testGsName)
				Expect(err).ToNot(HaveOccurred())
				val, ok := assignedPorts.Load(portToDeallocate)
				if !ok {
					Fail(fmt.Sprintf("port %d was not found in the map", portToDeallocate))
				}
				assignedPorts.Store(portToDeallocate, val.(int)-1)
			}(int32((i / 2) + int(min))) // , this outputs 20000 2 times, 20001 2 times, 20002 2 times etc.
		}
		wg.Wait()

		m = syncMapToMapInt32Int(&assignedPorts)
		verifyExpectedHostPorts(portRegistry, m, 1000)

		// allocate 500 ports
		for i := 0; i < int(max-min+1); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				ports, err := portRegistry.GetNewPorts(1)
				Expect(err).ToNot(HaveOccurred())
				port := ports[0]
				val, ok := assignedPorts.Load(port)
				if !ok {
					assignedPorts.Store(port, 1)
				} else {
					assignedPorts.Store(port, val.(int)+1)
				}
			}()
		}
		wg.Wait()

		m = syncMapToMapInt32Int(&assignedPorts)
		verifyExpectedHostPorts(portRegistry, m, 1500)

		// allocate another 500 ports
		for i := 0; i < int(max-min+1); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				n := rand.Intn(200) + 50 // n will be between 50 and 250
				time.Sleep(time.Duration(n) * time.Millisecond)
				ports, err := portRegistry.GetNewPorts(1)
				Expect(err).ToNot(HaveOccurred())
				port := ports[0]
				val, ok := assignedPorts.Load(port)
				if !ok {
					assignedPorts.Store(port, 1)
				} else {
					assignedPorts.Store(port, val.(int)+1)
				}
			}()
		}
		wg.Wait()

		m = syncMapToMapInt32Int(&assignedPorts)
		verifyExpectedHostPorts(portRegistry, m, 2000)

		// trying to get another port should fail, since we've allocated every available port
		_, err = portRegistry.GetNewPorts(1)
		Expect(err).To(HaveOccurred())
	})
})

// validatePort checks if the port is in the range testMinPort<=port<=testMaxPort
func validatePort(port, testMinPort, testMaxPort int32) {
	Expect(port).Should(BeNumerically(">=", testMinPort))
	Expect(port).Should(BeNumerically("<=", testMaxPort))
}

// verifyExpectedHostPorts compares the hostPortsPerNode map on the PortRegistry to the expectedHostPorts map
func verifyExpectedHostPorts(portRegistry *PortRegistry, expectedHostPorts map[int32]int, expectedTotalHostPortsCount int) {
	actualHostPorts := make(map[int32]int)
	actualTotalHostPortsCount := 0
	for port, count := range portRegistry.HostPortsPerNode {
		if count > 0 {
			actualHostPorts[port] = count
			actualTotalHostPortsCount += count
		}
	}
	Expect(reflect.DeepEqual(actualHostPorts, expectedHostPorts)).To(BeTrue())
	Expect(actualTotalHostPortsCount).To(Equal(expectedTotalHostPortsCount))
}

// verifyHostPortsPerNode verifies that the hostPortsPerNode map on the PortRegistry has the proper length
// and its item has the correct length as well
func verifyHostPortsPerNode(portRegistry *PortRegistry, expectedNodeCount int) error {
	if portRegistry.NodeCount != expectedNodeCount {
		return fmt.Errorf("NodeCount is not %d, it is %d", expectedNodeCount, portRegistry.NodeCount)
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
