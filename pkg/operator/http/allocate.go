package http

import (
	"encoding/json"
	"errors"
	"math/rand"

	"fmt"
	"net/http"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"github.com/playfab/thundernetes/pkg/operator/controllers"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type allocateHandler struct {
	client client.Client
	config *rest.Config
	scheme *runtime.Scheme
}

func (h *allocateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r)
}

func (h *allocateHandler) handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		badRequestError(ctx, w, errors.New("invalid method"), "Only POST and PATCH are accepted")
	}

	// Parse args.
	var args AllocateArgs
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		badRequestError(ctx, w, err, "cannot deserialize json")
		return
	}

	// validate args
	isValid := validateAllocateArgs(&args)
	if !isValid {
		badRequestError(ctx, w, errors.New("invalid sessionID or buildID"), "invalid arguments")
		return
	}

	// check if this build exists
	var gameServerBuilds mpsv1alpha1.GameServerBuildList
	err = h.client.List(ctx, &gameServerBuilds, client.MatchingFields{"spec.buildID": args.BuildID})
	if err != nil {
		if kerrors.IsNotFound(err) {
			notFoundError(ctx, w, err, "not found")
			return
		} else {
			internalServerError(ctx, w, err, "error listing")
			return
		}
	}
	if len(gameServerBuilds.Items) == 0 {
		notFoundError(ctx, w, errors.New("build not found"), fmt.Sprintf("Build with ID %s not found", args.BuildID))
		return
	}

	// check if this server is already allocated
	var gameserversForSessionID mpsv1alpha1.GameServerList
	err = h.client.List(r.Context(), &gameserversForSessionID, &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"status.sessionID": args.SessionID}),
		LabelSelector: labels.SelectorFromSet(labels.Set{controllers.LabelBuildID: args.BuildID}),
	})
	if err != nil {
		internalServerError(ctx, w, err, "error listing")
		return
	}

	// this should never happen, but just in case
	if len(gameserversForSessionID.Items) > 1 {
		internalServerError(ctx, w, errors.New("multiple servers found"), fmt.Sprintf("Multiple servers found for sessionID %s", args.SessionID))
		return
	}

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

	// get the standingBy GameServers for this BuildID
	var gameserversStandingBy mpsv1alpha1.GameServerList
	err = h.client.List(r.Context(), &gameserversStandingBy, &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"status.state": "StandingBy"}),
		LabelSelector: labels.SelectorFromSet(labels.Set{controllers.LabelBuildID: args.BuildID}),
	})
	if err != nil {
		internalServerError(ctx, w, err, "error listing")
		return
	}

	if len(gameserversStandingBy.Items) == 0 {
		tooManyRequestsError(ctx, w, fmt.Errorf("not enough standingBy"), "there are not enough standingBy servers")
		return
	}

	// pick a random one
	gs := gameserversStandingBy.Items[rand.Intn(len(gameserversStandingBy.Items))]

	gsd := createGameServerDetailForGameServer(&gs, args.InitialPlayers)

	err = h.client.Create(r.Context(), &gsd)
	if err != nil {
		internalServerError(ctx, w, err, "cannot create GameServerDetail")
		return
	}

	// set the relevant status fields
	gs.Status.State = mpsv1alpha1.GameServerStateActive
	gs.Status.SessionID = args.SessionID
	gs.Status.SessionCookie = args.SessionCookie

	err = h.client.Status().Update(r.Context(), &gs)
	if err != nil {
		internalServerError(ctx, w, err, "cannot update game server")
		return
	}

	rs := RequestMultiplayerServerResponse{
		IPV4Address: gs.Status.PublicIP,
		Ports:       gs.Status.Ports,
		SessionID:   args.SessionID,
	}
	err = json.NewEncoder(w).Encode(rs)
	if err != nil {
		internalServerError(ctx, w, err, "encode json response")
		return
	}
	controllers.AllocationsCounter.WithLabelValues(gs.Labels[controllers.LabelBuildName]).Inc()
}

func createGameServerDetailForGameServer(gs *mpsv1alpha1.GameServer, initialPlayers []string) mpsv1alpha1.GameServerDetail {
	return mpsv1alpha1.GameServerDetail{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name,
			Namespace: gs.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(gs, schema.GroupVersionKind{
					Group:   mpsv1alpha1.GroupVersion.Group,
					Version: mpsv1alpha1.GroupVersion.Version,
					Kind:    controllers.GameServerKind,
				}),
			},
			Labels: map[string]string{controllers.LabelBuildID: gs.Spec.BuildID, controllers.LabelOwningGameServer: gs.Name},
		},
		Spec: mpsv1alpha1.GameServerDetailSpec{
			InitialPlayers:        initialPlayers,
			ConnectedPlayersCount: 0,
		},
	}
}
