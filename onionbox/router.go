package onionbox

import (
	"net/http"
	"regexp"
)

var downloadURLreg = regexp.MustCompile(`((?:[a-z][a-z]+))`)

func (ob *Onionbox) Router(w http.ResponseWriter, r *http.Request) {
	// If base URL, send to upload handler
	if r.URL.Path == "/" {
		ob.upload(w, r)
	} else if matches := downloadURLreg.FindStringSubmatch(r.URL.Path); matches != nil {
		if ob.Store != nil {
			if ob.Store.Exists(r.URL.Path[1:]) {
				// If the path requested is a valid onionbuffer in the store,
				// add a request header with the filename and send the user to the download handler
				r.Header.Set("filename", r.URL.Path[1:])
				ob.download(w, r)
			}
		} else {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
	} else {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
}
