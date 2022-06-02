package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// ParseInt64FromEnv tries to read an int64 from an environment variable envVar
// if not possible it returns the defaultValue provided
func ParseInt64FromEnv(envVar string, defaultValue int64) int64 {
	value, ok := os.LookupEnv(envVar)
	if !ok {
		log.Infof("Env variable %s not found, using default value %d", envVar, defaultValue)
	} else {
		parsedValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			log.Infof("Error parsing env variable %s as int64, using default value %d", envVar, defaultValue)
		} else {
			return parsedValue
		}
	}
	return defaultValue	
}

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

// parseSessionDetails returns the sessionID and sessionCookie from the unstructured GameServer CR
func parseSessionDetails(u *unstructured.Unstructured, gameServerName, gameServerNamespace string) (string, string, []string) {
	logger := getLogger(gameServerName, gameServerNamespace)
	sessionID, sessionIDExists, sessionIDErr := unstructured.NestedString(u.Object, "status", "sessionID")
	sessionCookie, sessionCookieExists, sessionCookieErr := unstructured.NestedString(u.Object, "status", "sessionCookie")
	initialPlayers, initialPlayersExists, initialPlayersErr := unstructured.NestedStringSlice(u.Object, "status", "initialPlayers")

	if !sessionIDExists || !sessionCookieExists || !initialPlayersExists {
		logger.Debugf("sessionID or sessionCookie or initialPlayers do not exist, sessionIDExists: %t, sessionCookieExists: %t, initialPlayersExists: %t", sessionIDExists, sessionCookieExists, initialPlayersExists)
	}

	if sessionIDErr != nil {
		logger.Debugf("error getting sessionID: %s", sessionIDErr.Error())
	}

	if sessionCookieErr != nil {
		logger.Debugf("error getting sessionCookie: %s", sessionCookieErr.Error())
	}

	if initialPlayersErr != nil {
		logger.Debugf("error getting initialPlayers: %s", initialPlayersErr.Error())
	}

	return sessionID, sessionCookie, initialPlayers
}

// parseStateHealth parses the GameServer state and health from the unstructured GameServer CR.
// Returns state, health and error
func parseStateHealth(u *unstructured.Unstructured) (string, string, error) {
	state, stateExists, stateErr := unstructured.NestedString(u.Object, "status", "state")
	health, healthExists, healthErr := unstructured.NestedString(u.Object, "status", "health")

	if stateErr != nil {
		return "", "", stateErr
	}
	if !stateExists {
		return "", "", errors.New(ErrStateNotExists)
	}

	if healthErr != nil {
		return "", "", stateErr
	}
	if !healthExists {
		return "", "", errors.New(ErrHealthNotExists)
	}
	return state, health, nil
}

// parseBuildID parses and returns the GameServer buildID from the unstructured GameServer CR.
func parseBuildName(u *unstructured.Unstructured) (string, error) {
	buildID, buildIDExists, buildIDErr := unstructured.NestedString(u.Object, "metadata", "labels", "BuildName")
	if buildIDErr != nil {
		return "", buildIDErr
	}
	if !buildIDExists {
		return "", errors.New(ErrBuildIDNotExists)
	}
	return buildID, nil
}
