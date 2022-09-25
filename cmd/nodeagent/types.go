package main

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var (
	gameserverGVR = schema.GroupVersionResource{
		Group:    "mps.playfab.com",
		Version:  "v1alpha1",
		Resource: "gameservers",
	}

	gameserverDetailGVR = schema.GroupVersionResource{
		Group:    "mps.playfab.com",
		Version:  "v1alpha1",
		Resource: "gameserverdetails",
	}
)

// GameState represents the current state of the game.
type GameState string

// GameOperation represents the type of operation that the GSDK shoud do next
type GameOperation string

const (
	GameStateInvalid      GameState = "Invalid"
	GameStateInitializing GameState = "Initializing"
	GameStateStandingBy   GameState = "StandingBy"
	GameStateActive       GameState = "Active"
	GameStateTerminating  GameState = "Terminating"
	GameStateTerminated   GameState = "Terminated"
	GameStateQuarantined  GameState = "Quarantined" // Not used
)

const (
	GameOperationInvalid   GameOperation = "Invalid"
	GameOperationContinue  GameOperation = "Continue"
	GameOperationActive    GameOperation = "Active"
	GameOperationTerminate GameOperation = "Terminate"
)

var (
	GameServerStates = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "thundernetes",
		Name:      "gameserver_states",
		Help:      "Game server states",
	}, []string{"name", "state"})

	ConnectedPlayersGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "thundernetes",
		Name:      "connected_players",
		Help:      "Number of connected players per GameServer",
	}, []string{"namespace", "ServerName", "BuildName"})

	GameServerCreateDuration = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "thundernetes",
			Name:      "gameserver_create_duration",
			Help:      "Time taken to create a GameServers",
		},
		[]string{"BuildName"},
	)
)

// HeartbeatRequest contains data for the heartbeat request coming from the GSDK running alongside GameServer
type HeartbeatRequest struct {
	// CurrentGameState is the current state of the game server
	CurrentGameState GameState `json:"CurrentGameState"`
	// CurrentGameHealth is the current health of the game server
	CurrentGameHealth string `json:"CurrentGameHealth"`
	// CurrentPlayers is a slice containing details about the players currently connected to the game
	CurrentPlayers []ConnectedPlayer `json:"CurrentPlayers"`
}

// HeartbeatResponse contains data for the heartbeat response that is being sent to the GSDK running alongside GameServer
type HeartbeatResponse struct {
	SessionConfig               SessionConfig `json:"sessionConfig,omitempty"`
	NextScheduledMaintenanceUtc string        `json:"nextScheduledMaintenanceUtc,omitempty"`
	Operation                   GameOperation `json:"operation,omitempty"`
}

// SessionConfig contains data for the session config that is being sent to the GSDK running alongside GameServer
type SessionConfig struct {
	SessionId      string            `json:"sessionId,omitempty"`
	SessionCookie  string            `json:"sessionCookie,omitempty"`
	InitialPlayers []string          `json:"initialPlayers,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// ConnectedPlayer contains data for a player connected to the game
type ConnectedPlayer struct {
	PlayerId string
}

// GameServerInfo contains data regarding the details for the session that occurs when the GameServer state changes
type GameServerInfo struct {
	IsActive              bool // the GameState is Active on the Kubernetes API server
	SessionID             string
	SessionCookie         string
	InitialPlayers        []string
	PreviousGameState     GameState // the GameState on the previous heartbeat
	PreviousGameHealth    string    // the GameHealth on the previous heartbeat
	GameServerNamespace   string
	ConnectedPlayersCount int
	Mutex                 *sync.RWMutex
	GsUid                 types.UID    // UID of the GameServer object
	CreationTime          int64        // time when this GameServerInfo was created in the nodeagent
	CreationTimeStamp     *metav1.Time // time when the GameServer was created
	LastHeartbeatTime     int64        // time since the nodeagent received a heartbeat from this GameServer
	MarkedUnhealthy       bool         // if the GameServer was marked unhealthy by a heartbeat condition, used to avoid repeating the patch
	BuildName             string       // the name of the GameServerBuild that this GameServer belongs to
}
