package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"

	"fmt"
	"net/http"

	mpsv1alpha1 "github.com/playfab/thundernetes/operator/api/v1alpha1"
	"github.com/playfab/thundernetes/operator/controllers"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type allocateHandler struct {
	client                       client.Client
	config                       *rest.Config
	scheme                       *runtime.Scheme
	changeStatusInternalProvider func(podIP, state, sessionCookie, sessionId string, initialPlayers []string) error
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

	// set the relevant status fields
	gs.Status.State = mpsv1alpha1.GameServerStateActive
	gs.Status.SessionID = args.SessionID
	gs.Status.SessionCookie = args.SessionCookie
	gs.Status.InitialPlayers = args.InitialPlayers

	err = h.client.Status().Update(r.Context(), &gs)
	if err != nil {
		internalServerError(ctx, w, err, "cannot update game server")
		return
	}

	var pod corev1.Pod
	err = h.client.Get(r.Context(), types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, &pod)
	if err != nil {
		internalServerError(ctx, w, err, "cannot get pod")
		return
	}

	// after we change the GameServer.Status.State to Active, we need to notify the sidecar's HTTP server
	err = h.changeStatusInternalProvider(pod.Status.PodIP, "Active", args.SessionCookie, args.SessionID, args.InitialPlayers)
	if err != nil {
		// error in calling the Pod, let's try to revert the status of the GameServer
		err2 := h.client.Get(r.Context(), types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, &gs)
		if err2 != nil {
			internalServerError(ctx, w, err2, "cannot get game server to revert state")
			return
		}

		gs.Status.State = mpsv1alpha1.GameServerStateStandingBy
		gs.Status.SessionID = ""
		gs.Status.SessionCookie = ""
		gs.Status.InitialPlayers = nil
		err2 = h.client.Status().Update(r.Context(), &gs)
		if err2 != nil {
			internalServerError(ctx, w, err2, "cannot update game server to revert state")
			return
		}

		internalServerError(ctx, w, err, fmt.Sprintf("cannot change status on Pod %s - reverted state on GameServer %s", pod.Name, gs.Name))
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

func changeStatusInternal(podIP, state, sessionCookie, sessionId string, initialPlayers []string) error {
	postBody, _ := json.Marshal(map[string]interface{}{
		"state":          state,
		"sessionCookie":  sessionCookie,
		"sessionId":      sessionId,
		"initialPlayers": initialPlayers,
	})

	postBodyBytes := bytes.NewBuffer(postBody)
	resp, err := http.Post(fmt.Sprintf("http://%s:%d/v1/changeState", podIP, controllers.SidecarPort), "application/json", postBodyBytes)
	//Handle Error
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %d", "invalid status code", resp.StatusCode)
	}

	//Read the response body
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return nil
}
