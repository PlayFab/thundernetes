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
	"log"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/caarlos0/env/v6"
	"github.com/go-logr/logr"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"github.com/playfab/thundernetes/pkg/operator/controllers"

	//+kubebuilder:scaffold:imports

	corev1 "k8s.io/api/core/v1"
)

// Config is a struct containing configuration from environment variables
// source: https://github.com/caarlos0/env
type Config struct {
	ApiServiceSecurity                     string `env:"API_SERVICE_SECURITY"`
	TlsSecretName                          string `env:"TLS_SECRET_NAME" envDefault:"tls-secret"`
	TlsSecretNamespace                     string `env:"TLS_SECRET_NAMESPACE" envDefault:"thundernetes-system"`
	TlsCertificateName                     string `env:"TLS_CERTIFICATE_FILENAME" envDefault:"tls.crt"`
	TlsPrivateKeyFilename                  string `env:"TLS_PRIVATE_KEY_FILENAME" envDefault:"tls.key"`
	PortRegistryExclusivelyGameServerNodes bool   `env:"PORT_REGISTRY_EXCLUSIVELY_GAME_SERVER_NODES" envDefault:"false"`
	LogLevel                               string `env:"LOG_LEVEL" envDefault:"info"`
	MinPort                                int32  `env:"MIN_PORT" envDefault:"10000"`
	MaxPort                                int32  `env:"MAX_PORT" envDefault:"12000"`
	ListeningPort                          int32  `env:"LISTENING_PORT" envDefault:"5000"`
	InitContainerImageLinux                string `env:"THUNDERNETES_INIT_CONTAINER_IMAGE,notEmpty"`
	InitContainerImageWin                  string `env:"THUNDERNETES_INIT_CONTAINER_IMAGE_WIN,notEmpty"`
}

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(mpsv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	// load configuration from env variables
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		log.Fatal(err, "Cannot load configuration from environment variables")
	}
	listeningPort := cfg.ListeningPort

	// load the rest of the configuration from command-line flags
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
		Level:       getLogLevel(cfg.LogLevel),
		// https://github.com/uber-go/zap/issues/661#issuecomment-520686037 and https://github.com/uber-go/zap/issues/485#issuecomment-834021392
		TimeEncoder: zapcore.TimeEncoderOfLayout(time.RFC3339),
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// setupLog is valid after this call
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("Loaded configuration from environment variables", "config", cfg)

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
	// initialize a live API client, used for the PortRegistry and fetching the mTLS secret
	k8sClient := mgr.GetAPIReader()
	// get public and private key, if enabled
	crt, key := getCrtKeyIfTlsEnabled(k8sClient, cfg)

	// initialize the allocation API service, which is also a controller. So we add it to the manager
	aas := controllers.NewAllocationApiServer(crt, key, mgr.GetClient(), int32(listeningPort))
	if err = aas.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create HTTP allocation API Server", "Allocation API Server", "HTTP Allocation API Server")
		os.Exit(1)
	}

	// initialize the portRegistry
	portRegistry, err := initializePortRegistry(k8sClient, mgr.GetClient(), setupLog, cfg)
	if err != nil {
		setupLog.Error(err, "unable to initialize portRegistry")
		os.Exit(1)
	}
	// portRegistry is a controller so we add it to the manager (so it can reconcile the Nodes)
	if err := portRegistry.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PortRegistry")
		os.Exit(1)
	}

	// initialize the GameServer controller
	if err = controllers.NewGameServerReconciler(mgr, portRegistry, controllers.GetNodeDetails, cfg.InitContainerImageLinux, cfg.InitContainerImageWin).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GameServer")
		os.Exit(1)
	}

	// initialize the GameServerBuild controller
	if err = controllers.NewGameServerBuildReconciler(mgr, portRegistry).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GameServerBuild")
		os.Exit(1)
	}

	// initialize webhook for GameServerBuild validation
	if err = (&mpsv1alpha1.GameServerBuild{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "GameServerBuild")
		os.Exit(1)
	}
	// initialize webhook for GameServer validation
	if err = (&mpsv1alpha1.GameServer{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "GameServer")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

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
func initializePortRegistry(k8sClient client.Reader, crClient client.Client, setupLog logr.Logger, cfg *Config) (*controllers.PortRegistry, error) {
	var gameServers mpsv1alpha1.GameServerList
	if err := k8sClient.List(context.Background(), &gameServers); err != nil {
		return nil, err
	}

	useExclusivelyGameServerNodesForPortRegistry := cfg.PortRegistryExclusivelyGameServerNodes

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
	minPort, maxPort, err := validateMinMaxPort(cfg)
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
func getTlsSecret(k8sClient client.Reader, cfg *Config) ([]byte, []byte, error) {
	var secret corev1.Secret
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      cfg.TlsSecretName,
		Namespace: cfg.TlsSecretNamespace,
	}, &secret)
	if err != nil {
		return nil, nil, err
	}
	return []byte(secret.Data[cfg.TlsCertificateName]), []byte(secret.Data[cfg.TlsPrivateKeyFilename]), nil
}

// validateMinMaxPort validates minimum and maximum ports
func validateMinMaxPort(cfg *Config) (int32, int32, error) {
	if cfg.MinPort >= cfg.MaxPort {
		return 0, 0, errors.New("MIN_PORT cannot be greater or equal than MAX_PORT")
	}

	return cfg.MinPort, cfg.MaxPort, nil
}

// getLogLevel returns the log level based on the LOG_LEVEL environment variable
func getLogLevel(logLevel string) zapcore.LevelEnabler {
	switch logLevel {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	case "panic":
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}

// getCrtKeyIfTlsEnabled returns public and private key components for securing the allocation API service with mTLS
// for this to happen, user has to set "API_SERVICE_SECURITY" env as "usetls" and set the env "TLS_SECRET_NAMESPACE" with the namespace
// that contains the Kubernetes Secret with the cert
// if any of the mentioned conditions are not set, method returns nil
func getCrtKeyIfTlsEnabled(c client.Reader, cfg *Config) ([]byte, []byte) {
	if cfg.ApiServiceSecurity == "usetls" {
		crt, key, err := getTlsSecret(c, cfg)
		if err != nil {
			setupLog.Error(err, "unable to get TLS secret")
			os.Exit(1)
		}
		return crt, key
	}
	return nil, nil
}
