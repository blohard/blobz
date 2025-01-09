package minting

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const (
	// A good default value to use as a timeout with the L1 ethclient.
	L1_CLIENT_TIMEOUT = 15 * time.Second
)

var (
	// Make sure a returned fee-per-gas is never less than this number, for when a base fee happens to be extremely low.
	MinFeePerGas = big.NewInt(100)

	Two   = big.NewInt(2)
	Three = big.NewInt(3)
)

// type MintingStats maintains a periodically refreshed cache of stats relevant to token minting.
type MintingStats struct {
	l1Client     *ethclient.Client
	mintContract common.Address
	chainID      *uint256.Int

	mutex      sync.RWMutex
	success    bool  // whether the stats were last updated without error
	lastUpdate int64 // last successful update time in unix seconds, 0 if never

	mintAmount  float64
	gasFee      float64 // expected gas fee in ETH
	blobBaseFee uint64
}

// MintingStatsSnapshot is the return value of MintingStats.GetStats
type MintingStatsSnapshot struct {
	Success bool  // whether the last attempt to update stats succeeded
	Time    int64 // unix time of last successful stat update, 0 if none

	MintAmount      float64 // units: # of tokens
	EstimatedGasFee float64 // units: ETH
	BlobBaseFee     uint64  // units: Wei
}

func (m *MintingStats) GetStats() *MintingStatsSnapshot {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	r := &MintingStatsSnapshot{
		Success:         m.success,
		MintAmount:      m.mintAmount,
		EstimatedGasFee: m.gasFee,
		BlobBaseFee:     m.blobBaseFee,
		Time:            m.lastUpdate,
	}
	return r
}

func NewMintingStats(client *ethclient.Client, mintContract common.Address) *MintingStats {
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Crit("error getting chain ID", "error", err)
	}
	r := &MintingStats{
		l1Client:     client,
		mintContract: mintContract,
		chainID:      uint256.MustFromBig(chainID),
	}
	go r.refreshLoop()
	return r
}

// MaxFeesFromBaseFees returns transaction fee-per-gas values that should provide a high
// probability of the transaction being included in the next block upon submittion.
func MaxFeesFromBaseFees(baseFee *big.Int, excessBlobGas uint64) (priorityFee, maxFee, blobFee *big.Int) {
	// For now we always return fixed 1 gwei priority fee.
	priorityFee = big.NewInt(params.GWei)

	// We use a 1.5 multiplier to provide a buffer against sudden base fee increases.
	maxFee = new(big.Int).Set(baseFee)
	maxFee.Mul(maxFee, Three).
		Div(maxFee, Two).
		Add(maxFee, MinFeePerGas)

	blobFee = eip4844.CalcBlobFee(excessBlobGas)
	blobFee.Mul(blobFee, Three).
		Div(blobFee, Two).
		Add(blobFee, MinFeePerGas)
	return
}

func EstimateGas(client *ethclient.Client, m ethereum.CallMsg) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), L1_CLIENT_TIMEOUT)
	defer cancel()
	gas, err := client.EstimateGas(ctx, m)
	if err != nil {
		return 0, err
	}
	// Actual gas usage can vary with the blob gas price, so to prevent random out of gas, we add a bit of a buffer.
	return uint64(float64(gas) * 1.2), nil
}

// refresh periodically updates the minting stats
func (m *MintingStats) refreshLoop() {
	errorWait := func() {
		{
			m.mutex.Lock()
			m.success = false
			m.mutex.Unlock()
		}
		time.Sleep(5 * time.Second)
	}

	// create dummy tx calldata for estimating gas
	dummyData := []byte{0x75, 0x5E, 0xDD, 0x17} // "mintTo()"
	// any non-0 address is good enough for our dummy calldata so we just use the mint contract address
	dummyData = append(dummyData, make([]byte, 12)...)
	dummyData = append(dummyData, m.mintContract.Bytes()...)
	var dummyHash common.Hash
	dummyHash[0] = 0x01 // blob kzg hash version 1

	for {
		ctx, cancel := context.WithTimeout(context.Background(), L1_CLIENT_TIMEOUT)
		defer cancel()
		h, err := m.l1Client.HeaderByNumber(ctx, nil)
		if err != nil {
			log.Error("failed to get header by number", "error", err)
			errorWait()
			continue
		}
		if h.ExcessBlobGas == nil {
			// this shouldn't happen
			log.Error("excess blob gas was nil")
			errorWait()
			continue
		}
		pFee, maxFeePerGas, blobFeeCap := MaxFeesFromBaseFees(h.BaseFee, *h.ExcessBlobGas)
		log.Info("fee caps", "maxFeePerGas", maxFeePerGas, "blobFeeCap", blobFeeCap)

		// estimate gas of the tx
		gas, err := EstimateGas(m.l1Client, ethereum.CallMsg{
			// The sender account we use cannot be a random dummy addres: it must have enough ETH
			// to cover the gas fee otherwise this will error out. Alternatively you can leave the
			// gas fees unspecified, but then the estimate provided by geth is inaccurate (about 2X
			// higher than actual -- geth bug?).
			From:          common.HexToAddress("0xC824b5b6cE65533EFd29864403821e510344c7F5"),
			To:            &m.mintContract,
			GasFeeCap:     maxFeePerGas,
			GasTipCap:     pFee,
			BlobGasFeeCap: blobFeeCap,
			BlobHashes:    []common.Hash{dummyHash},
			Data:          dummyData,
		})
		if err != nil {
			log.Error("failed to estimate gas", "error", err)
			errorWait()
			continue
		}
		log.Info("estimated gas", "gas", gas)

		// combine individual fee components into a single ETH fee value
		totalMaxFee := new(big.Int).SetUint64(gas)
		totalMaxFee.Mul(totalMaxFee, maxFeePerGas)
		blobFeeCap.Mul(blobFeeCap, big.NewInt(1<<17)) // gas per blob
		totalMaxFee.Add(totalMaxFee, blobFeeCap)
		fee := new(big.Float).SetInt(totalMaxFee)
		fee.Quo(fee, big.NewFloat(params.Ether)) // convert to units of ETH

		log.Info("total max fee (ETH)", "fee", fee)

		msg := ethereum.CallMsg{
			To:   &m.mintContract,
			Data: []byte{0xaa, 0x8c, 0x21, 0x7c}, // amount()
		}
		ctx2, cancel2 := context.WithTimeout(context.Background(), L1_CLIENT_TIMEOUT)
		defer cancel2()
		result, err := m.l1Client.CallContract(ctx2, msg, nil)
		if err != nil {
			log.Error("failed to get token minting amount", "error", err)
			errorWait()
			continue
		}
		amount := new(big.Float).SetInt(new(big.Int).SetBytes(result))
		amount.Quo(amount, big.NewFloat(1e18))
		log.Info("got amount", "tokens", amount)

		m.mutex.Lock()
		m.success = true
		m.lastUpdate = time.Now().Unix()
		m.gasFee, _ = fee.Float64()
		m.blobBaseFee = eip4844.CalcBlobFee(*h.ExcessBlobGas).Uint64()
		m.mintAmount, _ = amount.Float64()
		m.mutex.Unlock()

		time.Sleep(12 * time.Second) // sleep for the duration of a single block
	}
}
