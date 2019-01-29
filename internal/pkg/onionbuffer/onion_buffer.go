package onionbuffer

import (
	"archive/zip"
	"bufio"
	"bytes"
	"io"
	"mime/multipart"
	"sync"
	"syscall"
	"time"
)

// OnionBuffer struct
type OnionBuffer struct {
	sync.RWMutex
	Name             string
	Bytes            []byte
	Checksum         string
	ChunkSize        int64
	Encrypted        bool
	Downloads        int64
	DownloadLimit    int64
	DownloadsLimited bool
	Expire           bool
	ExpiresAt        time.Time
}

// Destroy is mostly used to destroy temporary OnionBuffer objects after they
// have been copied to the store or to remove an individual OnionBuffer
// from the store.
func (b *OnionBuffer) Destroy() error {
	b.Lock()
	defer b.Unlock()
	var err error
	buffer := bytes.NewBuffer(b.Bytes)
	zWriter := zip.NewWriter(buffer)
	reader := bufio.NewReader(bytes.NewReader(b.Bytes))
	chunk := make([]byte, 1)
	bufFile, err := zWriter.Create(b.Name)
	if err != nil {
		return err
	}
	for {
		if _, err = reader.Read(chunk); err != nil {
			break
		}
		_, err := bufFile.Write([]byte("0"))
		if err != nil {
			return err
		}
	}
	if err != io.EOF {
		return err
	} else {
		err = nil
	}
	if err := syscall.Munlock(b.Bytes); err != nil { // Unlock memory allotted to chunk to be used for SWAP
		return err
	}
	return nil
}

// IsExpired is used to check if an OnionBuffer is expired or not.
func (b *OnionBuffer) IsExpired() bool {
	b.RLock()
	defer b.RUnlock()
	if b.Expire {
		if b.ExpiresAt.After(time.Now()) {
			return false
		}
		return true
	}
	return false
}

// SetExpiration is used to set the expiration duration of the OnionBuffer.
func (b *OnionBuffer) SetExpiration(expiration string) error {
	b.Lock()
	defer b.Unlock()
	t, err := time.ParseDuration(expiration)
	if err != nil {
		return err
	}
	b.Expire = true
	b.ExpiresAt = time.Now().Add(t)
	return nil
}

func WriteFilesToBuffers(w *zip.Writer, files []*multipart.FileHeader, wg *sync.WaitGroup, chunkSize int64) error {
	for _, fileHeader := range files {
		file, err := fileHeader.Open() // Open uploaded file
		if err != nil {
			return err
		}

		zBuffer, err := w.Create(fileHeader.Filename) // Create file in zip with same name
		if err != nil {
			return err
		}

		if err := writeBytesByChunk(file, zBuffer, chunkSize); err != nil { // Write file in chunks to zBuffer
			return err
		}
		// Flush zipwriter to write compressed bytes to buffer
		// before moving onto the next file
		if err := w.Flush(); err != nil {
			return err
		}
		wg.Done() // Signal to work group this file is done uploading
	}
	return nil
}

func writeBytesByChunk(file io.Reader, bufWriter io.Writer, chunkSize int64) error {
	var count int
	var err error
	reader := bufio.NewReader(file) // Read uploaded file
	chunk := make([]byte, chunkSize)
	for {
		if count, err = reader.Read(chunk); err != nil { // Read the specific chunk of uploaded file
			break
		}
		_, err := bufWriter.Write(chunk[:count]) // Write the specific chunk to the new zip entry
		if err != nil {
			return err
		}
	}
	if err != io.EOF { // If not EOF, return the err
		return err
	} else { // if EOF, do not return an error
		err = nil
	}
	if err := syscall.Mlock(chunk); err != nil { // Lock memory allotted to chunk from being used in SWAP
		return err
	}
	return nil
}
