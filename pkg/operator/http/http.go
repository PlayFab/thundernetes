package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
)

var (
	crtBytes []byte
	keyBytes []byte
)

const (
	listeningPort = 5000
)

// AllocationApiServer is a helper struct that implements manager.Runnable interface
// so it can be added to our Manager
type AllocationApiServer struct {
	client client.Client
	config *rest.Config
	scheme *runtime.Scheme
}

// NewAllocationApiServer creates a new AllocationApiServer and initializes the crt/key variables (can be nil)
func NewAllocationApiServer(mgr ctrl.Manager, crt, key []byte) error {
	crtBytes = crt
	keyBytes = key

	server := &AllocationApiServer{client: mgr.GetClient(), config: mgr.GetConfig(), scheme: mgr.GetScheme()}

	if err := server.setupIndexers(mgr); err != nil {
		return err
	}

	return mgr.Add(server)
}

func (s *AllocationApiServer) setupIndexers(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &mpsv1alpha1.GameServer{}, "status.state", func(rawObj client.Object) []string {
		gs := rawObj.(*mpsv1alpha1.GameServer)
		return []string{string(gs.Status.State)}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &mpsv1alpha1.GameServer{}, "status.sessionID", func(rawObj client.Object) []string {
		gs := rawObj.(*mpsv1alpha1.GameServer)
		return []string{gs.Status.SessionID}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &mpsv1alpha1.GameServerBuild{}, "spec.buildID", func(rawObj client.Object) []string {
		gsb := rawObj.(*mpsv1alpha1.GameServerBuild)
		return []string{gsb.Spec.BuildID}
	}); err != nil {
		return err
	}

	return nil
}

// NeedLeaderElection returns false since we need the allocation API service to run all on controller Pods
func (s *AllocationApiServer) NeedLeaderElection() bool {
	return false
}

// Start starts the HTTP(S) allocation API service
// if user has provided public/private cert details, it will create a TLS-auth HTTPS server
// otherwise it will create a HTTP server with no auth
func (s *AllocationApiServer) Start(ctx context.Context) error {
	log := log.FromContext(ctx)
	addr := os.Getenv("API_LISTEN")
	if addr == "" {
		addr = fmt.Sprintf(":%d", listeningPort)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/v1/allocate", &allocateHandler{
		client: s.client,
		config: s.config,
		scheme: s.scheme,
	})

	log.Info("serving allocation API service", "addr", addr, "port", listeningPort)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
	}

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		log.Info("shutting down allocation API service")

		// TODO: use a context with reasonable timeout
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout
			log.Error(err, "error shutting down the HTTP server")
		}
		close(done)
	}()

	if crtBytes != nil && keyBytes != nil {
		log.Info("starting TLS enabled allocation API service")
		// Generate a key pair from your pem-encoded cert and key ([]byte).
		cert, err := tls.X509KeyPair(crtBytes, keyBytes)
		if err != nil {
			return nil
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(crtBytes)
		// Construct a tls.config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    caCertPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			// Other options
		}

		// Build a server:
		srv.TLSConfig = tlsConfig
		// Finally: serve.
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			return err
		}
	} else {
		log.Info("starting insecure allocation API service")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
	}

	<-done
	return nil
}
