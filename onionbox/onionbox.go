package onionbox

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"onionbox/onionbuffer"
)

type Onionbox struct {
	Version       string
	Port          int
	Debug         bool
	Logger        *log.Logger
	Store         *onionbuffer.OnionStore
	MaxFormMemory int64
	TorVersion3   bool
	OnionURL      string
	ChunkSize     int64
}

// createCSRF creates a simple md5 hash which I use to avoid CSRF attacks when presenting HTML forms
func createCSRF() (string, error) {
	hasher := md5.New()
	_, err := io.WriteString(hasher, strconv.FormatInt(time.Now().Unix(), 10))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// Logf is a helper function which will utilize the Logger from ob
// to print formatted logs.
func (ob *Onionbox) Logf(format string, args ...interface{}) {
	ob.Logger.Printf(format, args...)
}

// Quit will Quit all stored buffers and exit onionbox.
func (ob *Onionbox) Quit() {
	if err := ob.Store.DestroyAll(); err != nil {
		ob.Logf("Error destroying all buffers from Store: %v", err)
	}
	os.Exit(0)
}
