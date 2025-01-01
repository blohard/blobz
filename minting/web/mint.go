// mintTo() request handler
package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"github.com/blobz/minting"
)

type mintRequest struct {
	PKey    string `json:"pkey"`    // hex encoded
	Address string `json:"address"` // hex encoded destination address for the minted tokens on Base
	Blob    string `json:"blob"`    // data to put in the blob, can be empty
}

type mintResponse struct {
	Code    int    `json:"code"`
	TxID    string `json:"txid"`
	Address string `json:"address"`
}

// type minter implements jsonHandler, handles requests of type `mintRequest`, and returns a `mintResponse`.
type minter struct {
	l1Client     *ethclient.Client
	mintContract common.Address
	chainID      *uint256.Int
}

// Next error code: 120
func (m minter) handle(r *http.Request, w http.ResponseWriter) (interface{}, *string, *string) {
	req := mintRequest{}
	if err := decode(&req, r.Body); err != nil {
		return err, nil, nil
	}

	req.PKey = strings.TrimSpace(req.PKey)
	if len(req.PKey) == 0 {
		log.Warn("no pkey set")
		return &Error{Code: 103}, nil, nil
	}
	if strings.HasPrefix(req.PKey, "0x") {
		req.PKey = req.PKey[2:]
	}
	var err error
	key, err := crypto.HexToECDSA(req.PKey)
	if err != nil {
		log.Warn("failed to create private key", "error", err)
		return &Error{Code: 104}, nil, nil
	}

	signerAddress := crypto.PubkeyToAddress(key.PublicKey)
	log.Info("Signer address", "address", signerAddress)
	req.Address = strings.TrimSpace(req.Address)
	destAddress := signerAddress
	if len(req.Address) > 0 {
		if !common.IsHexAddress(req.Address) {
			log.Warn("request address was not a hex address")
			return &Error{Code: 102}, nil, nil
		}
		destAddress = common.HexToAddress(req.Address)
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), minting.L1_CLIENT_TIMEOUT)
	defer cancel1()
	ethBalance, err := m.l1Client.BalanceAt(ctx1, signerAddress, nil)
	if err != nil {
		log.Error("failed to query balance", "error", err)
		return &Error{Code: 105}, nil, nil
	}
	if ethBalance.BitLen() == 0 {
		log.Warn("signer address has 0 balance")
		return &Error{Code: 106}, nil, nil
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), minting.L1_CLIENT_TIMEOUT)
	defer cancel2()
	h, err := m.l1Client.HeaderByNumber(ctx2, nil)
	if err != nil {
		log.Warn("Failed to get latest header", "error", err)
		return &Error{Code: 107}, nil, nil
	}
	if h.ExcessBlobGas == nil {
		// this shouldn't happen
		log.Error("excess blob gas is nil")
		return &Error{Code: 119}, nil, nil
	}
	priorityFee, maxFee, blobFeeCap := minting.MaxFeesFromBaseFees(h.BaseFee, *h.ExcessBlobGas)
	log.Info("fees", "priorityFee", priorityFee, "maxFee", maxFee, "baseFee", h.BaseFee, "blobFeeCap", blobFeeCap)

	if !utf8.ValidString(req.Blob) {
		log.Warn("invalid utf-8 in blob string")
		return &ErrorWithMessage{
			Code:    115,
			Message: "blob string was not UTF-8 as expected",
		}, nil, nil
	}
	req.Blob = strings.TrimSpace(req.Blob)
	if len(req.Blob) == 0 {
		log.Info("No blob data provided, using default blob text.")
		req.Blob = "I just minted $BLOBZ with proof-of-blob! Learn more at https://blobz.wtf"
	} else {
		req.Blob = "Proof-of-blob submitted through https://mint.blobz.wtf. User message: " + req.Blob
	}
	log.Info("blob text", "text", req.Blob)

	var blob kzg4844.Blob

	b := blob[:]
	woffset := 0
	roffset := 0

	userBlob := []byte(req.Blob)
	// TODO: this breaks UTF-8 if a rune is larger than 1 byte and we insert a 0 between it
	write32 := func() {
		// put a 0 in the first byte to make sure we always have a valid field element
		b[woffset] = 0
		woffset++
		wend := woffset + 31
		rend := roffset + 31
		if rend > len(userBlob) {
			rend = len(userBlob)
		}
		copy(b[woffset:wend], userBlob[roffset:rend])
		woffset = wend
		roffset = rend
	}

	for roffset < len(userBlob) && woffset < len(b)-32 {
		write32()
	}

	var c kzg4844.Commitment
	c, err = kzg4844.BlobToCommitment(&blob)
	if err != nil {
		log.Error("failed to compute blob commitment", "error", err)
		return &Error{Code: 108}, nil, nil
	}
	proof, err := kzg4844.ComputeBlobProof(&blob, c)
	if err != nil {
		log.Error("failed to compute blob proof", "error", err)
		return &Error{Code: 109}, nil, nil
	}

	sidecar := &types.BlobTxSidecar{}
	sidecar.Blobs = []kzg4844.Blob{blob}
	sidecar.Commitments = []kzg4844.Commitment{c}
	sidecar.Proofs = []kzg4844.Proof{proof}

	hasher := sha256.New()
	kzgHash := kzg4844.CalcBlobHashV1(hasher, &c)
	hashes := []common.Hash{kzgHash}

	data := []byte{0x75, 0x5E, 0xDD, 0x17} // "mintTo()"
	for i := 0; i < 12; i++ {
		data = append(data, 0x00)
	}
	data = append(data, destAddress.Bytes()...)
	log.Info("data", "data", hex.EncodeToString(data))

	// estimate gas of the tx
	gas, err := minting.EstimateGas(m.l1Client, ethereum.CallMsg{
		From:          signerAddress,
		To:            &m.mintContract,
		GasTipCap:     priorityFee,
		GasFeeCap:     maxFee,
		BlobGasFeeCap: blobFeeCap,
		BlobHashes:    hashes,
		Data:          data,
	})
	if err != nil {
		// this usually indicates there wasn't enough eth in the account to cover the fees
		log.Error("failed to estimate gas", "error", err)
		return &Error{Code: 110}, nil, nil
	}
	log.Info("estimated gas", "gas", gas)

	ctx4, cancel4 := context.WithTimeout(context.Background(), minting.L1_CLIENT_TIMEOUT)
	defer cancel4()
	nonce, err := m.l1Client.NonceAt(ctx4, signerAddress, nil)
	if err != nil {
		log.Error("failed to get nonce", "error", err)
		return &Error{Code: 111}, nil, nil
	}

	message := &types.BlobTx{
		Nonce:      nonce,
		ChainID:    m.chainID,
		To:         m.mintContract,
		Data:       data,
		Gas:        gas,
		BlobHashes: hashes,
		Sidecar:    sidecar,
		GasTipCap:  uint256.MustFromBig(priorityFee),
		GasFeeCap:  uint256.MustFromBig(maxFee),
		BlobFeeCap: uint256.MustFromBig(blobFeeCap),
	}

	tx := types.NewTx(message)
	signer := types.LatestSignerForChainID(m.chainID.ToBig())
	signature, err := crypto.Sign(signer.Hash(tx).Bytes(), key)
	if err != nil {
		log.Error("failed to sign transaction", "error", err)
		return &Error{Code: 112}, nil, nil
	}
	signedTx, err := tx.WithSignature(signer, signature)
	if err != nil {
		log.Error("failed to get tx WithSignature", "error", err)
		return &Error{Code: 113}, nil, nil
	}

	ctx5, cancel5 := context.WithTimeout(context.Background(), minting.L1_CLIENT_TIMEOUT)
	defer cancel5()
	err = m.l1Client.SendTransaction(ctx5, signedTx)
	if err != nil {
		log.Error("failed to send transaction", "error", err)
		return &ErrorWithMessage{
			Code:    114,
			Message: "Failed to send transaction: " + err.Error(),
		}, nil, nil
	}

	// Now await receipt for up to 1 minute before giving up
	txid := signedTx.Hash()
	start := time.Now()
	for time.Now().Sub(start) <= time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), minting.L1_CLIENT_TIMEOUT)
		defer cancel()
		r, err := m.l1Client.TransactionReceipt(ctx, txid)
		if err == ethereum.NotFound {
			time.Sleep(2 * time.Second)
			continue
		}
		if err != nil {
			log.Error("error waiting for receipt", "error", err)
			return &ErrorWithMessage{
				Code:    116,
				Message: "Error waiting for receipt: " + err.Error(),
			}, nil, nil
		}
		if r.Status != 0 {
			log.Info("got receipt", "txid", txid.Hex())
			return &mintResponse{
				Code:    1,
				TxID:    txid.Hex(),
				Address: destAddress.Hex(),
			}, nil, nil
		}
		// transaction must have reverted for unknown reasons
		log.Error("receipt status nonzero", "txid", txid.Hex())
		return &Error{Code: 118}, nil, nil
	}

	log.Error("timeout waiting for receipt", "txid", txid.Hex())
	// TODO: Should we return the txid + address just in case it goes through?
	return &Error{Code: 117}, nil, nil
}

func newMintHandler(client *ethclient.Client, mintContract common.Address) http.Handler {
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Crit("error getting chain ID", "error", err)
	}
	return &defaultHandler{
		j: &minter{
			l1Client:     client,
			mintContract: mintContract,
			chainID:      uint256.MustFromBig(chainID),
		},
	}
}
