package controllers

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PortRegistry implements a custom map for the port registry
type PortRegistry struct {
	client                            client.Client // used to get the list of nodes
	NodeCount                         int           // the number of Ready and Schedulable nodes in the cluster
	HostPortsPerNode                  map[int32]int // a slice for the entire port range. increases by 1 for each registered port
	Min                               int32         // Minimum Port
	Max                               int32         // Maximum Port
	FreePortsCount                    int           // the number of free ports
	lockMutex                         sync.Mutex    // lock for the map
	useSpecificNodePoolForGameServers bool          // if true, we only take into account Nodes that have the Label "mps.playfab.com/gameservernode"=true
	nextPortNumber                    int32         // the next port to be assigned
	logger                            logr.Logger
}

// NewPortRegistry initializes the map[port]counter that holds the port registry
// The way that this works is the following:
// We keep a map (HostPortsPerNode) of all the port numbers
// every time a new port is requested, we check if the counter for this port is less than the number of Nodes
// if it is, we increase it by one. If not, we check the next port.
// the nextPortNumber facilitates getting the next port (port+1),
// since getting the same port again would cause the GameServer Pod to be placed on a different Node, to avoid collision.
// This would have a negative impact in cases where we want as many GameServers as possible on the same Node.
// We also set up a Kubernetes Watch for the Nodes
// When a new Node is added or removed to the cluster, we modify the NodeCount variable
func NewPortRegistry(client client.Client, gameServers *mpsv1alpha1.GameServerList, min, max int32, nodeCount int, useSpecificNodePool bool, setupLog logr.Logger) (*PortRegistry, error) {
	if min > max {
		return nil, errors.New("min port cannot be greater than max port")
	}

	pr := &PortRegistry{
		client:                            client,
		Min:                               min,
		Max:                               max,
		NodeCount:                         nodeCount,
		FreePortsCount:                    nodeCount * int(max-min+1), // +1 since the [min,max] ports set is inclusive of both edges
		lockMutex:                         sync.Mutex{},
		useSpecificNodePoolForGameServers: useSpecificNodePool,
		nextPortNumber:                    min,
		logger:                            log.Log.WithName("portregistry"),
	}

	// initialize the ports
	pr.HostPortsPerNode = make(map[int32]int)
	for port := pr.Min; port <= pr.Max; port++ {
		pr.HostPortsPerNode[port] = 0
	}

	// gather ports for existing game servers
	if len(gameServers.Items) > 0 {
		for _, gs := range gameServers.Items {
			if len(gs.Spec.Template.Spec.Containers) == 0 {
				setupLog.Info("GameServer has no containers in the Pod Template", "GameServer", gs.Name)
				continue
			}

			for _, container := range gs.Spec.Template.Spec.Containers {
				portsExposed := make([]int32, len(container.Ports))
				portsExposedIndex := 0

				for _, portInfo := range container.Ports {
					if portInfo.HostPort == 0 {
						setupLog.Info("HostPort for GameServer and ContainerPort is zero, ignoring", "GameServerName", gs.Name, "ContainerPort", portInfo.ContainerPort)
						continue
					}
					portsExposed[portsExposedIndex] = portInfo.HostPort
					portsExposedIndex++
				}
				// and register them
				pr.assignRegisteredPorts(portsExposed)
			}
		}
	}
	return pr, nil
}

// Reconcile runs when a Node is created/deleted or the node status changes
func (pr *PortRegistry) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var nodeList v1.NodeList
	var err error

	// if we have a specific node pool/group for game servers (with mps.playfab.com/gameservernode=true Label)
	if pr.useSpecificNodePoolForGameServers {
		err = pr.client.List(ctx, &nodeList, client.MatchingLabels{LabelGameServerNode: "true"})
	} else { // get all the Nodes
		err = pr.client.List(ctx, &nodeList)
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	// calculate how many nodes are ready and schedulable
	schedulableNodesCount := 0
	for i := 0; i < len(nodeList.Items); i++ {
		if IsNodeReadyAndSchedulable(&nodeList.Items[i]) {
			schedulableNodesCount++
		}
	}
	log.Info("Reconciling Nodes", "schedulableNodesCount", schedulableNodesCount, "currentNodesCount", pr.NodeCount)

	// most probably it will just be a single node added or removed, but just in case
	if pr.NodeCount > schedulableNodesCount {
		for i := pr.NodeCount - 1; i >= schedulableNodesCount; i-- {
			log.Info("Node was removed")
			pr.onNodeRemoved()
		}
	} else if pr.NodeCount < schedulableNodesCount {
		for i := pr.NodeCount; i < schedulableNodesCount; i++ {
			log.Info("Node was added")
			pr.onNodeAdded()
		}
	}

	return ctrl.Result{}, nil
}

// onNodeAdded is called when a Node is added to the cluster
func (pr *PortRegistry) onNodeAdded() {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	pr.NodeCount++
	pr.FreePortsCount += int(pr.Max - pr.Min + 1)
}

// onNodeRemoved is called when a Node is removed from the cluster
// it removes one set of port ranges from the map
// we don't need to know which one was removed, we just need to move around the registered (set to true) ports
func (pr *PortRegistry) onNodeRemoved() {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	pr.NodeCount--
	pr.FreePortsCount -= int(pr.Max - pr.Min + 1)
}

// GetNewPorts returns and registers a slice of ports with "count" length that will be used by a GameServer
// It returns an error if there are no available ports
// You may wonder what happens if two GameServer Pods get assigned the same HostPort
// We will not have a collision, since Kubernetes is pretty smart and will place the Pod on a different Node, to prevent it
func (pr *PortRegistry) GetNewPorts(count int) ([]int32, error) {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	if count > pr.FreePortsCount {
		return nil, errors.New("not enough free ports")
	}
	portsToReturn := make([]int32, count)
	for i := 0; i < count; i++ {
		portFound := false
		// get the next port
		for port := pr.nextPortNumber; port <= pr.Max; port++ {
			// this port is used less times than the total number of Nodes
			if pr.HostPortsPerNode[port] < pr.NodeCount {
				pr.HostPortsPerNode[port]++  // increase the times (Nodes) this port is used
				pr.nextPortNumber = port + 1 // move the next port to the next one
				// we did a full circle on the map
				if pr.nextPortNumber > pr.Max {
					pr.nextPortNumber = pr.Min
				}
				pr.FreePortsCount--     // decrease the number of used ports
				portsToReturn[i] = port // add the port to the slice to be returned
				portFound = true
				break // exit the loop
			}
		}
		if !portFound {
			// we made a full circle, no available ports
			return nil, errors.New("cannot register a new port. No available ports")
		}
	}
	return portsToReturn, nil
}

// DeregisterServerPorts deregisters all host ports so they can be re-used by additional game servers
func (pr *PortRegistry) DeregisterServerPorts(ports []int32, gsName string) error {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	for i := 0; i < len(ports); i++ {
		if pr.HostPortsPerNode[ports[i]] > 0 {
			// following log should NOT be changed since an e2e test depends on it
			pr.logger.V(1).Info("Deregistering port", "port", ports[i], "GameServer", gsName)
			pr.HostPortsPerNode[ports[i]]--
			pr.FreePortsCount++
		} else {
			return fmt.Errorf("cannot deregister port %d, it is not registered or has already been deleted", ports[i])
		}
	}
	return nil
}

// assignRegisteredPorts assigns ports that are already registered
// used for existing game servers and when the controller is updated/crashed and started again
func (pr *PortRegistry) assignRegisteredPorts(ports []int32) {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	for i := 0; i < len(ports); i++ {
		pr.logger.V(1).Info("Registering port", "port", ports[i])
		pr.HostPortsPerNode[ports[i]]++
		pr.FreePortsCount--
	}
}

// SetupWithManager registers the PortRegistry controller with the manager
// we care to watch for changes in the Node objects, only if they are "Ready" and "Schedulable"
func (pr *PortRegistry) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Node{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				node := e.Object.(*v1.Node)
				if pr.useSpecificNodePoolForGameServers && !isNodeGameServerNode(node) {
					return false
				}
				return IsNodeReadyAndSchedulable(node)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				node := e.Object.(*v1.Node)
				// ignore this Node if we have a specific node pool for game servers (with mps.playfab.com/gameservernode=true Label)
				// and the current Node does not have this Label
				if pr.useSpecificNodePoolForGameServers && !isNodeGameServerNode(node) {
					return false
				}
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldNode := e.ObjectOld.(*v1.Node)
				newNode := e.ObjectNew.(*v1.Node)
				if pr.useSpecificNodePoolForGameServers && !isNodeGameServerNode(newNode) {
					return false
				}
				return IsNodeReadyAndSchedulable(oldNode) != IsNodeReadyAndSchedulable(newNode)
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		}).
		Complete(pr)
}
