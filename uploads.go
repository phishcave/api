package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	uploadTimeout = 2 * time.Minute
	tmpPrefix     = "phishcave"
	savePath      = "phishcave/uploads"
)

var (
	uploads   map[uint]*Upload
	uploadMut sync.RWMutex
)

type Upload struct {
	ID       uint
	FileInfo UploadRequest
	File     *os.File

	// During upload these channels are used
	// to sync chunk writers and status queryers.
	WriteStart chan bool
	WriteEnd   chan int
	Status     chan chan []int

	// After all chunks have been uploaded
	// these channels will be closed when their stages
	// are reached to allow queryers to die.
	FinishLatch      chan bool
	PostProcessLatch chan bool
}

func createSlot(req UploadRequest) (uint, error) {
	file, err := ioutil.TempFile("", tmpPrefix)
	if err != nil {
		log.Printf("Error: Could not open temp file for upload (%s): %v", req.Name, err)
		return 0, err
	}

	upload := &Upload{
		FileInfo: req,
		File:     file,

		WriteStart: make(chan bool),
		WriteEnd:   make(chan int),
		Status:     make(chan chan []int),

		FinishLatch:      make(chan bool),
		PostProcessLatch: make(chan bool),
	}

	uploadMut.Lock()
	var id uint
	for {
		id = uint(rand.Uint32())
		if id == 0 {
			continue
		}

		if _, ok := uploads[id]; !ok {
			upload.ID = id
			uploads[id] = upload
			break
		}
	}
	uploadMut.Unlock()

	go upload.manage()
	return id, nil
}

func (u *Upload) manage() {
	timeout := time.After(uploadTimeout)
	statusChan := make(chan []int)

	writers := 0
	chunks := 0
	timeouted := false
	chunkList := NewChunkList(u.FileInfo.Chunks)
	for chunks < u.FileInfo.Chunks && !timeouted {
		select {
		case <-timeout:
			timeouted = true
		case u.WriteStart <- true:
			timeout = time.After(uploadTimeout)
			writers++
		case number := <-u.WriteEnd:
			chunkList.Set(number)
			chunks++
			writers--
		case u.Status <- statusChan:
			statusChan <- chunkList.ToArray()
		}
	}

	close(statusChan)

	u.finish(writers, timeouted)
}

func (u *Upload) finish(writersLeft int, timeout bool) {
	close(u.WriteStart)
	close(u.Status)

	// Drain the writers before closing file
	for ; writersLeft > 0; writersLeft-- {
		<-u.WriteEnd
	}

	failure := false
	if timeout {
		log.Printf("finishUpload: The upload (%s) timed out", u.FileInfo.Name)
		failure = true
	}

	var sha string
	var err error
	if !failure {
		sha, err = sha256sum(u.File)
		if err != nil {
			log.Printf("finishUpload: Failed to hash the file %q (%v)", u.FileInfo.Name, err)
			failure = true
		}
	}

	if err = u.File.Close(); err != nil {
		log.Println("finishUpload: File failed to close %q:", u.FileInfo.Name, err)
		failure = true
	}

	// Remove this temp file no matter what, if it's been moved and
	// this fails, we don't care.
	oldFileName := u.File.Name()
	defer os.Remove(oldFileName)
	if failure {
		return
	}

	newFileName := filepath.Join(savePath, sha)

	if err := os.Rename(oldFileName, newFileName); err != nil {
		fmt.Printf("Rename: Failed to move file %q -> %q (%v)", oldFileName, newFileName, err)
		return
	}

	// TODO(dylanj): Update DB with details

	close(u.FinishLatch) // Ready to serve the files to the user
	u.postProcess(u.FileInfo, newFileName)
	close(u.PostProcessLatch) // Ready to serve postprocessed files like thumbnails

	uploadMut.Lock()
	delete(uploads, u.ID)
	uploadMut.Unlock()
}

func sha256sum(f io.ReadSeeker) (sha string, err error) {
	if _, err = f.Seek(0, os.SEEK_SET); err != nil {
		return "", err
	}

	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%064x", hash.Sum(nil)), nil
}

func (u *Upload) postProcess(fi UploadRequest, filename string) {
	output := &bytes.Buffer{}

	// TODO(dylanj): What does this function actually do?

	var err error
	if err = runCmd(output, fi.Name, "file", "--mime-type", filename); err != nil {
		return
	}

	mime := output.String()
	switch {
	case strings.HasPrefix(mime, "image/"):
		//err = runCmd(output, fi.Name, "convert", "--info", tmpFile, newFileName)
	case strings.HasPrefix(mime, "video/"):
		//err = runCmd(output, fi.Name, "ffmpeg", "--somethingcomplicated", tmpFile, newFileName)
	}

	// TODO(dylanj): Update DB to have some thumbnail crap if the commands above didn't fail

	if err != nil {
		return
	}
}

func runCmd(output *bytes.Buffer, name, cmd string, args ...string) error {
	command := exec.Command(cmd, args...)

	if output != nil {
		output.Reset()
		command.Stdout = output
	}

	if err := command.Run(); err != nil {
		log.Printf("postProcess: The command %q [%v] failed to run on %q (%v)", cmd, args, err)
		return err
	}

	return nil
}
