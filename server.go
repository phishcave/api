package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

const (
	urlUploadID    = "upload_id"
	urlChunkID     = "chunk_id"
	formValueChunk = "chunk"

	longPollTimeout = 2 * time.Minute
)

var (
	uploadNotFound = errors.New("upload not found")
	badRequest     = errors.New("bad request")
)

func main() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	mux := mux.NewRouter()

	gets := mux.Methods("GET").PathPrefix("/api").Subrouter()
	posts := mux.Methods("POST").PathPrefix("/api").Subrouter()
	gets.Handle("/upload/{upload_id}/status", handleErr(statusHandler))

	finishedHandler, processedHandler := longPoll(finishedSelector), longPoll(processedSelector)
	gets.Handle("/upload/{upload_id}/finished", handleErr(finishedHandler.ServeHTTP))
	gets.Handle("/upload/{upload_id}/processed", handleErr(processedHandler.ServeHTTP))

	posts.Handle("upload/chunk/{upload_id}/{chunk_id}", handleErr(chunkHandler))
	posts.Handle("upload", handleErr(uploadHandler))

	log.Println(http.ListenAndServe(net.JoinHostPort("", port), mux))
}

type UploadRequest struct {
	Name      string `json:"name"`
	Size      int64  `json:"size,omitempty"`
	Chunks    int    `json:"chunks,omitempty"`
	ChunkSize int    `json:"chunk_size,omitempty"`
}

type UploadResponse struct {
	ID     uint   `json:"upload_id,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	File   *File  `json:"file,omitempty"`
}

type File struct {
	Name      string    `json:"name,omitempty"`
	Size      int64     `json:"size,omitempty"`
	Author    string    `json:"author,omitempty"`
	Date      time.Time `json:"date,omitempty"`
	Thumbnail string    `json:"thumbnail,omitempty"`
}

type StatusResponse struct {
	Status []int `json:"status"`
}

func getUploadID(r *http.Request) (uint, error) {
	vals := mux.Vars(r)
	str, ok := vals[urlUploadID]

	if !ok {
		return 0, badRequest
	}

	bigID, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0, badRequest
	}

	return uint(bigID), nil
}

func getUpload(r *http.Request) (upload *Upload, err error) {
	var id uint
	id, err = getUploadID(r)
	if err != nil {
		return nil, err
	}

	uploadMut.RLock()
	upload, ok := uploads[id]
	uploadMut.RUnlock()

	if !ok {
		return nil, uploadNotFound
	}

	return upload, nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) (err error) {
	dec := json.NewDecoder(r.Body)

	// Decode json request
	var req UploadRequest
	if err = dec.Decode(&req); err != nil {
		return err
	}
	if err = r.Body.Close(); err != nil {
		return err
	}

	// TODO(dylanj): Check if we have the file in DB, because currently this constant won't do
	haveFile := false

	// Prepare response
	var resp UploadResponse
	if haveFile {
		resp.Status = "error"
		resp.Error = "file already exists"

		// TODO(dylanj): Fill out the files stuff from the DB
		resp.File = &File{
			Name:      "lollerskates.jpepng",
			Size:      18348,
			Author:    "fish",
			Date:      time.Now(),
			Thumbnail: "/thumb.jpegif",
		}
	} else {
		resp.Status = "success"
		resp.ID, err = createSlot(req)
		if err != nil {
			return err
		}
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	return enc.Encode(&resp)
}

func statusHandler(w http.ResponseWriter, r *http.Request) (err error) {
	upload, err := getUpload(r)
	if err != nil {
		return err
	}

	var status StatusResponse
	statusChan, ok := <-upload.Status
	if ok {
		status.Status = <-statusChan
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	return enc.Encode(&status)
}

func chunkHandler(w http.ResponseWriter, r *http.Request) (err error) {
	vars := mux.Vars(r)
	chunkID, err := strconv.Atoi(vars[urlChunkID])
	if err != nil {
		return err
	}

	file, _, err := r.FormFile(formValueChunk)
	if err != nil {
		return err
	}
	defer file.Close()

	upload, err := getUpload(r)
	if err != nil {
		return err
	}

	_, ok := <-upload.WriteStart
	if !ok {
		w.WriteHeader(http.StatusNotFound) // Upload was timeouted before we could write
		return nil
	}

	offset := int64(chunkID) * int64(upload.FileInfo.ChunkSize)

	data, err := ioutil.ReadAll(file)
	if err != nil {
		upload.WriteEnd <- -1
		return err
	}

	_, err = upload.File.WriteAt(data, offset)
	if err != nil {
		upload.WriteEnd <- -1
		return err
	}

	upload.WriteEnd <- chunkID
	return nil
}

type longPoll func(u *Upload) chan bool

func (selector longPoll) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	upload, err := getUpload(r)
	if err != nil {
		return err
	}

	select {
	case <-time.After(longPollTimeout):
		w.WriteHeader(http.StatusRequestTimeout)
		return nil
	case <-selector(upload):
	}

	var resp File

	resp.Name = "My file"
	// TODO(dylanj): Pull file from DB and fill with love, if it has thumbnail it has it, if not it doesn't
	// but if the URL hit was api/upload/{id}/processed then it will have a thumbnail if it was ever going
	// to get one.

	enc := json.NewEncoder(w)
	return enc.Encode(&resp)
}

func finishedSelector(u *Upload) chan bool {
	return u.FinishLatch
}

func processedSelector(u *Upload) chan bool {
	return u.PostProcessLatch
}
