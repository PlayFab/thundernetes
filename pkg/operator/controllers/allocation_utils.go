package controllers

import (
	"net/http"
	"regexp"
	"html"

	"github.com/go-logr/logr"
)

// AllocateArgs contains information necessary to allocate a GameServer
type AllocateArgs struct {
	SessionID      string   `json:"sessionID"`
	BuildID        string   `json:"buildID"`
	SessionCookie  string   `json:"sessionCookie"`
	InitialPlayers []string `json:"initialPlayers"`
}

// isValidUUID returns true if the string is a valid UUID
func isValidUUID(uuid string) bool {
	r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
	return r.MatchString(uuid)
}

// validateAllocateArgs validates an instance of the AllocateArgs struct.
func validateAllocateArgs(aa *AllocateArgs) bool {
	if !isValidUUID(aa.SessionID) || !isValidUUID(aa.BuildID) {
		return false
	}
	return true
}

// RequestMultiplayerServerResponse contains details that are returned on a successful GameServer allocation call
type RequestMultiplayerServerResponse struct {
	IPV4Address string
	Ports       string
	SessionID   string
}

// internalServerError is a helper function for returning an internal server error
func internalServerError(w http.ResponseWriter, l logr.Logger, err error, msg string) {
	l.Error(err, msg)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 - " + html.EscapeString(msg) + " " + html.EscapeString(err.Error())))
}

// badRequestError is a helper function for returning a bad request error
func badRequestError(w http.ResponseWriter, l logr.Logger, err error, msg string) {
	l.Info(msg)
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte("400 - " + html.EscapeString(msg) + " " + html.EscapeString(err.Error())))
}

// tooManyRequestsError is a helper function for returning a too many requests error
func tooManyRequestsError(w http.ResponseWriter, l logr.Logger, err error, msg string) {
	l.Info(msg)
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte("429 - " + html.EscapeString(msg) + " " + html.EscapeString(err.Error())))
}

// notFoundError is a helper function for returning a not found error
func notFoundError(w http.ResponseWriter, l logr.Logger, err error, msg string) {
	l.Info(msg)
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 - " + html.EscapeString(msg) + " " + html.EscapeString(err.Error())))
}
