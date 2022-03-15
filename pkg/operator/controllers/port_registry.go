package controllers

import (
	"context"
	"errors"
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
	client                                       client.Client    // used to get the list of nodes
	NodeCount                                    int              // the number of Ready and Schedulable nodes in the cluster
	HostPortsPerNode                             []map[int32]bool // a map of all the ports per node. false if available, true if registered
	Min                                          int32            // Minimum Port
	Max                                          int32            // Maximum Port
	lockMutex                                    sync.Mutex       // lock for the map
	useExclusivelyGameServerNodesForPortRegistry bool
}

// NewPortRegistry initializes the IndexedDictionary that holds the port registry
// The way that this works is the following:
// We keep a map (HostPortsPerNode) of all the port numbers and registered status (bool) for every Node
// every time a new port is requested, we check if there is an available port on any of the nodes
// if there is, we set it to true
// We also set up a Kubernetes Watch for the Nodes
// When a new Node is added to the cluster, we add a new set of ports to the map (size = Max-Min+1)
// When a Node is removed, we have to delete the port range for this Node from the map
func NewPortRegistry(client client.Client, gameServers *mpsv1alpha1.GameServerList, min, max int32, nodeCount int, useExclusivelyGameServerNodesForPortRegistry bool, setupLog logr.Logger) (*PortRegistry, error) {
	if min > max {
		return nil, errors.New("min port cannot be greater than max port")
	}

	pr := &PortRegistry{
		client:    client,
		Min:       min,
		Max:       max,
		lockMutex: sync.Mutex{},
		useExclusivelyGameServerNodesForPortRegistry: useExclusivelyGameServerNodesForPortRegistry,
	}

	// add the necessary set of ports to the map
	for i := 0; i < nodeCount; i++ {
		pr.onNodeAdded()
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
	if pr.useExclusivelyGameServerNodesForPortRegistry {
		err = pr.client.List(ctx, &nodeList, client.MatchingLabels{LabelGameServerNode: "true"})
	} else {
		err = pr.client.List(ctx, &nodeList)
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	// calculate how many nodes are ready and schedulable
	schedulableNodesCount := 0
	for i := 0; i < len(nodeList.Items); i++ {
		if !nodeList.Items[i].Spec.Unschedulable {
			schedulableNodesCount++
		}
	}
	log.Info("Reconciling Nodes", "schedulableNodesCount", schedulableNodesCount, "currentNodesCount", pr.NodeCount)
	// most probably it will just be a single node added or removed, but just in case
	if pr.NodeCount > schedulableNodesCount {
		for i := pr.NodeCount - 1; i >= schedulableNodesCount; i-- {
			log.Info("Removing Node")
			pr.onNodeRemoved()
		}
	} else if pr.NodeCount < schedulableNodesCount {
		for i := pr.NodeCount; i < schedulableNodesCount; i++ {
			log.Info("Adding Node")
			pr.onNodeAdded()
		}
	}

	return ctrl.Result{}, nil
}

// onNodeAdded is called when a Node is added to the cluster
func (pr *PortRegistry) onNodeAdded() {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	// create a new port range for the node
	hostPorts := make(map[int32]bool, pr.Max-pr.Min+1)
	for i := pr.Min; i <= pr.Max; i++ {
		hostPorts[i] = false
	}
	// add it to the map
	pr.HostPortsPerNode = append(pr.HostPortsPerNode, hostPorts)
	pr.NodeCount = pr.NodeCount + 1
}

// onNodeRemoved is called when a Node is removed from the cluster
// it removes one set of port ranges from the map
// we don't need to know which one was removed, we just need to move around the registered (set to true) ports
func (pr *PortRegistry) onNodeRemoved() {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	// we're removing the last Node port set
	indexToRemove := len(pr.HostPortsPerNode) - 1
	for port := pr.Min; port <= pr.Max; port++ {
		if pr.HostPortsPerNode[indexToRemove][port] {
			// find a new place (node) for this registered port
			for i := 0; i < len(pr.HostPortsPerNode)-1; i++ {
				if !pr.HostPortsPerNode[i][port] {
					pr.HostPortsPerNode[i][port] = true
					break
				}
			}
		}
	}
	// removes the last item from the slice
	pr.HostPortsPerNode = pr.HostPortsPerNode[:len(pr.HostPortsPerNode)-1]
	pr.NodeCount = pr.NodeCount - 1
}

// GetNewPort returns and registers a new port for the designated game server
// One may wonder what happens if two GameServer Pods get assigned the same HostPort
// The answer is that we will not have a collision, since Kubernetes is pretty smart and will place the Pod on a different Node, to prevent collision
func (pr *PortRegistry) GetNewPort() (int32, error) {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	// loops through all the Node maps, returns the first available port
	// we expect game servers to go up and down all the time, so the
	// location of the first available port is non-deterministic
	for nodeIndex := 0; nodeIndex < int(pr.NodeCount); nodeIndex++ {
		for port := pr.Min; port <= pr.Max; port++ {
			if !pr.HostPortsPerNode[nodeIndex][port] {
				pr.HostPortsPerNode[nodeIndex][port] = true
				return port, nil
			}
		}
	}
	return -1, errors.New("cannot register a new port. No available ports")
}

// DeregisterServerPorts deregisters all host ports so they can be re-used by additional game servers
func (pr *PortRegistry) DeregisterServerPorts(ports []int32) {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	for i := 0; i < len(ports); i++ {
		for nodeIndex := 0; nodeIndex < pr.NodeCount; nodeIndex++ {
			if pr.HostPortsPerNode[nodeIndex][ports[i]] {
				// setting the port to false means it can be re-used
				pr.HostPortsPerNode[nodeIndex][ports[i]] = false
				break
			}
		}
	}
}

// assignRegisteredPorts assigns ports that are already registered
// used for existing game servers and when the controller is updated/crashed and started again
func (pr *PortRegistry) assignRegisteredPorts(ports []int32) {
	defer pr.lockMutex.Unlock()
	pr.lockMutex.Lock()
	for i := 0; i < len(ports); i++ {
		for nodeIndex := 0; nodeIndex < pr.NodeCount; nodeIndex++ {
			if pr.HostPortsPerNode[nodeIndex][ports[i]] {
				// setting the port to true means it's registered
				pr.HostPortsPerNode[nodeIndex][ports[i]] = true
				break
			}
		}
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
				if shouldUseExclusivelyGameServersAndNodeIsNotGameServerNode(pr.useExclusivelyGameServerNodesForPortRegistry, node) {
					return false
				}
				return IsNodeReadyAndSchedulable(node)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				node := e.Object.(*v1.Node)
				if shouldUseExclusivelyGameServersAndNodeIsNotGameServerNode(pr.useExclusivelyGameServerNodesForPortRegistry, node) {
					return false
				}
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldNode := e.ObjectOld.(*v1.Node)
				newNode := e.ObjectNew.(*v1.Node)
				if shouldUseExclusivelyGameServersAndNodeIsNotGameServerNode(pr.useExclusivelyGameServerNodesForPortRegistry, newNode) {
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
