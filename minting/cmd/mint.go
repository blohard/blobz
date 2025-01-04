// mint is a simple command-line script for minting BLOBZ from the command line without exposing
// your private key outside your machine. If successful, it performs exactly one invocation of the
// mint() contract, waits for the receipt, then prints the txid of the successful transaction.
//
// BUILDING:
//
//	# cd to the directory containing this file, then:
//	go build mint.go
//
// RUNNING THE SCRIPT:
//
//	./mint -key=[your private key here]"
//
// You can also change the default RPC provider by specifying the -rpc flag. Note that most public
// RPC endpoints do NOT allow submitting blobs, so choose your provider carefully.
package main

import (
	"context"
	"flag"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/holiman/uint256"

	"github.com/blobz/minting"
	"github.com/blobz/minting/web"
)

var (
	key = flag.String("key", "", "private key string")
	rpc = flag.String("rpc", "https://eth-mainnet-public.unifra.io", "Ethereum L1 RPC endpoint")

	mintContract  = common.HexToAddress("0x5a3322F3A365465413d7302388E55717685a926C")
	clientTimeout = 15 * time.Second
)

func init() {
	flag.Parse()
}

func main() {
	mint()
}

func mint() {
	// prepare the private key
	pKey := strings.TrimSpace(*key)
	if pKey == "" {
		log.Fatalln("must specify --key with your account's private key")
	}
	if strings.HasPrefix(pKey, "0x") {
		pKey = pKey[2:]
	}
	pkey, err := crypto.HexToECDSA(pKey)
	if err != nil {
		log.Fatalln("failed to parse private key string:", err)
	}

	// set up the ethclient
	log.Println("Using RPC endpoint:", *rpc)
	client, err := ethclient.Dial(*rpc)
	if err != nil {
		log.Fatalln("error dialing L1 rpc endpoint:", err)
	}

	// prepare the address of the key holder, which is where tokens will be delivered to on Base
	address := crypto.PubkeyToAddress(pkey.PublicKey)
	log.Println("account address:", address)

	// make sure the account had some balance
	ctx1, cancel1 := context.WithTimeout(context.Background(), clientTimeout)
	defer cancel1()
	ethBalance, err := client.BalanceAt(ctx1, address, nil)
	if err != nil {
		log.Fatalln("failed to query balance:", err)
	}
	if ethBalance.BitLen() == 0 {
		log.Fatalln("the account address doesn't hold any Ethereum mainnet ETH")
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), clientTimeout)
	defer cancel2()
	h, err := client.HeaderByNumber(ctx2, nil)
	if err != nil {
		log.Fatalln("failed to get latest block header:", err)
	}
	if h.ExcessBlobGas == nil {
		log.Fatalln("excess blob gas is nil")
	}
	priorityFee, maxFee, blobFeeCap := minting.MaxFeesFromBaseFees(h.BaseFee, *h.ExcessBlobGas)
	log.Printf("baseFee=%v priorityFee=%v maxFee=%v maxBlobFee=%v\n", h.BaseFee, priorityFee, maxFee, blobFeeCap)

	// prepare blob
	sidecar, hashes, err := web.BlobFromString("")
	if err != nil {
		log.Fatalln("Could not convert blob to string:", err)
	}

	data := []byte{0x12, 0x49, 0xc5, 0x8b} // "mint()", 0x1249c58b
	// estimate gas of the tx
	gas, err := minting.EstimateGas(client, ethereum.CallMsg{
		From:          address,
		To:            &mintContract,
		GasTipCap:     priorityFee,
		GasFeeCap:     maxFee,
		BlobGasFeeCap: blobFeeCap,
		BlobHashes:    hashes,
		Data:          data,
	})
	if err != nil {
		// this usually indicates there wasn't enough eth in the account to cover the fees
		log.Fatalln("failed to estimate gas:", err)
	}
	log.Println("estimated gas:", gas)

	ctx3, cancel3 := context.WithTimeout(context.Background(), clientTimeout)
	defer cancel3()
	nonce, err := client.NonceAt(ctx3, address, nil)
	if err != nil {
		log.Fatalln("failed to get nonce:", err)
	}

	message := &types.BlobTx{
		Nonce:      nonce,
		ChainID:    uint256.NewInt(1),
		To:         mintContract,
		Data:       data,
		Gas:        gas,
		BlobHashes: hashes,
		Sidecar:    sidecar,
		GasTipCap:  uint256.MustFromBig(priorityFee),
		GasFeeCap:  uint256.MustFromBig(maxFee),
		BlobFeeCap: uint256.MustFromBig(blobFeeCap),
	}

	tx := types.NewTx(message)
	signer := types.LatestSignerForChainID(big.NewInt(1))
	signature, err := crypto.Sign(signer.Hash(tx).Bytes(), pkey)
	if err != nil {
		log.Fatalln("failed to sign transaction:", err)
	}
	signedTx, err := tx.WithSignature(signer, signature)
	if err != nil {
		log.Fatalln("failed to get tx WithSignature:", err)
	}

	txid := signedTx.Hash()
	log.Println("Sending transaction. txid:", txid)
	ctx4, cancel4 := context.WithTimeout(context.Background(), clientTimeout)
	defer cancel4()
	err = client.SendTransaction(ctx4, signedTx)
	if err != nil {
		log.Fatalln("failed to send transaction:", err)
	}

	// Now await receipt for up to 2 minutes before giving up
	start := time.Now()
	log.Println("Awaiting receipt...")
	for time.Now().Sub(start) <= 2*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), minting.L1_CLIENT_TIMEOUT)
		defer cancel()
		r, err := client.TransactionReceipt(ctx, txid)
		if err == ethereum.NotFound {
			time.Sleep(2 * time.Second)
			continue
		}
		if err != nil {
			log.Fatalln("error waiting for receipt:", err)
		}
		if r.Status != 0 {
			log.Println("got receipt. txid:", txid.Hex())
			return
		}
		// transaction must have reverted for unknown reasons
		log.Fatalln("receipt status nonzero. txid:", txid.Hex())
	} // for

	log.Fatalln("timeout waiting for receipt. txid", txid.Hex())
}
