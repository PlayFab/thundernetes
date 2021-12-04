package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
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

	logger := log.WithFields(log.Fields{"GameServerName": gameServerName, "GameServerNamespace": crdNamespace})

	sm := NewSidecarManager(k8sClient, gameServerName, crdNamespace, logger)

	http.HandleFunc("/v1/sessionHosts/", sm.heartbeatHandler)

	http.ListenAndServe(fmt.Sprintf("localhost:%d", SidecarPort), nil)
}
