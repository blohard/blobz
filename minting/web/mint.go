// mintTo() request handler
package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"net/http"
	"time"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const (
	L1_CLIENT_TIMEOUT = 15 * time.Second
)

type mintRequest struct {
	PKey    string `json:"pkey"`    // hex encoded
	Address string `json:"address"` // hex encoded destination address for the minted tokens on Base
	Blob    []byte `json:"blob"`    // base64 encoded data to put in the blob, can be empty
}

type mintResponse struct {
	Code int    `json:"code"`
	TxID string `json:"txid"`
}

// type minter implements jsonHandler, handles requests of type `mintRequest`, and returns a `mintResponse`.
type minter struct {
	l1Client     *ethclient.Client
	mintContract common.Address
	chainID      *uint256.Int
}

func (m minter) handle(r *http.Request, w http.ResponseWriter) (interface{}, *string, *string) {
	req := mintRequest{}
	if err := decode(&req, r.Body); err != nil {
		return err, nil, nil
	}
	if !common.IsHexAddress(req.Address) {
		log.Warn("request address was not a hex address")
		return &Error{Code: 2}, nil, nil
	}
	if len(req.PKey) == 0 {
		log.Warn("no pkey set")
		return &Error{Code: 3}, nil, nil
	}
	var err error
	key, err := crypto.HexToECDSA(req.PKey)
	if err != nil {
		log.Warn("failed to create private key", "error", err)
		return &Error{Code: 4}, nil, nil
	}
	signerAddress := crypto.PubkeyToAddress(key.PublicKey)
	log.Info("Signer address", "address", signerAddress)

	ctx1, cancel1 := context.WithTimeout(context.Background(), L1_CLIENT_TIMEOUT)
	defer cancel1()
	ethBalance, err := m.l1Client.BalanceAt(ctx1, signerAddress, nil)
	if err != nil {
		log.Error("failed to query balance", "error", err)
		return &Error{Code: 5}, nil, nil
	}
	if ethBalance.BitLen() == 0 {
		log.Warn("signer address has 0 balance")
		return &Error{Code: 6}, nil, nil
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), L1_CLIENT_TIMEOUT)
	defer cancel2()
	h, err := m.l1Client.HeaderByNumber(ctx2, nil)
	if err != nil {
		log.Warn("Failed to get latest header", "error", err)
		return &Error{Code: 7}, nil, nil
	}

	priorityFee := big.NewInt(params.GWei)
	maxFee := new(big.Int).Set(h.BaseFee)
	maxFee.Mul(maxFee, big.NewInt(2)).Add(maxFee, priorityFee)

	blobFeeCap := eip4844.CalcBlobFee(*h.ExcessBlobGas)
	blobFeeCap.Mul(blobFeeCap, big.NewInt(2))

	log.Info("fees", "priorityFee", priorityFee, "maxFee", maxFee, "baseFee", h.BaseFee, "blobFeeCap", blobFeeCap)

	if len(req.Blob) == 0 {
		log.Info("No blob data provided, using default blob text.")
		defaultBlobText := "I just minted $BLOBZ with proof-of-blob! Learn more at https://blobz.wtf"
		req.Blob = []byte(defaultBlobText)
	}
	log.Info("blob text", "text", string(req.Blob))

	var blob kzg4844.Blob

	b := blob[:]
	woffset := 0
	roffset := 0

	write32 := func() {
		// put a 0 in the first byte to make sure we always have a valid field element
		b[woffset] = 0
		woffset++
		wend := woffset + 31
		rend := roffset + 31
		if rend > len(req.Blob) {
			rend = len(req.Blob)
		}
		copy(b[woffset:wend], req.Blob[roffset:rend])
		woffset = wend
		roffset = rend
	}

	for roffset < len(req.Blob) && woffset < len(b)-32 {
		write32()
	}

	var c kzg4844.Commitment
	c, err = kzg4844.BlobToCommitment(&blob)
	if err != nil {
		log.Error("failed to compute blob commitment", "error", err)
		return &Error{Code: 8}, nil, nil
	}
	proof, err := kzg4844.ComputeBlobProof(&blob, c)
	if err != nil {
		log.Error("failed to compute blob proof", "error", err)
		return &Error{Code: 9}, nil, nil
	}

	sidecar := &types.BlobTxSidecar{}
	sidecar.Blobs = []kzg4844.Blob{blob}
	sidecar.Commitments = []kzg4844.Commitment{c}
	sidecar.Proofs = []kzg4844.Proof{proof}

	hasher := sha256.New()
	kzgHash := kzg4844.CalcBlobHashV1(hasher, &c)
	hashes := []common.Hash{kzgHash}

	destAddress := common.HexToAddress(req.Address)
	data := []byte{0x75, 0x5E, 0xDD, 0x17} // "mintTo()"
	for i := 0; i < 12; i++ {
		data = append(data, 0x00)
	}
	data = append(data, destAddress.Bytes()...)
	log.Info("data", "data", hex.EncodeToString(data))

	// estimate gas of the tx
	ctx3, cancel3 := context.WithTimeout(context.Background(), L1_CLIENT_TIMEOUT)
	defer cancel3()
	gas, err := m.l1Client.EstimateGas(ctx3, ethereum.CallMsg{
		From:          signerAddress,
		To:            &m.mintContract,
		GasTipCap:     priorityFee,
		GasFeeCap:     maxFee,
		BlobGasFeeCap: blobFeeCap,
		BlobHashes:    hashes,
		Data:          data,
	})
	if err != nil {
		log.Error("failed to estimate gas", "error", err)
		return &Error{Code: 10}, nil, nil
	}
	log.Info("estimated gas", "gas", gas)
	// Actual gas usage can vary with the blob gas price, so to prevent random out of gas, we add a bit of a buffer:
	gas = uint64(float64(gas) * 1.2)

	ctx4, cancel4 := context.WithTimeout(context.Background(), L1_CLIENT_TIMEOUT)
	defer cancel4()
	nonce, err := m.l1Client.NonceAt(ctx4, signerAddress, nil)
	if err != nil {
		log.Error("failed to get nonce", "error", err)
		return &Error{Code: 11}, nil, nil
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
		return &Error{Code: 12}, nil, nil
	}
	signedTx, err := tx.WithSignature(signer, signature)
	if err != nil {
		log.Error("failed to get tx WithSignature", "error", err)
		return &Error{Code: 13}, nil, nil
	}

	ctx5, cancel5 := context.WithTimeout(context.Background(), L1_CLIENT_TIMEOUT)
	defer cancel5()
	err = m.l1Client.SendTransaction(ctx5, signedTx)
	if err != nil {
		log.Error("failed to send transaction", "error", err)
		return &Error{Code: 14}, nil, nil
	}

	return &mintResponse{
		Code: 1,
		TxID: signedTx.Hash().Hex(),
	}, nil, nil
}

func newMintHandler(l1RPC string, mintContract common.Address) http.Handler {
	// TODO: handle clean shutdown
	client, err := ethclient.Dial(l1RPC)
	if err != nil {
		log.Crit("error dialing L1 rpc endpoint", "error", err)
	}
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
