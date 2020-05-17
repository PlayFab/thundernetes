package http

import (
	"context"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// internalServerError is a helper function for returning an internal server error
func internalServerError(ctx context.Context, w http.ResponseWriter, err error, msg string) {
	log := log.FromContext(ctx)
	log.Error(err, msg)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 - " + msg + " " + err.Error()))
}

// badRequestError is a helper function for returning a bad request error
func badRequestError(ctx context.Context, w http.ResponseWriter, err error, msg string) {
	log := log.FromContext(ctx)
	log.Info(msg)
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte("400 - " + msg + " " + err.Error()))
}

// tooManyRequestsError is a helper function for returning a too many requests error
func tooManyRequestsError(ctx context.Context, w http.ResponseWriter, err error, msg string) {
	log := log.FromContext(ctx)
	log.Info(msg)
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte("429 - " + msg + " " + err.Error()))
}

// notFoundError is a helper function for returning a not found error
func notFoundError(ctx context.Context, w http.ResponseWriter, err error, msg string) {
	log := log.FromContext(ctx)
	log.Info(msg)
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 - " + msg + " " + err.Error()))
}
