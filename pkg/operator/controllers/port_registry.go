package controllers

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
)

// PortRegistry implements a custom map for the port registry
type PortRegistry struct {
	HostPorts         map[int32]bool
	Indexes           []int32
	NextFreePortIndex int32
	Min               int32         // Minimum Port
	Max               int32         // Maximum Port
	portRequests      chan struct{} // buffered channel to store port requests (request is the containerPort)
	portResponses     chan int32    // buffered channel to store port responses (system returns the HostPort)
}

// NewPortRegistry initializes the IndexedDictionary that holds the port registry.
func NewPortRegistry(gameServers mpsv1alpha1.GameServerList, min, max int32, setupLog logr.Logger) (*PortRegistry, error) {
	pr := &PortRegistry{
		HostPorts:     make(map[int32]bool, max-min+1),
		Indexes:       make([]int32, max-min+1),
		Min:           min,
		Max:           max,
		portRequests:  make(chan struct{}, 100),
		portResponses: make(chan int32, 100),
	}

	// gather ports for existing game servers
	if len(gameServers.Items) > 0 {
		for _, gs := range gameServers.Items {
			if len(gs.Spec.Template.Spec.Containers) == 0 {
				setupLog.Info("GameServer with name %s has no containers in its Pod Template: %#v", gs.Name, gs)
				continue
			}

			for _, container := range gs.Spec.Template.Spec.Containers {

				portsExposed := make([]int32, len(container.Ports))
				portsExposedIndex := 0

				for _, portInfo := range container.Ports {
					if portInfo.HostPort == 0 {
						setupLog.Info("HostPort for GameServer %s and ContainerPort %d is zero, ignoring", gs.Name, portInfo.ContainerPort)
						continue
					}
					portsExposed[portsExposedIndex] = portInfo.HostPort
					portsExposedIndex++
				}

				pr.assignRegisteredPorts(portsExposed)
			}

		}
	}

	pr.assignUnregisteredPorts()

	go pr.portProducer()

	return pr, nil

}

func (pr *PortRegistry) displayRegistry() {
	fmt.Printf("-------------------------------------\n")
	fmt.Printf("Ports: %v\n", pr.HostPorts)
	fmt.Printf("Indexes: %v\n", pr.Indexes)
	fmt.Printf("NextIndex: %d\n", pr.NextFreePortIndex)
	fmt.Printf("-------------------------------------\n")
}

// GetNewPort returns and registers a new port for the designated game server. Locks a mutex
func (pr *PortRegistry) GetNewPort() (int32, error) {
	pr.portRequests <- struct{}{}

	port := <-pr.portResponses

	if port == -1 {
		return -1, errors.New("cannot register a new port. No available ports")
	}

	return port, nil
}

func (pr *PortRegistry) portProducer() {
	for range pr.portRequests { //wait till a new request comes

		initialIndex := pr.NextFreePortIndex
		for {
			if !pr.HostPorts[pr.Indexes[pr.NextFreePortIndex]] {
				//we found a port
				port := pr.Indexes[pr.NextFreePortIndex]
				pr.HostPorts[port] = true

				pr.increaseNextFreePortIndex()

				//port is set
				pr.portResponses <- port
				break
			}

			pr.increaseNextFreePortIndex()

			if initialIndex == pr.NextFreePortIndex {
				//we did a full loop - no empty ports
				pr.portResponses <- -1
				break
			}
		}
	}
}

// Stop stops port registry mechanism by closing requests and responses channels
func (pr *PortRegistry) Stop() {
	close(pr.portRequests)
	close(pr.portResponses)
}

// DeregisterServerPorts deregisters all host ports so they can be re-used by additional game servers
func (pr *PortRegistry) DeregisterServerPorts(ports []int32) {
	for i := 0; i < len(ports); i++ {
		pr.HostPorts[ports[i]] = false
	}
}

func (pr *PortRegistry) assignRegisteredPorts(ports []int32) {
	for i := 0; i < len(ports); i++ {
		pr.HostPorts[ports[i]] = true
		pr.Indexes[i] = ports[i]
		pr.increaseNextFreePortIndex()
	}
}

func (pr *PortRegistry) assignUnregisteredPorts() {
	i := pr.NextFreePortIndex
	for _, port := range pr.getPorts() {
		if _, ok := pr.HostPorts[port]; !ok {
			pr.HostPorts[port] = false
			pr.Indexes[i] = port
			i++
		}
	}
}

func (pr *PortRegistry) increaseNextFreePortIndex() {
	pr.NextFreePortIndex++
	//reset the index if needed
	if pr.NextFreePortIndex == pr.Max-pr.Min+1 {
		pr.NextFreePortIndex = 0
	}

}

func (pr *PortRegistry) getPorts() []int32 {
	ports := make([]int32, pr.Max-pr.Min+1)
	for i := 0; i < len(ports); i++ {
		ports[i] = int32(pr.Min) + int32(i)
	}
	return ports
}
