package fuse

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/404wolf/valfs/common"
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
)

// WriteTimeout defines the duration of inactivity after which the write pipe
// will be closed
const WriteTimeout = 15 * time.Second

// UploadTimeout is the amount of time once the upload has been fully queued
// that we are allowed to wait
const UploadTimeout = 15 * time.Second

// BufferSize defines the size of the buffer used for uploading
const BufferSize = 24 * 1024 * 1024 // 24MB

// BlobUpload represents an ongoing upload of a blob file
type BlobUpload struct {
	BlobFile *BlobFile

	ongoing        bool
	pipeWriter     *nio.PipeWriter
	pipeReader     *nio.PipeReader
	uploadCtx      context.Context
	uploadCancel   context.CancelFunc
	uploadLock     *sync.Mutex
	endOfPipeIndex int64
	writeTimer     *time.Timer
	writeQueueLen  int64
}

// Ongoing returns whether the upload is currently ongoing
func (f *BlobUpload) Ongoing() bool {
	return f.ongoing
}

// GetWriteQueueLen returns the current length of the write queue
func (w *BlobUpload) GetWriteQueueLen() int64 {
	return w.writeQueueLen
}

// DirectlyAfterPipeEnd checks if the provided offset is directly after the current pipe end
func (f *BlobUpload) DirectlyAfterPipeEnd(off int64) bool {
	return off == f.endOfPipeIndex
}

// Start initiates the upload process
func (w *BlobUpload) Start() error {
	log.Printf("Starting upload for %s", w.BlobFile.Meta.Key)

	// Create a new buffered pipe
	buf := buffer.New(BufferSize)
	pipeReader, pipeWriter := nio.Pipe(buf)

	// Create a new context for the upload operation
	uploadCtx, uploadCancel := context.WithCancel(context.Background())

	w.pipeReader = pipeReader
	w.pipeWriter = pipeWriter
	w.uploadCtx = uploadCtx
	w.uploadCancel = uploadCancel
	w.ongoing = true
	w.uploadLock = &sync.Mutex{}

	// Start the upload process
	go func() {
		defer w.finishUpload()
		w.uploadLock.Lock()

		log.Printf("Uploading blob %s to server", w.BlobFile.Meta.Key)
		_, err := w.BlobFile.myBlobs.client.APIClient.RawRequest(
			w.uploadCtx,
			http.MethodPost,
			"/v1/blob/"+w.BlobFile.Meta.Key,
			w.pipeReader,
		)

		if err != nil {
			common.ReportError("Failed to upload file %v", err, w.BlobFile.Meta.Key)
		} else {
			log.Printf("Successfully uploaded file %s", w.BlobFile.Meta.Key)
		}
	}()

	return nil
}

// Write handles writing data at the specified offset
func (w *BlobUpload) Write(off int64, data []byte) error {
	if !w.ongoing {
		return fmt.Errorf("upload not ongoing")
	}

	// If the write is directly after the current pipe end, we can just append
	if w.DirectlyAfterPipeEnd(off) {
		return w.AddBytesToUpload(data)
	}

	// Otherwise, we need to create a new upload with the modified content
	file, err := os.Open(w.BlobFile.Meta.Key)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	modifiedContent, err := createModifiedContentReader(file, off, data)
	if err != nil {
		return fmt.Errorf("failed to create modified content: %w", err)
	}

	// Copy the modified content to the pipe
	go io.Copy(w.pipeWriter, modifiedContent)

	w.endOfPipeIndex = off + int64(len(data))
	w.writeQueueLen += int64(len(data))
	w.ResetWriteTimeout()

	return nil
}

// ResetWriteTimeout resets the write timeout timer
func (w *BlobUpload) ResetWriteTimeout() {
	if w.writeTimer != nil {
		w.writeTimer.Stop()
	}

	w.writeTimer = time.AfterFunc(WriteTimeout, func() {
		if w.pipeWriter != nil {
			log.Printf("Write timeout reached for %s, closing pipe", w.BlobFile.Meta.Key)
			w.pipeWriter.Close()
		}
	})
}

// Cancel cancels the upload operation and cleans up resources
func (w *BlobUpload) Cancel() {
	w.ongoing = false
	if w.uploadCancel != nil {
		w.uploadCancel()
	}
	if w.writeTimer != nil {
		w.writeTimer.Stop()
	}
	if w.pipeWriter != nil {
		w.pipeWriter.Close()
	}
	if w.pipeReader != nil {
		w.pipeReader.Close()
	}
}

// Finish waits for the upload to complete or timeout
func (w *BlobUpload) Finish() {
	if w.writeTimer != nil {
		w.writeTimer.Stop()
	}

	if w.pipeWriter != nil {
		w.pipeWriter.Close()
	}

	done := make(chan struct{})
	go func() {
		w.uploadLock.Lock()
		w.uploadLock.Unlock()
		close(done)
	}()

	select {
	case <-done:
		// Upload finished
	case <-time.After(UploadTimeout):
		log.Printf("Timeout waiting for upload of %s to finish", w.BlobFile.Meta.Key)
	}
}

// AddBytesToUpload adds bytes to the upload pipe
func (w *BlobUpload) AddBytesToUpload(data []byte) error {
	w.ResetWriteTimeout()
	_, err := w.pipeWriter.Write(data)
	if err != nil {
		return err
	}
	w.endOfPipeIndex += int64(len(data))
	w.writeQueueLen += int64(len(data))
	return nil
}

// createModifiedContentReader creates a MultiReader that combines the original file content
// with the new data to be inserted at the specified offset
func createModifiedContentReader(file *os.File, off int64, data []byte) (io.Reader, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	readers := []io.Reader{
		io.NewSectionReader(file, 0, off),
		bytes.NewReader(data),
		io.NewSectionReader(file, off+int64(len(data)), fileSize-off-int64(len(data))),
	}

	return io.MultiReader(readers...), nil
}

// finishUpload handles cleanup after upload completion
func (w *BlobUpload) finishUpload() {
	if w.pipeWriter != nil {
		w.pipeWriter.Close()
	}
	if w.pipeReader != nil {
		w.pipeReader.Close()
	}
	w.ongoing = false
	w.uploadLock.Unlock()
}
