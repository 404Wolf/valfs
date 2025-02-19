package valfs

import (
	"context"
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
	BlobFile *BlobFile // Reference to the blob file being uploaded

	ongoing        bool               // Flag indicating if upload is in progress
	pipeWriter     *nio.PipeWriter    // Writer end of the upload pipe
	pipeReader     *nio.PipeReader    // Reader end of the upload pipe
	uploadCtx      context.Context    // Context for controlling the upload operation
	uploadCancel   context.CancelFunc // Function to cancel the upload context
	uploadWg       *sync.WaitGroup    // WaitGroup for tracking upload completion
	endOfPipeIndex int64              // Current end position of the pipe
	writeTimer     *time.Timer        // Timer for write timeout
	writeQueueLen  int64              // Length of pending writes in the queue
	client         *common.Client
}

// setup initializes all necessary fields for the BlobUpload
func (w *BlobUpload) setup() {
	log.Printf("Setting up upload components for %s", w.BlobFile.Meta.Key)

	// Initialize WaitGroup if nil
	if w.uploadWg == nil {
		w.uploadWg = &sync.WaitGroup{}
	}

	// Create a new buffered pipe
	buf := buffer.New(BufferSize)
	pipeReader, pipeWriter := nio.Pipe(buf)

	// Create a new context for the upload operation
	uploadCtx, uploadCancel := context.WithCancel(context.Background())

	// Set all fields
	w.pipeReader = pipeReader
	w.pipeWriter = pipeWriter
	w.uploadCtx = uploadCtx
	w.uploadCancel = uploadCancel
	w.ongoing = true
	w.endOfPipeIndex = 0
	w.writeQueueLen = 0
	w.writeTimer = nil

	log.Printf("Completed setup of upload components for %s", w.BlobFile.Meta.Key)
}

// Ongoing returns whether the upload is currently ongoing
func (f *BlobUpload) Ongoing() bool {
	return f.ongoing
}

// GetWriteQueueLen returns the current length of the write queue
func (w *BlobUpload) GetWriteQueueLen() int64 {
	log.Printf("Current write queue length for %s: %d bytes", w.BlobFile.Meta.Key, w.writeQueueLen)
	return w.writeQueueLen
}

// DirectlyAfterPipeEnd checks if the provided offset is directly after the current pipe end
func (f *BlobUpload) DirectlyAfterPipeEnd(off int64) bool {
	isDirectly := off == f.endOfPipeIndex
	log.Printf("Checking if offset %d is directly after pipe end %d for %s: %v",
		off, f.endOfPipeIndex, f.BlobFile.Meta.Key, isDirectly)
	return isDirectly
}

// Start initiates the upload process
func (w *BlobUpload) Start() error {
	// First cancel any existing upload to ensure clean state
	w.Cancel()
	if w.uploadWg != nil {
		w.uploadWg.Wait()
	}

	log.Printf("Starting upload process for %s (Buffer size: %d bytes)",
		w.BlobFile.Meta.Key, BufferSize)

	// Setup new upload state
	w.setup()

	// Start the upload process
	go w.startUpload()

	return nil
}

func (w *BlobUpload) startUpload() {
	defer w.finishUpload()

	log.Printf("Beginning upload request for blob %s to server", w.BlobFile.Meta.Key)
	startTime := time.Now()

	_, err := w.BlobFile.myBlobs.client.APIClient.RawRequest(
		w.uploadCtx,
		http.MethodPost,
		"/v1/blob/"+w.BlobFile.Meta.Key,
		w.pipeReader,
	)

	duration := time.Since(startTime)
	if err != nil {
		common.Logger.Error("Failed to upload file %v after %v", err, w.BlobFile.Meta.Key, duration)
	} else {
		log.Printf("Successfully uploaded file %s in %v", w.BlobFile.Meta.Key, duration)
	}
}

// Write handles writing data at the specified offset
func (w *BlobUpload) Write(off int64, data []byte, file *os.File) error {
	if !w.Ongoing() {
		log.Printf("Upload is not ongoing for %s, starting upload", w.BlobFile.Meta.Key)
		w.Start()
	}
	w.ResetWriteTimeout()

	log.Printf("Writing %d bytes at offset %d for %s", len(data), off, w.BlobFile.Meta.Key)

	// // If the write is directly after the current pipe end, we can just append
	// if w.DirectlyAfterPipeEnd(off) {
	// 	log.Printf("Performing sequential write for %s", w.BlobFile.Meta.Key)
	// 	err := w.AddBytesToUpload(data)
	// 	if err == nil {
	// 		return nil
	// 	}
	// 	log.Printf("Sequential write failed for %s: %v", w.BlobFile.Meta.Key, err)
	// }

	// Otherwise we have to totally restart the upload
	log.Printf("Non-sequential write detected for %s, restarting upload", w.BlobFile.Meta.Key)
	w.Cancel()
	w.Start()

	go func() {
		file.Seek(0, 0)
		copied, err := io.Copy(w.pipeWriter, file)
		if err != nil {
			log.Printf("Error during file copy for %s: %v", w.BlobFile.Meta.Key, err)
		} else {
			log.Printf("Copied %d bytes for %s", copied, w.BlobFile.Meta.Key)
		}
	}()

	return nil
}

// ResetWriteTimeout resets the write timeout timer
func (w *BlobUpload) ResetWriteTimeout() {
	if w.writeTimer != nil {
		w.writeTimer.Stop()
		log.Printf("Stopped existing write timer for %s", w.BlobFile.Meta.Key)
	}

	w.writeTimer = time.AfterFunc(WriteTimeout, func() {
		if w.pipeWriter != nil {
			log.Printf("Write timeout (%v) reached for %s, closing pipe", WriteTimeout, w.BlobFile.Meta.Key)
			w.pipeWriter.Close()
		}
	})
	log.Printf("Reset write timeout timer for %s", w.BlobFile.Meta.Key)
}

// Cancel cancels the upload operation and cleans up resources
func (w *BlobUpload) Cancel() {
	log.Printf("Cancelling upload for %s", w.BlobFile.Meta.Key)

	w.ongoing = false
	if w.uploadCancel != nil {
		w.uploadCancel()
		log.Printf("Cancelled upload context for %s", w.BlobFile.Meta.Key)
	}
	if w.writeTimer != nil {
		w.writeTimer.Stop()
		log.Printf("Stopped write timer for %s", w.BlobFile.Meta.Key)
	}
	if w.pipeReader != nil {
		w.pipeReader.Close()
		log.Printf("Closed pipe reader for %s", w.BlobFile.Meta.Key)
	}
	if w.pipeWriter != nil {
		w.pipeWriter.Close()
		log.Printf("Closed pipe writer for %s", w.BlobFile.Meta.Key)
	}

	// Reset all fields to nil/zero values
	w.pipeReader = nil
	w.pipeWriter = nil
	w.uploadCtx = nil
	w.uploadCancel = nil
	w.writeTimer = nil
	w.endOfPipeIndex = 0
	w.writeQueueLen = 0

	log.Printf("Upload cancelled and resources cleaned up for %s", w.BlobFile.Meta.Key)
}

// Finish waits for the upload to complete or timeout
func (w *BlobUpload) Finish() {
	log.Printf("Beginning finish process for %s", w.BlobFile.Meta.Key)

	if w.writeTimer != nil {
		w.writeTimer.Stop()
		log.Printf("Stopped write timer for %s", w.BlobFile.Meta.Key)
	}

	if w.pipeWriter != nil {
		w.pipeWriter.Close()
		log.Printf("Closed pipe writer for %s", w.BlobFile.Meta.Key)
	}

	// Create a channel to handle timeout
	done := make(chan struct{})
	go func() {
		if w.uploadWg != nil {
			log.Printf("Waiting for upload to complete for %s", w.BlobFile.Meta.Key)
			w.uploadWg.Wait() // Wait for upload to complete
		}
		close(done)
	}()

	select {
	case <-done:
		log.Printf("Upload finished successfully for %s", w.BlobFile.Meta.Key)
	case <-time.After(UploadTimeout):
		log.Printf("Timeout (%v) reached waiting for upload of %s to finish", UploadTimeout, w.BlobFile.Meta.Key)
		w.ongoing = false
	}
}

// AddBytesToUpload adds bytes to the upload pipe
func (w *BlobUpload) AddBytesToUpload(data []byte) error {
	log.Printf("Adding %d bytes to upload for %s", len(data), w.BlobFile.Meta.Key)
	w.ResetWriteTimeout()

	w.endOfPipeIndex += int64(len(data))
	w.writeQueueLen += int64(len(data))

	go w.pipeWriter.Write(data)

	log.Printf("Successfully added bytes to upload for %s. New pipe end: %d, Queue length: %d",
		w.BlobFile.Meta.Key, w.endOfPipeIndex, w.writeQueueLen)
	return nil
}

// finishUpload handles cleanup after upload completion
func (w *BlobUpload) finishUpload() {
	if w.uploadWg != nil {
		w.uploadWg.Add(1)
	}
	log.Printf("Finishing upload for %s", w.BlobFile.Meta.Key)

	if w.pipeWriter != nil {
		w.pipeWriter.Close()
		log.Printf("Closed pipe writer for %s", w.BlobFile.Meta.Key)
	}
	if w.pipeReader != nil {
		w.pipeReader.Close()
		log.Printf("Closed pipe reader for %s", w.BlobFile.Meta.Key)
	}

	w.ongoing = false
	if w.uploadWg != nil {
		w.uploadWg.Done()
	}
	log.Printf("Upload finished and resources cleaned up for %s", w.BlobFile.Meta.Key)
}
