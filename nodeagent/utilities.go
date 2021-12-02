package main

import (
	"errors"
	"fmt"
	"net/http"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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

func initializeKubernetesClient() (*kubernetes.Clientset, dynamic.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return clientset, client, nil
}
