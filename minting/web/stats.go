// mintTo() request handler
package web

import (
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"github.com/blobz/minting"
)

type statsResponse struct {
	Code int `json:"code"`

	Time             int64   `json:"time"`
	MintAmount       float64 `json:"mint_amount"`
	EstimatedGasFee  float64 `json:"estimated_fee"`
	CongestionFactor int     `json:"congestion_factor"`
}

// type minter implements jsonHandler, handles requests of type `mintRequest`, and returns a `mintResponse`.
type statter struct {
	*minting.MintingStats
}

func (m *statter) handle(r *http.Request, w http.ResponseWriter) (interface{}, *string, *string) {
	stats := m.GetStats()
	if !stats.Success {
		if time.Now().Unix()-stats.Time > 3600 {
			// don't use the stats if they haven't been successfully updated in over an hour
			return &statsResponse{Code: 100}, nil, nil
		}
		log.Warn("returning potentially stale stats")
	}
	return &statsResponse{
		Code:             1,
		Time:             stats.Time,
		MintAmount:       stats.MintAmount,
		EstimatedGasFee:  stats.EstimatedGasFee,
		CongestionFactor: int(stats.BlobBaseFee/1e9) + 1,
	}, nil, nil
}

func newStatsHandler(client *ethclient.Client, mintContract common.Address) http.Handler {
	return &defaultHandler{
		j: &statter{minting.NewMintingStats(client, mintContract)},
	}
}
