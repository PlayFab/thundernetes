package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
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
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
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
		if err := customListenAndServeTLS(srv, crtBytes, keyBytes); err != nil && err != http.ErrServerClosed {
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

// customListenAndServeTLS creates a new http server with []byte cert and []byte key
// Golang's ListenAndServerTLS accepts filenames for cert and key whereas we have []byte
// https://stackoverflow.com/a/30818656
func customListenAndServeTLS(srv *http.Server, certPEMBlock, keyPEMBlock []byte) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}
	config := &tls.Config{}
	if srv.TLSConfig != nil {
		config = srv.TLSConfig
	}
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, config)
	return srv.Serve(tlsListener)
}
