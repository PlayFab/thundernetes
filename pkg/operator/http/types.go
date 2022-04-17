package http

import (
	"regexp"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var allocationLogger = log.Log.WithName("allocation-api")

// AllocateArgs contains information necessary to allocate a GameServer
type AllocateArgs struct {
	SessionID      string   `json:"sessionID"`
	BuildID        string   `json:"buildID"`
	SessionCookie  string   `json:"sessionCookie"`
	InitialPlayers []string `json:"initialPlayers"`
}

// isValidUUID returns true if the string is a valid UUID
func isValidUUID(uuid string) bool {
	r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
	return r.MatchString(uuid)
}

// validateAllocateArgs validates an instance of the AllocateArgs struct.
func validateAllocateArgs(aa *AllocateArgs) bool {
	if !isValidUUID(aa.SessionID) || !isValidUUID(aa.BuildID) {
		return false
	}
	return true
}

// RequestMultiplayerServerResponse contains details that are returned on a successful GameServer allocation call
type RequestMultiplayerServerResponse struct {
	IPV4Address string
	Ports       string
	SessionID   string
}

// GameServers is an alias for a GameServer slice
// following code is needed to implement sort.Interface ourselves, for faster performance
// https://stackoverflow.com/questions/54276285/performance-sorting-slice-vs-sorting-type-of-slice-with-sort-implementation
type GameServers []mpsv1alpha1.GameServer

// Less is part of sort.Interface. It is implemented by comparing the NodeAge on the GameServer
func (g GameServers) Less(i, j int) bool {
	return g[i].Status.NodeAge < g[j].Status.NodeAge
}

// Len is part of sort.Interface. It is implemented by returning the length of the GameServer slice
func (g GameServers) Len() int { return len(g) }

// Swap is part of sort.Interface. It is implemented by swapping two items on the GameServer slice
func (g GameServers) Swap(i, j int) { g[i], g[j] = g[j], g[i] }

//-------------------------------------------------------------------------------------------------
type ByNodeAge []mpsv1alpha1.GameServer

func (a ByNodeAge) Len() int           { return len(a) }
func (a ByNodeAge) Less(i, j int) bool { return a[i].Status.NodeAge < a[j].Status.NodeAge }
func (a ByNodeAge) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

//-------------------------------------------------------------------------------------------------
