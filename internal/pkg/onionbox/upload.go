package onionbox

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html/template"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/Pallinder/go-randomdata"
	"github.com/ciehanski/onionbox/internal/pkg/onionbuffer"
	"github.com/ciehanski/onionbox/internal/pkg/templates"
)

func (ob *Onionbox) upload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		csrf, err := createCSRF()
		if err != nil {
			ob.Logf("Error creating CSRF token: %v", err)
			http.Error(w, "Error displaying web page, please try refreshing.", http.StatusInternalServerError)
			return
		}
		// Parse template
		t, err := template.New("upload").Parse(templates.UploadHTML)
		if err != nil {
			ob.Logf("Error loading template: %v", err)
			http.Error(w, "Error displaying web page, please try refreshing.", http.StatusInternalServerError)
			return
		}
		// Execute template
		if err := t.Execute(w, csrf); err != nil {
			ob.Logf("Error executing template: %v", err)
			http.Error(w, "Error displaying web page, please try refreshing.", http.StatusInternalServerError)
			return
		}
	case http.MethodPost:
		// Parse file(s) from form
		if err := r.ParseMultipartForm(ob.MaxFormMemory << 20); err != nil {
			ob.Logf("Error parsing files from form: %v", err)
			http.Error(w, "Error parsing files.", http.StatusInternalServerError)
			return
		}
		files := r.MultipartForm.File["files"]
		// A buffered channel that we can send work requests on.
		uploadQueue := make(chan *multipart.FileHeader, len(files))
		// Loop through files attached in form and offload to uploadQueue channel
		for _, fileHeader := range files {
			uploadQueue <- fileHeader
		}
		// Wait group for sync
		var wg sync.WaitGroup
		wg.Add(len(uploadQueue))
		// Create buffer for session in-memory zip file
		zipBuffer := new(bytes.Buffer)
		// Lock memory allotted to zipBuffer from being used in SWAP
		if err := syscall.Mlock(zipBuffer.Bytes()); err != nil {
			ob.Logf("Error mlocking allotted memory for zipBuffer: %v", err)
		}
		// Create new zip writer
		zWriter := zip.NewWriter(zipBuffer)
		// Write all files in queue to memory
		go func() {
			if err := onionbuffer.WriteFilesToBuffers(zWriter, uploadQueue, wg, ob.ChunkSize); err != nil {
				ob.Logf("Error writing files in queue to memory: %v", err)
				http.Error(w, "Error writing your files to memory.", http.StatusInternalServerError)
				return
			}
		}()
		// Wait for zip to be finished
		//wg.Wait()
		// Close uploadQueue channel after upload done
		close(uploadQueue)
		// Close zipwriter
		if err := zWriter.Close(); err != nil {
			ob.Logf("Error closing zip writer: %v", err)
		}
		// Create OnionBuffer object
		oBuffer := &onionbuffer.OnionBuffer{Name: strings.ToLower(randomdata.SillyName()), ChunkSize: ob.ChunkSize}
		// If password option was enabled
		if r.FormValue("password_enabled") == "on" {
			var err error
			pass := r.FormValue("password")
			oBuffer.Bytes, err = onionbuffer.Encrypt(zipBuffer.Bytes(), pass)
			if err != nil {
				ob.Logf("Error encrypting buffer: %v", err)
				http.Error(w, "Error encrypting buffer.", http.StatusInternalServerError)
				return
			}
			// Lock memory allotted to oBuffer from being used in SWAP
			if err := syscall.Mlock(oBuffer.Bytes); err != nil {
				ob.Logf("Error mlocking allotted memory for oBuffer: %v", err)
			}
			oBuffer.Encrypted = true
			chksm, err := oBuffer.GetChecksum()
			if err != nil {
				ob.Logf("Error getting checksum: %v", err)
				http.Error(w, "Error getting checksum.", http.StatusInternalServerError)
				return
			}
			oBuffer.Checksum = chksm
		} else {
			oBuffer.Bytes = zipBuffer.Bytes()
			// Lock memory allotted to oBuffer from being used in SWAP
			if err := syscall.Mlock(oBuffer.Bytes); err != nil {
				ob.Logf("Error mlocking allotted memory for oBuffer: %v", err)
			}
			// Get checksum
			chksm, err := oBuffer.GetChecksum()
			if err != nil {
				ob.Logf("Error getting checksum: %v", err)
				http.Error(w, "Error getting checksum.", http.StatusInternalServerError)
				return
			}
			oBuffer.Checksum = chksm
		}
		// If limit downloads was enabled
		if r.FormValue("limit_downloads") == "on" {
			form := r.FormValue("download_limit")
			limit, err := strconv.Atoi(form)
			if err != nil {
				ob.Logf("Error converting duration string into time.Duration: %v", err)
				http.Error(w, "Error getting expiration time.", http.StatusInternalServerError)
				return
			}
			oBuffer.DownloadLimit = int64(limit)
		}
		// if expiration was enabled
		if r.FormValue("expire") == "on" {
			expiration := fmt.Sprintf("%sm", r.FormValue("expiration_time"))
			if err := oBuffer.SetExpiration(expiration); err != nil {
				ob.Logf("Error parsing expiration time: %v", err)
				http.Error(w, "Error parsing expiration time.", http.StatusInternalServerError)
				return
			}
		}
		// Add OnionBuffer to Store
		if err := ob.Store.Add(oBuffer); err != nil {
			ob.Logf("Error adding file to Store: %v", err)
			http.Error(w, "Error adding file to Store.", http.StatusInternalServerError)
			return
		}
		// Destroy temp OnionBuffer
		if err := oBuffer.Destroy(); err != nil {
			ob.Logf("Error destroying temporary var for %s", oBuffer.Name)
		}
		// Write the zip's URL to client for sharing
		_, err := w.Write([]byte(fmt.Sprintf("Files uploaded. Please share this link with your recipients: http://%s.onion/%s",
			ob.OnionURL, oBuffer.Name)))
		if err != nil {
			ob.Logf("Error writing to client: %v", err)
			http.Error(w, "Error writing to client.", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Invalid HTTP Method.", http.StatusMethodNotAllowed)
		return
	}
}
