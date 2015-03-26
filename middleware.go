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
	case errBadRequest:
		log.Printf("Bad Request (%s | %s): %v", r.URL.Path, r.RemoteAddr, err)
		w.WriteHeader(http.StatusBadRequest)
	case errUploadNotFound:
		log.Printf("Upload Not Found (%s | %s): %v", r.URL.Path, r.RemoteAddr, err)
		w.WriteHeader(http.StatusNotFound)
	case errAlreadyUploading:
		log.Printf("Duplicate Upload (%s | %s): %v", r.URL.Path, r.RemoteAddr, err)
		w.WriteHeader(http.StatusTeapot)
	default:
		log.Printf("Error (%s | %s): %v", r.URL.Path, r.RemoteAddr, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
