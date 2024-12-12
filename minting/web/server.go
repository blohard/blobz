package web

import (
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type ServeArgs struct {
	WebFolderPathname                  string // path to the folder containing the static web content
	FullChainPathname, PrivKeyPathname string // paths to the TLS key files

	L1RPCEndpoint string
	MintContract  common.Address

	Dev bool // true if this is a development server
}

// Serve starts web request dispatching.
func Serve(si *ServeArgs) error {
	handler := GetWebHandler(si.WebFolderPathname, si.L1RPCEndpoint, si.MintContract)
	if !si.Dev {
		// In prod (!si.Dev) mode we redirect any non-HTTPS request to its HTTPS equivalent.
		go func() {
			srv := getRedirectServer()
			if err := srv.ListenAndServe(); err != nil {
				log.Crit("ListenAndServe failed while redirecting requests", "error", err)
			}
		}()
		// web handler is used with HTTPS only in production mode
		tlsSrv := &http.Server{
			Addr:    ":8443",
			Handler: handler,
		}
		if err := tlsSrv.ListenAndServeTLS(si.FullChainPathname, si.PrivKeyPathname); err != nil {
			log.Crit("ListenAndServe for top level prod dispatch failed", "error", err)
			return err
		}
	} else {
		// In dev mode we serve web requests only over http so we don't have to worry about TLS
		// cert management.
		log.Warn("Starting localhost dev mode server")
		srv := &http.Server{
			Addr:    ":8080",
			Handler: handler,
		}
		if err := srv.ListenAndServe(); err != nil {
			log.Crit("ListenAndServe for top level dev dispatch failed", "error", err)
			return err
		}
	}
	return nil
}

func getRedirectServer() *http.Server {
	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Scheme = "https"
		r.URL.Host = SITE_HOSTNAME
		w.Header().Set("Location", r.URL.String())
		w.WriteHeader(MOVED_PERMANENTLY_HTTP_CODE)
	})
	return &http.Server{
		Addr:           ":8080",
		Handler:        redirectHandler,
		ReadTimeout:    20 * time.Second,
		WriteTimeout:   20 * time.Second,
		MaxHeaderBytes: 4096,
	}
}
