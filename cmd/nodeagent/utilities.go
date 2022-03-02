package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// internalServerError writes an internal server error to the response
func internalServerError(w http.ResponseWriter, err error, msg string) {
	log.Debugf("Error %s because of %s \n", msg, err.Error())
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 - " + msg + " " + err.Error()))
}

// badRequest writes a bad request error to the response
func badRequest(w http.ResponseWriter, err error, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte("400 - " + msg + " " + err.Error()))
}

// validateHeartbeatRequest validates the heartbeat request
// returns an error if invalid
func validateHeartbeatRequest(hb *HeartbeatRequest) error {
	// some GSDKs (e.g. Unity) send empty string as Health
	// we should accept it and set it to Healthy
	if hb.CurrentGameHealth == "" {
		hb.CurrentGameHealth = "Healthy"
	}
	var msg string
	if hb.CurrentGameState == "" {
		msg = fmt.Sprintf("%s - CurrentGameState cannot be empty", msg)
	}
	if msg != "" {
		return errors.New(msg)
	}
	return nil
}

// initializeKubernetesClient initializes and returns a dynamic Kubernetes client
func initializeKubernetesClient() (dynamic.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// isValidStateTransition returns true if the transition between the two states is valid
// transition from "" to Initializing and StandingBy is valid
// transition from Initializing to StandingBy is valid
// transition from StandingBy to Active is valid
func isValidStateTransition(old, new GameState) bool {
	if old == "" && new == GameStateInitializing {
		return true
	}
	if old == "" && new == GameStateStandingBy {
		return true
	}
	if old == GameStateInitializing && new == GameStateStandingBy {
		return true
	}
	if old == GameStateStandingBy && new == GameStateActive {
		return true
	}
	if old == new {
		return true
	}
	return false
}

// getLogger returns a logger for the specified game server name and namespace
func getLogger(gameServerName, gameServerNamespace string) *log.Entry {
	return log.WithFields(log.Fields{"GameServerName": gameServerName, "GameServerNamespace": gameServerNamespace})
}

// sanitize removes new line characters from the string
// https://codeql.github.com/codeql-query-help/go/go-log-injection/
func sanitize(s string) string {
	s2 := strings.Replace(s, "\n", "", -1)
	return strings.Replace(s2, "\r", "", -1)
}
