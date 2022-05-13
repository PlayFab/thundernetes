package controllers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
)

const (
	// listeningPort is the port the API server will listen on
	listeningPort   = 5000
	allocationTries = 3
	// statusSessionId is the field name used to index GameServer objects by their session ID
	statusSessionId string = "status.sessionID"
	// specBuildId is the field name used to index GameServerBuild objects by their build ID
	specBuildId string = "spec.buildID"
)

// AllocationApiServer is a helper struct that implements manager.Runnable interface
// so it can be added to our Manager
type AllocationApiServer struct {
	Client client.Client
	// CrtBytes is the PEM-encoded certificate
	CrtBytes []byte
	// KeyBytes is the PEM-encoded key
	KeyBytes []byte
	// gameServerQueue is a map of priority queues for game servers
	gameServerQueue *GameServersQueue
	// events is a buffered channel of GenericEvent
	// it is used to re-enqueue GameServer objects that their allocation failed for whatever reason
	events chan event.GenericEvent
	logger logr.Logger
}

func NewAllocationApiServer(crt, key []byte, cl client.Client) *AllocationApiServer {
	return &AllocationApiServer{
		CrtBytes: crt,
		KeyBytes: key,
		Client:   cl,
		events:   make(chan event.GenericEvent, 100),
		logger:   log.Log.WithName("allocation-api"),
	}
}

// Start starts the HTTP(S) allocation API service
// if user has provided public/private cert details, it will create a TLS-auth HTTPS server
// otherwise it will create a HTTP server with no auth
func (s *AllocationApiServer) Start(ctx context.Context) error {
	addr := os.Getenv("API_LISTEN")
	if addr == "" {
		addr = fmt.Sprintf(":%d", listeningPort)
	}

	// create the queue for game servers
	s.gameServerQueue = NewGameServersQueue()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/allocate", s.handleAllocationRequest)

	s.logger.Info("serving allocation API service", "addr", addr, "port", listeningPort)

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
		s.logger.Info("shutting down allocation API service")

		shutDownContext, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
		defer cancelFunc()
		if err := srv.Shutdown(shutDownContext); err != nil {
			// Error from closing listeners, or context timeout
			s.logger.Error(err, "error shutting down the HTTP server")
		}
		close(done)
	}()

	if s.CrtBytes != nil && s.KeyBytes != nil {
		s.logger.Info("starting TLS enabled allocation API service")
		// Generate a key pair from your pem-encoded cert and key ([]byte).
		cert, err := tls.X509KeyPair(s.CrtBytes, s.KeyBytes)
		if err != nil {
			return nil
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(s.CrtBytes)
		// Construct a tls.config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    caCertPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}

		// Build a server:
		srv.TLSConfig = tlsConfig
		// Finally: serve.
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			return err
		}
	} else {
		s.logger.Info("starting insecure allocation API service")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
	}

	<-done
	return nil
}

// setupIndexers sets up the necessary indexers for the GameServer objects
func (s *AllocationApiServer) setupIndexers(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &mpsv1alpha1.GameServer{}, statusSessionId, func(rawObj client.Object) []string {
		gs := rawObj.(*mpsv1alpha1.GameServer)
		return []string{gs.Status.SessionID}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &mpsv1alpha1.GameServerBuild{}, specBuildId, func(rawObj client.Object) []string {
		gsb := rawObj.(*mpsv1alpha1.GameServerBuild)
		return []string{gsb.Spec.BuildID}
	}); err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the allocation api controller with the manager
func (s *AllocationApiServer) SetupWithManager(mgr ctrl.Manager) error {
	err := s.setupIndexers(mgr)
	if err != nil {
		return err
	}
	// our controller is triggered by changes in any GameServer object
	// as well as by manual insertions in the s.events channel
	err = ctrl.NewControllerManagedBy(mgr).
		For(&mpsv1alpha1.GameServer{}).
		Watches(&source.Channel{Source: s.events}, &handler.EnqueueRequestForObject{}).
		Complete(s)

	if err != nil {
		return err
	}
	return mgr.Add(s)
}

// Reconcile gets triggered when there is a change on a game server object
func (s *AllocationApiServer) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var gs mpsv1alpha1.GameServer
	if err := s.Client.Get(ctx, req.NamespacedName, &gs); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Unable to fetch GameServer, it was deleted - deleting from queue")
			s.gameServerQueue.RemoveFromQueue(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch GameServer")
		return ctrl.Result{}, err
	}
	// we only put a GameServer to the queue if it has reached the StandingBy state
	// making sure to record the ResourceVersion, to ensure deterministic lock in when we try and PATCH the GameServer with the Active state during allocation
	if gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
		s.gameServerQueue.PushToQueue(&GameServerForQueue{
			Name:            gs.Name,
			Namespace:       gs.Namespace,
			BuildID:         gs.Spec.BuildID,
			NodeAge:         gs.Status.NodeAge,
			ResourceVersion: gs.ObjectMeta.ResourceVersion,
		})
	}

	return ctrl.Result{}, nil
}

// handleAllocationRequest handles the allocation request from the client
func (s *AllocationApiServer) handleAllocationRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		badRequestError(w, s.logger, errors.New("invalid method"), "Only POST and PATCH are accepted")
	}

	// Parse args
	var args AllocateArgs
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		badRequestError(w, s.logger, err, "cannot deserialize json")
		return
	}

	// validate args
	isValid := validateAllocateArgs(&args)
	if !isValid {
		badRequestError(w, s.logger, errors.New("invalid sessionID or buildID"), "invalid arguments")
		return
	}

	// check if this build exists
	var gameServerBuilds mpsv1alpha1.GameServerBuildList
	err = s.Client.List(ctx, &gameServerBuilds, client.MatchingFields{specBuildId: args.BuildID})
	if err != nil {
		if apierrors.IsNotFound(err) {
			notFoundError(w, s.logger, err, "not found")
			return
		} else {
			internalServerError(w, s.logger, err, "error listing")
			return
		}
	}
	if len(gameServerBuilds.Items) == 0 {
		notFoundError(w, s.logger, errors.New("build not found"), fmt.Sprintf("Build with ID %s not found", args.BuildID))
		return
	}

	// check if this server is already allocated
	var gameserversForSessionID mpsv1alpha1.GameServerList
	err = s.Client.List(ctx, &gameserversForSessionID, &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{statusSessionId: args.SessionID}),
		LabelSelector: labels.SelectorFromSet(labels.Set{LabelBuildID: args.BuildID}),
	})
	if err != nil {
		internalServerError(w, s.logger, err, "error listing")
		return
	}

	// this should never happen, but just in case
	if len(gameserversForSessionID.Items) > 1 {
		internalServerError(w, s.logger, errors.New("multiple servers found"), fmt.Sprintf("Multiple servers found for sessionID %s", args.SessionID))
		return
	}

	// found a GameServer in this GameServerBuild with the same sessionID
	if len(gameserversForSessionID.Items) == 1 {
		// return it
		gs := gameserversForSessionID.Items[0]
		rs := RequestMultiplayerServerResponse{
			IPV4Address: gs.Status.PublicIP,
			Ports:       gs.Status.Ports,
			SessionID:   args.SessionID,
		}
		json.NewEncoder(w).Encode(rs)
		return
	}

	// allocation using the heap
	for i := 0; i < allocationTries; i++ {
		if i > 0 {
			s.logger.Info("allocation retrying", "buildID", args.BuildID, "retry count", i, "sessionID", args.SessionID)
		}
		gs := s.gameServerQueue.PopFromQueue(args.BuildID)
		if gs == nil {
			// pop from queue returned nil, this means no more game servers in this build
			tooManyRequestsError(w, s.logger, fmt.Errorf("not enough standingBy"), "there are not enough standingBy servers")
			return
		}

		// we got a standingBy server, so let's prepare for the Patch
		gs2 := mpsv1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:            gs.Name,
				Namespace:       gs.Namespace,
				ResourceVersion: gs.ResourceVersion,
			},
		}
		// we're using optimistic lock to make sure the ResourceVersion of the GameServer CR has not been modified
		m := &client.MergeFromWithOptimisticLock{}
		patch := client.MergeFromWithOptions(gs2.DeepCopy(), m)

		// set the relevant status fields for the GameServer
		gs2.Status.State = mpsv1alpha1.GameServerStateActive
		gs2.Status.SessionID = args.SessionID
		gs2.Status.SessionCookie = args.SessionCookie
		gs2.Status.InitialPlayers = args.InitialPlayers

		err = s.Client.Status().Patch(ctx, &gs2, patch)
		if err != nil {
			if apierrors.IsConflict(err) {
				s.logger.Info("error conflict patching game server", "error", err, "sessionID", args.SessionID, "buildID", args.BuildID, "retry", i)
			} else if apierrors.IsNotFound(err) {
				s.logger.Info("error not found patching game server", "error", err, "sessionID", args.SessionID, "buildID", args.BuildID, "retry", i)
			} else {
				s.logger.Error(err, "error patching game server", "sessionID", args.SessionID, "buildID", args.BuildID, "retry", i)
			}
			// in case of any error, trigger a reconciliation for this GameServer object
			// so it's re-added to the queue
			s.events <- event.GenericEvent{
				Object: &gs2,
			}
			// retry if possible
			continue
		}

		// once we reach this point, the GameServer has been successfully allocated
		rs := RequestMultiplayerServerResponse{
			IPV4Address: gs2.Status.PublicIP,
			Ports:       gs2.Status.Ports,
			SessionID:   args.SessionID,
		}
		err = json.NewEncoder(w).Encode(rs)
		if err != nil {
			internalServerError(w, s.logger, err, "encode json response")
			return
		}
		s.logger.Info("Allocated GameServer", "name", gs2.Name, "sessionID", args.SessionID, "buildID", args.BuildID, "ip", gs2.Status.PublicIP, "ports", gs2.Status.Ports)
		AllocationsCounter.WithLabelValues(gs2.Labels[LabelBuildName]).Inc()
		return
	}

	// if we reach this point, it means that we have tried multiple times and failed
	// we return the last error, if possible
	if err == nil {
		err = errors.New("unknown error")
	}
	s.logger.Info("Error allocating", "sessionID", args.SessionID, "buildID", args.BuildID, "error", err)
	internalServerError(w, s.logger, err, "error allocating")
}
