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
	cmap "github.com/orcaman/concurrent-map/v2"
)

// ReleaseWaitInterval defines the interval at which the release function will
// check if the upload has finished
const ReleaseWaitInterval = 100 * time.Millisecond

// WriteTimeout defines the duration of inactivity after which the write pipe
// will be closed
const WriteTimeout = 5 * time.Second

// blobUpload represents an ongoing upload of a blob file
type blobUpload struct {
	BlobFile       *BlobFile
	ongoingUploads cmap.ConcurrentMap[string, *blobUpload]

	mu                     sync.Mutex
	writePipe              *io.PipeWriter
	readPipe               *io.PipeReader
	uploadCtx              context.Context
	uploadCancel           context.CancelFunc
	writePipeLastByteIndex int64
	writeTimer             *time.Timer
}

// newBlobUpload initializes a new upload process for the blob file. It sets up
// the necessary components for streaming modified data and starts the upload.
// Once this has been called, and once f.UploadInProgress is true, then you can
// write to f.writePipe directly, and that data will get streamed to the server
// after whatever data is streamed initially.
func (f *BlobFile) newBlobUpload(
	off int64,
	data []byte,
	file *os.File,
) (*blobUpload, error) {
	log.Printf("Initializing new upload for %s", f.BlobListing.Key)

	// Create a new pipe for streaming data
	readPipe, writePipe := io.Pipe()

	// Create the modified content reader
	newData, err := createModifiedContentReader(file, off, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create modified content reader: %w", err)
	}

	// Create a new context for the upload operation
	uploadCtx, uploadCancel := context.WithCancel(context.Background())

	// Put the initial data into the read pipe, and then start the upload
	// process. The upload process will read out of this pipe.
	go io.Copy(writePipe, newData)

	// Create the upload object, then set it to be the new upload object
	newBlobUpload := &blobUpload{
		mu:                     sync.Mutex{},
		BlobFile:               f,
		writePipe:              writePipe,
		readPipe:               readPipe,
		uploadCtx:              uploadCtx,
		uploadCancel:           uploadCancel,
		writePipeLastByteIndex: off + int64(len(data)),
		writeTimer:             nil,
	}
	newBlobUpload.ResetWriteTimeout()

	// Start the upload
	f.myBlobs.ongoingUploads.Set(f.BlobListing.Key, newBlobUpload)
	go newBlobUpload.startUpload()

	return newBlobUpload, nil
}

// ResetWriteTimeout resets the write timeout timer This function should be
// called after each successful write operation
func (w *blobUpload) ResetWriteTimeout() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Stop the existing timer if it's running
	if w.writeTimer != nil {
		w.writeTimer.Stop()
	}

	// Start a new timer
	w.writeTimer = time.AfterFunc(WriteTimeout, func() {
		if w.writePipe != nil {
			log.Printf("Write timeout reached for %s, closing pipe", w.BlobFile.BlobListing.Key)
			w.writePipe.Close()
		}
	})
}

// AddBytesToUpload adds the provided bytes to the upload pipe. This function
// is used to add data to an ongoing upload operation.
func (w *blobUpload) AddBytesToUpload(data []byte) error {
	_, err := w.writePipe.Write(data)
	if err != nil {
		return err
	}
	w.writePipeLastByteIndex += int64(len(data))
	return nil
}

// createModifiedContentReader creates a MultiReader that combines the original file content
// with the new data to be inserted at the specified offset.
func createModifiedContentReader(file *os.File, off int64, data []byte) (io.Reader, error) {
	// Get file information to determine its size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Create readers for each part of the modified content
	readers := []io.Reader{
		// Data before the insertion point
		io.NewSectionReader(file, 0, off),

		// New data to be inserted
		bytes.NewReader(data),

		// Data after the insertion point (if any)
		io.NewSectionReader(file, off+int64(len(data)), fileSize-off-int64(len(data))),
	}

	// Combine all readers into a single MultiReader
	return io.MultiReader(readers...), nil
}

// startUpload initiates the upload process for the file
func (w *blobUpload) startUpload() {
	// Schedule cleanup for the upload
	defer func() {
		if w.writePipe != nil {
			w.writePipe.Close()
		}
		if w.readPipe != nil {
			w.readPipe.Close()
		}
		w.ongoingUploads.Remove(w.BlobFile.BlobListing.Key)
	}()

	// Upload the file to the server
	log.Printf("Uploading blob %s to server", w.BlobFile.BlobListing.Key)
	_, err := w.BlobFile.client.APIClient.RawRequest(
		w.uploadCtx,
		http.MethodPost,
		"/v1/blob/"+w.BlobFile.BlobListing.Key,
		w.readPipe,
	)

	// Report the status of the upload
	if err != nil {
		common.ReportError("Failed to upload file %v", err, w.BlobFile.BlobListing.Key)
	}
	log.Printf("Successfully uploaded file %s", w.BlobFile.BlobListing.Key)
}
