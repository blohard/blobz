package main

import (
	"flag"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	. "github.com/blobz/minting/web"
)

var web = flag.String("web", "", "pathname of the web folder")

// Starts request dispatching in dev mode

func init() {
	flag.Parse()
}

func main() {
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelDebug, true)))

	si := &ServeArgs{
		WebFolderPathname: *web,
		FullChainPathname: "",
		PrivKeyPathname:   "",
		L1RPCEndpoint:     "https://1rpc.io/sepolia",
		MintContract:      common.HexToAddress("0x998Cd2C603F2c8E52788bc7Ee9C39abFd8Abe131"),
		Dev:               true,
	}
	if *web == "" {
		log.Crit("Must specify -web=[pathname]")
	}
	if err := Serve(si); err != nil {
		log.Crit("serving failed :(", "error", err)
	}
}
