package main

import (
	"log"
	"net/http"
)

type ErrHandler func(w http.ResponseWriter, r *http.Request) error

type errorMiddleware struct {
	handler ErrHandler
}

func handleErr(h ErrHandler) http.Handler {
	return errorMiddleware{h}
}

func (e errorMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add panic handler if desired.

	err := e.handler(w, r)
	switch err {
	case badRequest:
		log.Printf("BadRequest (%s | %s): %v", r.URL.Path, r.RemoteAddr, err)
		w.WriteHeader(http.StatusBadRequest)
	case uploadNotFound:
		log.Printf("Upload Not Found (%s | %s): %v", r.URL.Path, r.RemoteAddr, err)
		w.WriteHeader(http.StatusNotFound)
	default:
		log.Printf("Error (%s | %s): %v", r.URL.Path, r.RemoteAddr, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
