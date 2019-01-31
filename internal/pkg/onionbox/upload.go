package onionbox

import (
	"archive/zip"
	"bytes"
	"crypto/subtle"
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

func (ob Onionbox) upload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		csrf, err := createCSRF() // Create CSRF to inject into template
		if err != nil {
			ob.Logf("Error creating CSRF token: %v", err)
			http.Error(w, "Error displaying web page, please try refreshing.", http.StatusInternalServerError)
			return
		}

		// Set CSRF cookie
		http.SetCookie(w, &http.Cookie{
			Name:  cookieCSRF,
			Value: csrf,
		})

		t, err := template.New("upload").Parse(templates.UploadHTML) // Parse template
		if err != nil {
			ob.Logf("Error parsing template: %v", err)
			http.Error(w, "Error displaying web page, please try refreshing.", http.StatusInternalServerError)
			return
		}

		if err := t.Execute(w, csrf); err != nil { // Execute template
			ob.Logf("Error executing template: %v", err)
			http.Error(w, "Error displaying web page, please try refreshing.", http.StatusInternalServerError)
			return
		}
	case http.MethodPost:
		if err := r.ParseMultipartForm(ob.MaxFormMemory << 20); err != nil { // Parse file(s) from form
			ob.Logf("Error parsing files from form: %v", err)
			http.Error(w, "Error parsing files.", http.StatusInternalServerError)
			return
		}

		// Check CSRF
		csrfForm := r.FormValue(formCSRF)
		csrfCookie, err := r.Cookie(cookieCSRF)
		if err != nil {
			ob.Logf("Error getting CSRF cookie: %v", err)
			http.Error(w, "Error getting CSRF.", http.StatusInternalServerError)
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrfForm), []byte(csrfCookie.Value)) == 0 {
			ob.Logf("Form CSRF and Cookie CSRF values do not match")
			http.Error(w, "Invalid CSRF value.", http.StatusUnauthorized)
			return
		}

		files := r.MultipartForm.File["files"]

		uploadQueue := make(chan *multipart.FileHeader, len(files)) // A channel that we can queue upload requests on

		var fileSizes int64
		for _, fileHeader := range files { // Loop through files attached in form and offload to uploadQueue channel
			fileSizes += fileHeader.Size
			uploadQueue <- fileHeader
		}

		tb, _ := onionbuffer.Allocate(int(fileSizes))
		zBuffer := bytes.NewBuffer(tb)
		//zBuffer := new(bytes.Buffer, tb) // Create buffer for session's in-memory zip file
		zWriter := zip.NewWriter(zBuffer) // Create new zip file

		wg := new(sync.WaitGroup) // Wait group for sync
		wg.Add(len(files))

		go func() { // Write all files in queue to memory
			if err := onionbuffer.WriteFilesToBuffers(zWriter, uploadQueue, wg, ob.ChunkSize); err != nil {
				ob.Logf("Error writing files in queue to memory: %v", err)
				http.Error(w, "Error writing your files to memory.", http.StatusInternalServerError)
				return
			}
		}()

		wg.Wait() // Wait for zip to be finished

		if err := zWriter.Close(); err != nil { // Close zipwriter
			ob.Logf("Error closing zip writer: %v", err)
		}

		if err := syscall.Mlock(zBuffer.Bytes()); err != nil { // Lock memory allotted to zBuffer from being used in SWAP
			ob.Logf("Error mlocking allotted memory for zBuffer: %v", err)
		}

		// Create OnionBuffer object
		oBuffer := &onionbuffer.OnionBuffer{Name: strings.ToLower(randomdata.SillyName()), ChunkSize: ob.ChunkSize}

		if r.FormValue("password_enabled") == "on" { // If password option was enabled
			var err error
			pass := r.FormValue("password")
			oBuffer.Bytes, err = onionbuffer.Encrypt(zBuffer.Bytes(), pass)
			if err != nil {
				ob.Logf("Error encrypting buffer: %v", err)
				http.Error(w, "Error encrypting buffer.", http.StatusInternalServerError)
				return
			}

			if err := syscall.Mlock(oBuffer.Bytes); err != nil { // Lock memory allotted to oBuffer from being used in SWAP
				ob.Logf("Error mlocking allotted memory for oBuffer: %v", err)
			}

			oBuffer.Encrypted = true
			chksm, err := oBuffer.GetChecksum()
			if err != nil {
				ob.Logf("Error getting buffer's checksum: %v", err)
				http.Error(w, "Error getting checksum.", http.StatusInternalServerError)
				return
			}

			oBuffer.Checksum = chksm

		} else { // If password option was NOT enabled
			//subtle.ConstantTimeCopy(1, oBuffer.Bytes, zBuffer.Bytes())
			oBuffer.Bytes = zBuffer.Bytes()

			if err := syscall.Mlock(oBuffer.Bytes); err != nil { // Lock memory allotted to oBuffer from being used in SWAP
				ob.Logf("Error mlocking allotted memory for oBuffer: %v", err)
			}

			chksm, err := oBuffer.GetChecksum() // Get checksum
			if err != nil {
				ob.Logf("Error getting checksum: %v", err)
				http.Error(w, "Error getting checksum.", http.StatusInternalServerError)
				return
			}

			oBuffer.Checksum = chksm
		}

		if r.FormValue("limit_downloads") == "on" { // If limit downloads was enabled
			form := r.FormValue("download_limit")
			limit, err := strconv.Atoi(form)
			if err != nil {
				ob.Logf("Error converting duration string into time.Duration: %v", err)
				http.Error(w, "Error getting expiration time.", http.StatusInternalServerError)
				return
			}
			oBuffer.DownloadLimit = int64(limit)
		}

		if r.FormValue("expire") == "on" { // if expiration was enabled
			expiration := fmt.Sprintf("%sm", r.FormValue("expiration_time"))
			if err := oBuffer.SetExpiration(expiration); err != nil {
				ob.Logf("Error parsing expiration time: %v", err)
				http.Error(w, "Error parsing expiration time.", http.StatusInternalServerError)
				return
			}
		}

		if err := ob.Store.Add(oBuffer); err != nil { // Add OnionBuffer to Store
			ob.Logf("Error adding file to store: %v", err)
			http.Error(w, "Error adding file to store.", http.StatusInternalServerError)
			return
		}

		if err := oBuffer.Destroy(); err != nil { // Destroy temp OnionBuffer
			ob.Logf("Error destroying temporary onionbuffer: %v", err)
		}

		// Write the zip's URL to client for sharing
		_, err = w.Write([]byte(fmt.Sprintf("Files uploaded. Please share this link with your recipients: http://%s.onion/%s",
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
