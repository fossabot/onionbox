package onionstore

import (
	"crypto/subtle"
	"runtime"
	"time"

	"github.com/ciehanski/onionbox/internal/pkg/onionbuffer"
	"golang.org/x/sys/unix"
)

type OnionStore struct {
	BufferFiles []*onionbuffer.OnionBuffer
}

// Used to create a nil store.
func NewStore() *OnionStore {
	return &OnionStore{BufferFiles: make([]*onionbuffer.OnionBuffer, 0)}
}

func (s *OnionStore) Add(b *onionbuffer.OnionBuffer) error {
	b.Lock()
	defer b.Unlock()
	s.BufferFiles = append(s.BufferFiles, b)
	// Advise the kernel not to dump. Ignore failure.
	// Unable to reference unix.MADV_DONTDUMP, raw value is 0x10 per:
	// https://godoc.org/golang.org/x/sys/unix
	unix.Madvise(b.Bytes, 0x10)
	// Lock bytes from SWAP
	if err := b.Mlock(); err != nil {
		return err
	}
	return nil
}

func (s *OnionStore) Get(bufName string) *onionbuffer.OnionBuffer {
	for _, f := range s.BufferFiles {
		if subtle.ConstantTimeCompare([]byte(f.Name), []byte(bufName)) == 1 {
			return f
		}
	}
	return nil
}

func (s *OnionStore) Destroy(b *onionbuffer.OnionBuffer) error {
	for i, f := range s.BufferFiles {
		if subtle.ConstantTimeCompare([]byte(f.Name), []byte(b.Name)) == 1 {
			if err := b.Destroy(); err != nil {
				return err
			}
			// Remove from s
			f.Lock()
			s.BufferFiles = append(s.BufferFiles[:i], s.BufferFiles[i+1:]...)
			// Free niled allotted memory for SWAP usage
			if err := f.Munlock(); err != nil {
				return err
			}
			f.Unlock()
		}
	}
	return nil
}

func (s *OnionStore) Exists(bufName string) bool {
	for _, f := range s.BufferFiles {
		if subtle.ConstantTimeCompare([]byte(f.Name), []byte(bufName)) == 1 {
			return true
		}
	}
	return false
}

func (s *OnionStore) DestroyAll() error {
	if s != nil {
		for i, f := range s.BufferFiles {
			if err := f.Destroy(); err != nil {
				return err
			}
			f.Lock()
			s.BufferFiles = append(s.BufferFiles[:i], s.BufferFiles[i+1:]...)
			if err := f.Munlock(); err != nil {
				return err
			}
			f.Unlock()
		}
		// TODO: needs further testing. DestroyAll should only be
		//  used when killing the application.
		runtime.GC()
	}
	return nil
}

// DestroyExpiredBuffers will indefinitely loop through the store and destroy
// expired OnionBuffers.
func (s *OnionStore) DestroyExpiredBuffers() error {
	for {
		select {
		case <-time.After(time.Second):
			if s != nil {
				for _, f := range s.BufferFiles {
					if f.Expire && f.ExpiresAt.Equal(time.Now()) || f.ExpiresAt.Before(time.Now()) {
						if err := s.Destroy(f); err != nil {
							return err
						}
					}
				}
			}
		}
	}
}
