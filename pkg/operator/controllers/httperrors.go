package controllers

import (
	"net/http"
)

// internalServerError is a helper function for returning an internal server error
func internalServerError(w http.ResponseWriter, err error, msg string) {
	allocationLogger.Error(err, msg)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 - " + msg + " " + err.Error()))
}

// badRequestError is a helper function for returning a bad request error
func badRequestError(w http.ResponseWriter, err error, msg string) {
	allocationLogger.Info(msg)
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte("400 - " + msg + " " + err.Error()))
}

// tooManyRequestsError is a helper function for returning a too many requests error
func tooManyRequestsError(w http.ResponseWriter, err error, msg string) {
	allocationLogger.Info(msg)
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte("429 - " + msg + " " + err.Error()))
}

// notFoundError is a helper function for returning a not found error
func notFoundError(w http.ResponseWriter, err error, msg string) {
	allocationLogger.Info(msg)
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 - " + msg + " " + err.Error()))
}
