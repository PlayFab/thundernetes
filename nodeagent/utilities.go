package main

import (
	"errors"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func internalServerError(w http.ResponseWriter, err error, msg string) {
	log.Debugf("Error %s because of %s \n", msg, err.Error())
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 - " + msg + " " + err.Error()))
}

func badRequest(w http.ResponseWriter, err error, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte("400 - " + msg + " " + err.Error()))
}

func validateHeartbeatRequestArgs(hb *HeartbeatRequest) error {
	var msg string
	if hb.CurrentGameHealth == "" {
		msg = "CurrentGameHealth cannot be empty"
	}
	if hb.CurrentGameState == "" {
		msg = fmt.Sprintf("%s - CurrentGameState cannot be empty", msg)
	}
	if msg != "" {
		return errors.New(msg)
	}
	return nil
}

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

func getLogger(gameServerName, gameServerNamespace string) *log.Entry {
	return log.WithFields(log.Fields{"GameServerName": gameServerName, "GameServerNamespace": gameServerNamespace})
}
