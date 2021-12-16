package main

import (
	"context"
	"fmt"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

func main() {
	port := getNumericEnv("AGENT_PORT", 56001)

	nodeName := getEnv("NODE_NAME", true)

	// todo: use these env vars
	// nodeInternalIp := getEnv("NODE_INTERNAL_IP", true)
	// logEveryHeartbeatStr := getEnv("LOG_EVERY_HEARTBEAT", true)

	dynamicClient, err := initializeKubernetesClient()
	if err != nil {
		log.Fatal(err)
	}

	n := NewNodeAgentManager(dynamicClient, nodeName)
	log.Debug("Starting HTTP server")
	http.HandleFunc("/v1/sessionHosts/", n.heartbeatHandler)
	http.HandleFunc("/healthz", healthzHandler)
	http.Handle("/metrics", promhttp.Handler())

	// main api server
	srv := &http.Server{
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
		Addr:           fmt.Sprintf(":%d", port),
	}

	// starts the server in a goroutine
	startHttpServer(srv)

	// wait for SIGINT or SIGTERM
	ctx, cancel := waitForShutdownSignal()
	defer cancel()

	// shut down gracefully, but wait no longer than 5 seconds before halting
	stopHttpServer(srv, ctx)

	close(n.watchStopper)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func waitForShutdownSignal() (context.Context, context.CancelFunc) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("Shutting down")
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func getEnv(key string, required bool) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	if required {
		log.Fatal(fmt.Sprintf("%s environment variable must be set", key))
	}
	return ""
}

func getNumericEnv(key string, fallback int) int {
	if strValue := getEnv(key, false); strValue != "" {
		if v, err := strconv.Atoi(strValue); err == nil {
			return v
		}
	}
	log.Warn(fmt.Sprintf("%s must be a number", key))
	return fallback
}

func startHttpServer(server *http.Server) {
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()
}

func stopHttpServer(srv *http.Server, ctx context.Context) {
	err := srv.Shutdown(ctx)
	if err != nil {
		log.Error(err)
	}
}
