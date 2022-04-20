/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"strconv"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/go-logr/logr"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"github.com/playfab/thundernetes/pkg/operator/controllers"

	//+kubebuilder:scaffold:imports

	corev1 "k8s.io/api/core/v1"

	"github.com/playfab/thundernetes/pkg/operator/http"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	secretName          = "tls-secret"
	certificateFileName = "tls.crt"
	privateKeyFileName  = "tls.key"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(mpsv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "25951049.playfab.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// initialize a live API client, used for the PortRegistry
	k8sClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to start live API client")
		os.Exit(1)
	}

	var crt, key []byte
	apiServiceSecurity := os.Getenv("API_SERVICE_SECURITY")

	if apiServiceSecurity == "usetls" {
		namespace := os.Getenv("TLS_SECRET_NAMESPACE")
		if namespace == "" {
			setupLog.Error(err, "unable to get TLS_SECRET_NAMESPACE env variable")
			os.Exit(1)
		}
		crt, key, err = getTlsSecret(k8sClient, namespace)
		if err != nil {
			setupLog.Error(err, "unable to get TLS secret")
			os.Exit(1)
		}
	}

	portRegistry, err := initializePortRegistry(k8sClient, mgr.GetClient(), setupLog)
	if err != nil {
		setupLog.Error(err, "unable to initialize portRegistry")
		os.Exit(1)
	}
	// add the portRegistry to the manager so it can reconcile the Nodes
	if err := portRegistry.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PortRegistry")
		os.Exit(1)
	}

	if err = (&controllers.GameServerReconciler{
		Client:                     mgr.GetClient(),
		Scheme:                     mgr.GetScheme(),
		PortRegistry:               portRegistry,
		Recorder:                   mgr.GetEventRecorderFor("GameServer"),
		GetPublicIpForNodeProvider: controllers.GetPublicIPForNode,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GameServer")
		os.Exit(1)
	}
	if err = (&controllers.GameServerBuildReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		PortRegistry: portRegistry,
		Recorder:     mgr.GetEventRecorderFor("GameServerBuild"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GameServerBuild")
		os.Exit(1)
	}

	if err = (&mpsv1alpha1.GameServerBuild{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "GameServerBuild")
		os.Exit(1)
	}
	if err = (&mpsv1alpha1.GameServer{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "GameServer")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	// initialize the allocation API service
	err = http.NewAllocationApiServer(mgr, crt, key)
	if err != nil {
		setupLog.Error(err, "unable to create HTTP allocation API Server", "Allocation API Server", "HTTP Allocation API Server")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// initializePortRegistry performs some initialization and creates a new PortRegistry struct
// the k8sClient is a live API client and is used to get the existing gameservers and the "Ready" Nodes
// the crClient is the cached controller-runtime client, used to watch for changes to the nodes from inside the PortRegistry
func initializePortRegistry(k8sClient client.Client, crClient client.Client, setupLog logr.Logger) (*controllers.PortRegistry, error) {
	var gameServers mpsv1alpha1.GameServerList
	if err := k8sClient.List(context.Background(), &gameServers); err != nil {
		return nil, err
	}

	useExclusivelyGameServerNodesForPortRegistry := useExclusivelyGameServerNodesForPortRegistry()

	var nodes corev1.NodeList
	if err := k8sClient.List(context.Background(), &nodes); err != nil {
		return nil, err
	}

	schedulableAndReadyNodeCount := 0
	for i := 0; i < len(nodes.Items); i++ {
		if controllers.IsNodeReadyAndSchedulable(&nodes.Items[i]) {
			if useExclusivelyGameServerNodesForPortRegistry && nodes.Items[i].Labels[controllers.LabelGameServerNode] == "true" {
				schedulableAndReadyNodeCount = schedulableAndReadyNodeCount + 1
			}
		}
	}

	// get the min/max port from enviroment variables
	// the code does not offer any protection in case the port range changes while game servers are running
	minPort, maxPort, err := getMinMaxPortFromEnv()
	if err != nil {
		return nil, err
	}

	setupLog.Info("initializing port registry", "minPort", minPort, "maxPort", maxPort, "schedulableAndReadyNodeCount", schedulableAndReadyNodeCount)

	portRegistry, err := controllers.NewPortRegistry(crClient, &gameServers, minPort, maxPort, schedulableAndReadyNodeCount, useExclusivelyGameServerNodesForPortRegistry, setupLog)
	if err != nil {
		return nil, err
	}

	return portRegistry, nil
}

// getTlsSecret returns the TLS secret from the given namespace
// used in the allocation API service
func getTlsSecret(k8sClient client.Client, namespace string) ([]byte, []byte, error) {
	var secret corev1.Secret
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      secretName,
		Namespace: namespace,
	}, &secret)
	if err != nil {
		return nil, nil, err
	}
	return []byte(secret.Data[certificateFileName]), []byte(secret.Data[privateKeyFileName]), nil
}

// getMinMaxPortFromEnv returns minimum and maximum port from environment variables
func getMinMaxPortFromEnv() (int32, int32, error) {
	minPortStr := os.Getenv("MIN_PORT")
	maxPortStr := os.Getenv("MAX_PORT")

	// if both of them are not set, return default values
	if minPortStr == "" && maxPortStr == "" {
		setupLog.Info("MIN_PORT and MAX_PORT environment variables are not set. Using default values 10000 and 12000.")
		return 10000, 12000, nil
	}

	if minPortStr == "" {
		// this means that MAX_PORT is set, but not MIN_PORT
		return 0, 0, errors.New("MIN_PORT env variable is not set")
	}
	// we use ParseInt insteaf of Atoi because CodeQL triggered this https://codeql.github.com/codeql-query-help/go/go-incorrect-integer-conversion/
	minPortParsed, err := strconv.ParseInt(minPortStr, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	if maxPortStr == "" {
		// this means that MIN_PORT is set, but not MAX_PORT
		return 0, 0, errors.New("MAX_PORT env variable is not set")
	}
	maxPortParsed, err := strconv.ParseInt(maxPortStr, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	minPort := int32(minPortParsed)
	maxPort := int32(maxPortParsed)

	if minPort >= maxPort {
		return 0, 0, errors.New("MIN_PORT cannot be greater or equal than MAX_PORT")
	}

	return minPort, maxPort, nil
}

func useExclusivelyGameServerNodesForPortRegistry() bool {
	return os.Getenv("PORT_REGISTRY_EXCLUSIVELY_GAMESERVER_NODES") == "true"
}
