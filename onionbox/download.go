package onionbox

import (
	"fmt"
	"html/template"
	"net/http"
	"syscall"

	"onionbox/onionbuffer"
	"onionbox/templates"
)

func (ob *Onionbox) download(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		oBuffer := ob.Store.Get(r.Header.Get("filename"))
		if oBuffer.Encrypted {
			csrf, err := createCSRF()
			if err != nil {
				ob.Logf("Error creating CSRF token: %v", err)
				http.Error(w, "Error displaying web page, please try refreshing.", http.StatusInternalServerError)
				return
			}
			// Parse template
			t, err := template.New("download_encrypted").Parse(templates.DownloadHTML)
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
		} else {
			if oBuffer.DownloadLimit > 0 && oBuffer.Downloads >= oBuffer.DownloadLimit {
				if err := ob.Store.Destroy(oBuffer); err != nil {
					ob.Logf("Error deleting onion file from Store: %v", err)
				}
				ob.Logf("Download limit reached for %s", oBuffer.Name)
				http.Error(w, "Download limit reached.", http.StatusUnauthorized)
				return
			}
			// Validate checksum
			chksmValid, err := oBuffer.ValidateChecksum()
			if err != nil {
				ob.Logf("Error validating checksum: %v", err)
				http.Error(w, "Error validating checksum.", http.StatusInternalServerError)
				return
			}
			if !chksmValid {
				ob.Logf("Invalid checksum for file %s", oBuffer.Name)
				http.Error(w, "Invalid checksum.", http.StatusInternalServerError)
				return
			}
			// Increment files download count
			oBuffer.Downloads++
			// Check download amount
			if oBuffer.Downloads >= oBuffer.DownloadLimit {
				if err := oBuffer.Destroy(); err != nil {
					ob.Logf("Error destroying buffer %s: %v", oBuffer.Name, err)
				}
			}
			// Set headers for browser to initiate download
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", oBuffer.Name))
			// Write the zip bytes to the response for download
			_, err = w.Write(oBuffer.Bytes)
			if err != nil {
				ob.Logf("Error writing to client: %v", err)
				http.Error(w, "Error writing to client.", http.StatusInternalServerError)
				return
			}
		}
	// If buffer was password protected
	case http.MethodPost:
		oBuffer := ob.Store.Get(r.Header.Get("filename"))
		if oBuffer.DownloadLimit > 0 && oBuffer.Downloads >= oBuffer.DownloadLimit {
			if err := ob.Store.Destroy(oBuffer); err != nil {
				ob.Logf("Error deleting onion file from Store: %v", err)
			}
			ob.Logf("Download limit reached for %s", oBuffer.Name)
			http.Error(w, "Download limit reached.", http.StatusUnauthorized)
			return
		}
		// Validate checksum
		chksmValid, err := oBuffer.ValidateChecksum()
		if err != nil {
			ob.Logf("Error validating checksum: %v", err)
			http.Error(w, "Error validating checksum.", http.StatusInternalServerError)
			return
		}
		if !chksmValid {
			ob.Logf("Invalid checksum for file %s", oBuffer.Name)
			http.Error(w, "Invalid checksum.", http.StatusInternalServerError)
			return
		}
		// Get password and decrypt zip for download
		pass := r.FormValue("password")
		decryptedBytes, err := onionbuffer.Decrypt(oBuffer.Bytes, pass)
		if err != nil {
			ob.Logf("Error decrypting buffer: %v", err)
			http.Error(w, "Error decrypting buffer.", http.StatusInternalServerError)
			return
		}
		// Lock memory allotted to decryptedBytes from being used in SWAP
		if err := syscall.Mlock(decryptedBytes); err != nil {
			ob.Logf("Error mlocking allotted memory for decryptedBytes: %v", err)
		}
		// Increment files download count
		oBuffer.Downloads++
		// Check download amount
		if oBuffer.Downloads >= oBuffer.DownloadLimit {
			if err := ob.Store.Destroy(oBuffer); err != nil {
				ob.Logf("Error destroying buffer %s: %v", oBuffer.Name, err)
			}
		}
		// Set headers for browser to initiate download
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", oBuffer.Name))
		// Write the zip bytes to the response for download
		_, err = w.Write(decryptedBytes)
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
