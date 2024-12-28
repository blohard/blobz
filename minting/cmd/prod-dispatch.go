package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	. "github.com/blobz/minting/web"
)

var app = flag.String("app", "", "pathname of the application root folder")

// Starts request dispatching in prod mode

func init() {
	flag.Parse()
}

func main() {
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelDebug, true)))
	if *app == "" {
		log.Crit("Must specify -app=[app folder]")
	}
	si := &ServeArgs{
		WebFolderPathname: filepath.Join(*app, "html_root"),
		L1RPCEndpoint:     "http://localhost:8545",
		FullChainPathname: filepath.Join(*app, "data/secret/fullchain.pem"),
		PrivKeyPathname:   filepath.Join(*app, "data/secret/privkey.pem"),
		MintContract:      common.HexToAddress("0x998Cd2C603F2c8E52788bc7Ee9C39abFd8Abe131"),
		Dev:               false,
	}
	if err := SetWeblogOutput(filepath.Join(*app, "log/httplog.txt")); err != nil {
		log.Crit("couldn't set log output", "error", err)
	}
	if err := Serve(si); err != nil {
		log.Crit("serving failed :(", "error", err)
	}
}
