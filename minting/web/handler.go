package web

import (
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const (
	DEFAULT_EXPIRATION_SECONDS = 60        // 1 minute
	STATIC_EXPIRATION_SECONDS  = 24 * 3600 // 1 day

	STATIC_FILE_HANDLER_PREFIX = "/static/"
	JSON_HANDLER_PREFIX        = "/json/"
)

func GetWebHandler(webFolderPath, l1RPCEndpoint string, mintContract common.Address) http.Handler {
	mux := http.NewServeMux()

	log.Info("Serving web content", "path", webFolderPath)
	// This "default" file handler is for html files and other file content that we might be
	// changing with some regularity.
	fileHandler := http.FileServer(http.Dir(webFolderPath))
	chv := fmt.Sprint("max-age=", DEFAULT_EXPIRATION_SECONDS, ", public")
	defaultFileHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := statusResponseWriter{200, w}
		w.Header().Add("Cache-Control", chv)
		fileHandler.ServeHTTP(&writer, r)
		LogRequest(r, writer.status, nil, nil)
	})
	// The static web handler is for images and other content which we need not expire with any
	// regularity. It serves files out of the "static" subdirectory of the web folder.
	schv := fmt.Sprint("max-age=", STATIC_EXPIRATION_SECONDS, ", public")
	staticFileHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := statusResponseWriter{200, w}
		w.Header().Add("Cache-Control", schv)
		fileHandler.ServeHTTP(&writer, r)
		LogRequest(r, writer.status, nil, nil)
	})

	mux.Handle("/", defaultFileHandler)

	mux.Handle(STATIC_FILE_HANDLER_PREFIX, staticFileHandler)

	mux.Handle(JSON_HANDLER_PREFIX+"mint", newMintHandler(l1RPCEndpoint, mintContract))

	return mux
}

type statusResponseWriter struct {
	status int
	http.ResponseWriter
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
