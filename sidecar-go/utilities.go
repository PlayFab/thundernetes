package main

import (
	"errors"
	"fmt"
	"net/http"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func internalServerError(w http.ResponseWriter, err error, msg string) {
	fmt.Printf("Error %s because of %s\n", msg, err.Error())
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

func validateSessionDetailsArgs(sd *SessionDetails) error {
	var msg string
	if sd.SessionId == "" {
		msg = "SessionId cannot be empty"
	}
	if sd.State == "" {
		msg = fmt.Sprintf("%s - State cannot be empty", msg)
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
