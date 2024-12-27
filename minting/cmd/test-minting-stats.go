package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"github.com/blobz/minting"
)

var rpc = flag.String("l1rpc", "https://1rpc.io/sepolia", "url for the L1 rpc endpoint")

func init() {
	flag.Parse()
}

// simply fires up a minting stats and allows one to watch if it successfully updates via the debug
// log.
func main() {
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelDebug, true)))
	client, err := ethclient.Dial(*rpc)
	if err != nil {
		log.Crit("error dialing L1 rpc endpoint", "error", err)
	}
	ms := minting.NewMintingStats(client, common.HexToAddress("0x998Cd2C603F2c8E52788bc7Ee9C39abFd8Abe131"))
	for {
		fmt.Printf("Minting stats: %+v\n", ms.GetStats())
		time.Sleep(2 * time.Second)
	}
}
