package api

import (
	"bytes"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"idena-go/blockchain/types"
	"idena-go/common"
	"idena-go/common/hexutil"
	"idena-go/core/ceremony"
	"idena-go/core/flip"
	"idena-go/ipfs"
	"idena-go/protocol"
)

const (
	MaxFlipSize = 1024 * 600
)

type FlipApi struct {
	baseApi   *BaseApi
	fp        *flip.Flipper
	pm        *protocol.ProtocolManager
	ipfsProxy ipfs.Proxy
	ceremony  *ceremony.ValidationCeremony
}

// NewFlipApi creates a new FlipApi instance
func NewFlipApi(baseApi *BaseApi, fp *flip.Flipper, pm *protocol.ProtocolManager, ipfsProxy ipfs.Proxy, ceremony *ceremony.ValidationCeremony) *FlipApi {
	return &FlipApi{baseApi, fp, pm, ipfsProxy, ceremony}
}

type FlipSubmitResponse struct {
	TxHash   common.Hash `json:"txHash"`
	FlipHash string      `json:"flipHash"`
}

// SubmitFlip receives an image as hex
func (api *FlipApi) SubmitFlip(hex *hexutil.Bytes) (FlipSubmitResponse, error) {

	if hex == nil {
		return FlipSubmitResponse{}, errors.New("flip is empty")
	}

	rawFlip := *hex

	if len(rawFlip) > MaxFlipSize {
		return FlipSubmitResponse{}, errors.Errorf("flip is too big, max expected size %v, actual %v", MaxFlipSize, len(rawFlip))
	}

	epoch := api.baseApi.engine.GetAppState().State.Epoch()

	cid, encryptedFlip, err := api.fp.PrepareFlip(epoch, rawFlip)

	if err != nil {
		return FlipSubmitResponse{}, err
	}

	tx, err := api.baseApi.getSignedTx(api.baseApi.getCurrentCoinbase(), common.Address{}, types.SubmitFlipTx, decimal.Zero, 0, 0, cid.Bytes(), nil)

	if err != nil {
		return FlipSubmitResponse{}, err
	}

	flip := types.Flip{
		Tx:   tx,
		Data: encryptedFlip,
	}

	if err := api.fp.AddNewFlip(flip); err != nil {
		return FlipSubmitResponse{}, err
	}

	if _, err := api.baseApi.sendInternalTx(tx); err != nil {
		return FlipSubmitResponse{}, err
	}

	api.pm.BroadcastFlip(&flip)

	if err != nil {
		return FlipSubmitResponse{}, err
	}

	return FlipSubmitResponse{
		TxHash:   tx.Hash(),
		FlipHash: cid.String(),
	}, nil
}

type FlipsResponse struct {
	Hexes []hexutil.Bytes `json:"hex"`
}

func (api *FlipApi) GetFlips() (FlipsResponse, error) {
	flips := api.ceremony.GetFlipsToSolve()
	if flips == nil {
		return FlipsResponse{}, errors.New("ceremony is not started")
	}

	var result []hexutil.Bytes
	for _, v := range flips {
		result = append(result, hexutil.Bytes(v))
	}

	return FlipsResponse{
		Hexes: result,
	}, nil
}

type FlipResponse struct {
	Hex   hexutil.Bytes `json:"hex"`
	Epoch uint16        `json:"epoch"`
	Mined bool          `json:"mined"`
}

func (api *FlipApi) GetFlip(hash string) (FlipResponse, error) {
	cids := api.baseApi.getAppState().State.FlipCids()
	c, _ := cid.Decode(hash)
	cidBytes := c.Bytes()

	data, epoch, err := api.fp.GetFlip(cidBytes)

	mined := false
	for _, item := range cids {
		if bytes.Compare(item, cidBytes) == 0 {
			mined = true
			break
		}
	}

	if err != nil {
		return FlipResponse{}, err
	}

	return FlipResponse{
		Hex:   hexutil.Bytes(data),
		Epoch: epoch,
		Mined: mined,
	}, nil
}
