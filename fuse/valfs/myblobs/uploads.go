package fuse

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/404wolf/valfs/common"
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
)

// ReleaseWaitInterval defines the interval at which the release function will
// check if the upload has finished
const ReleaseWaitInterval = 100 * time.Millisecond

// WriteTimeout defines the duration of inactivity after which the write pipe
// will be closed
const WriteTimeout = 5 * time.Second

// BufferSize defines the size of the buffer used for uploading
const BufferSize = 10 * 1024 * 1024 // 5MB

// BlobUpload represents an ongoing upload of a blob file
type BlobUpload struct {
	BlobFile *BlobFile

	pipeWriter     *nio.PipeWriter
	pipeReader     *nio.PipeReader
	uploadCtx      context.Context
	uploadCancel   context.CancelFunc
	endOfPipeIndex int64
	writeTimer     *time.Timer
	writeQueueLen  int64
}

// GetWriteQueueLen returns the current length of the write queue
func (w *BlobUpload) GetWriteQueueLen() int64 {
	return w.writeQueueLen
}

// EndOfPipeIndex is the offset of the last byte in the pipe relative to where
// the byte is in the file. If you are writing more data to the pipe directly,
// you want to make sure that your data comes directly after the
// EndOfPipeIndex.
func (f *BlobUpload) DirectlyAfterPipeEnd(off int64) bool {
	return off == f.endOfPipeIndex
}

// NewBlobUpload initializes a new upload process for the blob file. It sets up
// the necessary components for streaming modified data and starts the upload.
// Once this has been called, and once f.UploadInProgress is true, then you can
// write to f.pipeWriter directly, and that data will get streamed to the server
// after whatever data is streamed initially.
func (f *BlobFile) NewBlobUpload(
	off int64,
	data []byte,
	file *os.File,
) (*BlobUpload, error) {
	log.Printf("Initializing new upload for %s", f.BlobListing.Key)

	// Create a new buffered pipe
	buf := buffer.New(BufferSize)
	pipeReader, pipeWriter := nio.Pipe(buf)

	// Create the modified content reader
	newData, err := createModifiedContentReader(file, off, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create modified content reader: %w", err)
	}

	// Create a new context for the upload operation
	uploadCtx, uploadCancel := context.WithCancel(context.Background())

	// Put the initial data into the read pipe, and then start the upload
	// process. The upload process will read out of this pipe.
	go io.Copy(pipeWriter, newData)

	// Start the upload
	newBlobUpload := &BlobUpload{
		BlobFile: f,

		pipeWriter:     pipeWriter,
		pipeReader:     pipeReader,
		uploadCtx:      uploadCtx,
		uploadCancel:   uploadCancel,
		endOfPipeIndex: off + int64(len(data)),
		writeTimer:     nil,
		writeQueueLen:  int64(len(data)), // Length of initial data
	}
	f.myBlobs.ongoingUploads.Set(f.BlobListing.Key, newBlobUpload)

	// Create the upload object, then set it to be the new upload object
	go newBlobUpload.startUpload()

	return newBlobUpload, nil
}

// ResetWriteTimeout resets the write timeout timer This function should be
// called after each successful write operation
func (w *BlobUpload) ResetWriteTimeout() {
	// Stop the existing timer if it's running
	if w.writeTimer != nil {
		w.writeTimer.Stop()
	}

	// Start a new timer
	w.writeTimer = time.AfterFunc(WriteTimeout, func() {
		if w.pipeWriter != nil {
			log.Printf("Write timeout reached for %s, closing pipe", w.BlobFile.BlobListing.Key)
			w.pipeWriter.Close()
		} else {
			w.writeTimer.Stop() // Kill the old timer
		}
	})
}

// CancelUpload cancels the upload operation and cleans up the resources
func (w *BlobUpload) CancelUpload() {
	if w.BlobFile.myBlobs.ongoingUploads.Has(w.BlobFile.BlobListing.Key) {
		w.BlobFile.myBlobs.ongoingUploads.Remove(w.BlobFile.BlobListing.Key)
	}

	if w.uploadCancel != nil {
		w.uploadCancel()
	}
}

// AddBytesToUpload adds the provided bytes to the upload pipe. This function
// is used to add data to an ongoing upload operation.
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

func (w *BlobUpload) WaitForUpload() {
	for w.BlobFile.myBlobs.ongoingUploads.Has(w.BlobFile.BlobListing.Key) {
		w.ResetWriteTimeout()
		time.Sleep(ReleaseWaitInterval)
	}
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

// finishUpload gracefully finishes the upload process, allowing the data that
// has been streamed so far to make it to valtown, and then cleans up
// resources and does basic bookeeping.
func (w *BlobUpload) finishUpload() {
	// Close the write pipe to signal the end of the upload
	if w.pipeWriter != nil {
		w.pipeWriter.Close()
	}

	// Remove from the uploads map
	if w.BlobFile.myBlobs.ongoingUploads.Has(w.BlobFile.BlobListing.Key) {
		w.BlobFile.myBlobs.ongoingUploads.Remove(w.BlobFile.BlobListing.Key)
	}
}

// startUpload initiates the upload process for the file
func (w *BlobUpload) startUpload() {
	defer w.finishUpload()

	// Upload the file to the server
	log.Printf("Uploading blob %s to server", w.BlobFile.BlobListing.Key)
	_, err := w.BlobFile.myBlobs.client.APIClient.RawRequest(
		w.uploadCtx,
		http.MethodPost,
		"/v1/blob/"+w.BlobFile.BlobListing.Key,
		w.pipeReader,
	)

	// Report the status of the upload
	if err != nil {
		common.ReportError("Failed to upload file %v", err, w.BlobFile.BlobListing.Key)
	} else {
		log.Printf("Successfully uploaded file %s", w.BlobFile.BlobListing.Key)
	}
}
