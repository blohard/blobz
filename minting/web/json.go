// JSON request handling
package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"

	"github.com/ethereum/go-ethereum/log"
)

const (
	JSON_REQUEST_SIZE_LIMIT = 200000 // size limit on the size of any JSON request in bytes
)

type jsonHandler interface {
	handle(r *http.Request, w http.ResponseWriter) (response interface{}, forLog *string, cacheControl *string)
}

// defaultHandler is an http.Handler that dispatches to the internal json handler.
type defaultHandler struct {
	j jsonHandler
}

func (h defaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	response, forLog, cacheControl := h.j.handle(r, w)
	if response == nil {
		log.Error("handler returned nil response", "url", r.URL)
		return
	}
	codeField := reflect.ValueOf(response).Elem().FieldByName("Code")
	var code int
	if codeField.IsValid() {
		code = int(codeField.Int())
	} else {
		log.Error("code field missing from json response", "response", response)
	}
	w.Header().Add("Content-Type", "application/json")
	if code == 0 {
		log.Error("Code not set, changing it to 1", "response", response)
		code = 1
	}
	if code != 1 {
		// Error! Make sure we return non-OK response code
		log.Warn("error response", "code", code)
		w.WriteHeader(500)
	} else {
		if cacheControl == nil {
			w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			w.Header().Add("Cache-Control", *cacheControl)
		}
	}
	json.NewEncoder(w).Encode(response)
	LogRequest(r, code, forLog, nil)
}

type Error struct {
	Code int
}

// use this reader for decoding JSON from the network to protect against oversized requests
func decodeReader(r io.Reader, n int64) io.Reader {
	return &safeReader{r, n}
}

type safeReader struct {
	reader    io.Reader
	remaining int64
}

func (l *safeReader) Read(p []byte) (n int, err error) {
	if l.remaining <= 0 {
		return 0, errors.New("request size limit exceeded")
	}
	if int64(len(p)) > l.remaining {
		p = p[0:l.remaining]
	}
	n, err = l.reader.Read(p)
	l.remaining -= int64(n)
	return
}

func decode(message interface{}, r io.ReadCloser) *Error {
	defer r.Close()
	if err := json.NewDecoder(decodeReader(r, JSON_REQUEST_SIZE_LIMIT)).Decode(&message); err != io.EOF && err != nil {
		log.Warn("json decoding failed", "error", err)
		return &Error{10}
	}
	return nil
}
