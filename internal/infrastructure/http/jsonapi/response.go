package jsonapi

import (
	"encoding/json"
	"net/http"
)

type envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(envelope{Data: data})
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(envelope{Error: msg})
}

func WriteNotFound(w http.ResponseWriter) {
	WriteError(w, http.StatusNotFound, "not found")
}

func WriteBadRequest(w http.ResponseWriter, msg string) {
	WriteError(w, http.StatusBadRequest, msg)
}
