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
	// Lock memory allotted to chunk from being used in SWAP
	if err := syscall.Mlock(chunk); err != nil {
		return err
	}
	bufFile, _ := zWriter.Create(b.Name)
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
	if err := syscall.Munlock(b.Bytes); err != nil {
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

func WriteFilesToBuffers(w *zip.Writer, uploadQueue <-chan *multipart.FileHeader, wg sync.WaitGroup, chunkSize int64) error {
	for {
		select {
		case fileHeader := <-uploadQueue:
			// Open uploaded file
			file, err := fileHeader.Open()
			if err != nil {
				return err
			}
			// Create file in zip with same name
			zBuffer, err := w.Create(fileHeader.Filename)
			if err != nil {
				return err
			}
			// Read uploaded file
			if err := writeBytesByChunk(file, zBuffer, chunkSize); err != nil {
				return err
			}
			// Flush zipwriter to write compressed bytes to buffer
			// before moving onto the next file
			if err := w.Flush(); err != nil {
				return err
			}
			wg.Done()
		}
	}
}

func writeBytesByChunk(file io.Reader, bufWriter io.Writer, chunkSize int64) error {
	// Read uploaded file
	var count int
	var err error
	reader := bufio.NewReader(file)
	chunk := make([]byte, chunkSize)
	// Lock memory allotted to chunk from being used in SWAP
	if err := syscall.Mlock(chunk); err != nil {
		return err
	}
	for {
		if count, err = reader.Read(chunk); err != nil {
			break
		}
		_, err := bufWriter.Write(chunk[:count])
		if err != nil {
			return err
		}
	}
	if err != io.EOF {
		return err
	} else {
		err = nil
	}
	return nil
}
