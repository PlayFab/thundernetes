package main

import (
	"fmt"
	"net/http"
	"os"
)

const SidecarPort = 56001

func main() {
	gameServerName := os.Getenv("PF_GAMESERVER_NAME")
	if gameServerName == "" {
		panic("PF_GAMESERVER_NAME not defined")
	}

	crdNamespace := os.Getenv("PF_GAMESERVER_NAMESPACE")
	if crdNamespace == "" {
		panic("PF_GAMESERVER_NAMESPACE not defined")
	}

	var err error
	k8sClient, err := initializeKubernetesClient()
	if err != nil {
		panic(err)
	}

	h := NewHttpHandler(k8sClient, gameServerName, crdNamespace)

	http.HandleFunc("/v1/sessionHosts/", h.heartbeatHandler)

	http.ListenAndServe(fmt.Sprintf(":%d", SidecarPort), nil)
}
