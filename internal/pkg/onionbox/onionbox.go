package onionbox

import (
	"crypto/md5"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/ciehanski/onionbox/internal/pkg/onionstore"
)

const (
	formCSRF   = "token"
	cookieCSRF = "X-CSRF-Token"
)

type Onionbox struct {
	OnionURL      string
	RemotePort    int
	Logger        *log.Logger
	Store         *onionstore.OnionStore
	MaxFormMemory int64
	TorVersion3   bool
	ChunkSize     int64
	Debug         bool
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
func (ob Onionbox) Logf(format string, args ...interface{}) {
	ob.Logger.Printf(format, args...)
}

// Quit will Quit all stored buffers and exit onionbox.
func (ob Onionbox) Quit() {
	if err := ob.Store.DestroyAll(); err != nil {
		ob.Logf("Error destroying all buffers from Store: %v", err)
	}
	os.Exit(0)
}

// DisableCoreDumps disables core dumps on Unix systems.
// ref: https://github.com/awnumar/memguard/blob/master/memcall/memcall_unix.go
func (ob Onionbox) DisableCoreDumps() error {
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{Cur: 0, Max: 0}); err != nil {
		return err
	}
	return nil
}
