package main

import (
	"fmt"
	"strconv"
	"time"

	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	port := 56001
	portStr := os.Getenv("AGENT_PORT")
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil {
			log.Errorf("AGENT_PORT is not a number: %s", portStr)
		}
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatalf("NODE_NAME environment variable must be set")
	}

	nodeInternalIp := os.Getenv("NODE_INTERNAL_IP")
	if nodeInternalIp == "" {
		log.Fatalf("NODE_INTERNAL_IP environment variable must be set")
	}

	logEveryHeartbeatStr := os.Getenv("LOG_EVERY_HEARTBEAT")
	if logEveryHeartbeatStr == "true" {
		logEveryHeartbeat = true
	}

	dynamicClient, err := initializeKubernetesClient()
	if err != nil {
		log.Fatal(err)
	}

	n := NewNodeAgentManager(dynamicClient, nodeName)
	log.Debug("Starting HTTP server")
	http.HandleFunc("/v1/sessionHosts/", n.heartbeatHandler)
	http.HandleFunc("/healthz", healthzHandler)

	srv := &http.Server{
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
		Addr:           fmt.Sprintf(":%d", port),
	}

	err = srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
	close(n.watchStopper)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
