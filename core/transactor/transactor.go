/*
 * Copyright (C) 2019 The "MysteriumNetwork/node" Authors.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package transactor

import (
	"fmt"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/ethereum/go-ethereum/common"
	pc "github.com/mysteriumnetwork/payments/crypto"
	"github.com/mysteriumnetwork/payments/registration"
	"github.com/pkg/errors"

	"github.com/mysteriumnetwork/node/identity"
	"github.com/mysteriumnetwork/node/metadata"
	"github.com/mysteriumnetwork/node/requests"
)

type transactor struct {
	http            requests.HTTPTransport
	endpointAddress string
	signerFactory   identity.SignerFactory
	registryAddress string
	accountantID    string
}

// NewTransactor creates and returns new Transactor instance
func NewTransactor(bindAddress, endpointAddress, registryAddress, accountantID string, signerFactory identity.SignerFactory) *transactor {
	return &transactor{
		http:            requests.NewHTTPClient(bindAddress, 20*time.Second),
		endpointAddress: endpointAddress,
		signerFactory:   signerFactory,
		registryAddress: registryAddress,
		accountantID:    accountantID,
	}
}

// Fees represents fees applied by Transactor
// swagger:model Fees
type Fees struct {
	Transaction  uint64 `json:"transaction"`
	Registration uint64 `json:"registration"`
}

// IdentityRegistrationRequestDTO represents the identity registration user input parameters
// swagger:model IdentityRegistrationRequestDTO
type IdentityRegistrationRequestDTO struct {
	// Stake is used by Provider, default 0
	Stake uint64 `json:"stake,omitempty"`
	// Cache out address for Provider
	Beneficiary string `json:"beneficiary,omitempty"`
	// Fee: negotiated fee with transactor
	Fee uint64 `json:"fee,omitempty"`
}

// IdentityRegistrationRequest represents the identity registration request body
type IdentityRegistrationRequest struct {
	RegistryAddress string `json:"registryAddress"`
	AccountantID    string `json:"accountantID"`
	// Stake is used by Provider, default 0
	Stake uint64 `json:"stake"`
	// Fee: negotiated fee with transactor
	Fee uint64 `json:"fee"`
	// Beneficiary: Provider channelID by default, optionally set during Identity registration.
	// Can be updated later through transactor. We can check it's value directly from SC.
	// Its a cache out address
	Beneficiary string `json:"beneficiary"`
	// Signature from fields above
	Signature string `json:"signature"`
	Identity  string `json:"identity"`
}

//  FetchFees fetches current transactor fees
func (t *transactor) FetchFees() (Fees, error) {
	f := Fees{}

	req, err := requests.NewGetRequest(t.endpointAddress, "fee/register", nil)
	if err != nil {
		return f, errors.Wrap(err, "failed to fetch transactor fees")
	}

	err = t.http.DoRequestAndParseResponse(req, &f)
	return f, err
}

// RegisterIdentity instructs Transactor to register identity on behalf of a client identified by 'id'
func (t *transactor) RegisterIdentity(id string, regReqDTO *IdentityRegistrationRequestDTO) error {
	regReq, err := t.fillIdentityRegistrationRequest(id, *regReqDTO)
	if err != nil {
		return errors.Wrap(err, "failed to fill in identity request")
	}

	err = t.validateRegisterIdentityRequest(regReq)
	if err != nil {
		return errors.Wrap(err, "identity request validation failed")
	}

	req, err := requests.NewPostRequest(t.endpointAddress, "identity/register", regReq)
	if err != nil {
		return errors.Wrap(err, "identity request to Transactor failed")
	}

	return t.http.DoRequest(req)
}

func (t *transactor) fillIdentityRegistrationRequest(id string, regReqDTO IdentityRegistrationRequestDTO) (IdentityRegistrationRequest, error) {
	regReq := IdentityRegistrationRequest{RegistryAddress: t.registryAddress, AccountantID: t.accountantID}

	regReq.Stake = regReqDTO.Stake
	regReq.Fee = regReqDTO.Fee

	if regReqDTO.Beneficiary == "" {
		// TODO: inject ChannelImplAddress through constructor
		channelAddress, err := pc.GenerateChannelAddress(id, regReq.RegistryAddress, metadata.TestnetDefinition.ChannelImplAddress)
		if err != nil {
			return IdentityRegistrationRequest{}, errors.Wrap(err, "failed to calculate channel address")
		}

		regReq.Beneficiary = channelAddress
	} else {
		regReq.Beneficiary = regReqDTO.Beneficiary
	}

	signer := t.signerFactory(identity.FromAddress(id))

	sig, err := t.signRegistrationRequest(signer, regReq)
	if err != nil {
		return IdentityRegistrationRequest{}, errors.Wrap(err, "failed to sign identity registration request")
	}

	signatureHex := common.Bytes2Hex(sig)
	regReq.Signature = strings.ToLower(fmt.Sprintf("0x%v", signatureHex))
	log.Info("regReq: %v", regReq)
	regReq.Identity = id

	return regReq, nil
}

func (t *transactor) validateRegisterIdentityRequest(regReq IdentityRegistrationRequest) error {
	if regReq.AccountantID == "" {
		return errors.New("AccountantID is required")
	}
	if regReq.RegistryAddress == "" {
		return errors.New("RegistryAddress is required")
	}
	return nil
}

func (t *transactor) signRegistrationRequest(signer identity.Signer, regReq IdentityRegistrationRequest) ([]byte, error) {
	req := registration.Request{
		RegistryAddress: strings.ToLower(regReq.RegistryAddress),
		AccountantID:    strings.ToLower(regReq.AccountantID),
		Stake:           regReq.Stake,
		Fee:             regReq.Fee,
		Beneficiary:     strings.ToLower(regReq.Beneficiary),
	}

	message := req.GetMessage()

	signature, err := signer.Sign(message)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign a registration request")
	}

	err = pc.ReformatSignatureVForBC(signature.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "signature reformat failed")
	}
	return signature.Bytes(), err
}
