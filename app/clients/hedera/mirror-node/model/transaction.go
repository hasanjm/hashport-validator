/*
 * Copyright 2022 LimeChain Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import (
	"errors"
	"github.com/limechain/hedera-eth-bridge-validator/app/helper/timestamp"
	"github.com/limechain/hedera-eth-bridge-validator/constants"
)

type (
	// Transaction struct used by the Hedera Mirror node REST API
	Transaction struct {
		ConsensusTimestamp   string        `json:"consensus_timestamp"`
		EntityId             string        `json:"entity_id"`
		TransactionHash      string        `json:"transaction_hash"`
		ValidStartTimestamp  string        `json:"valid_start_timestamp"`
		ChargedTxFee         int           `json:"charged_tx_fee"`
		MemoBase64           string        `json:"memo_base64"`
		Result               string        `json:"result"`
		Name                 string        `json:"name"`
		MaxFee               string        `json:"max_fee"`
		ValidDurationSeconds string        `json:"valid_duration_seconds"`
		Node                 string        `json:"node"`
		Scheduled            bool          `json:"scheduled"`
		TransactionID        string        `json:"transaction_id"`
		Transfers            []Transfer    `json:"transfers"`
		TokenTransfers       []Transfer    `json:"token_transfers"`
		NftTransfers         []NftTransfer `json:"nft_transfers"`
	}
	// Transfer struct used by the Hedera Mirror node REST API
	Transfer struct {
		Account string `json:"account"`
		Amount  int64  `json:"amount"`
		// When retrieving ordinary hbar transfers, this field does not get populated
		Token string `json:"token_id"`
	}
	// NftTransfer struct used by the Hedera mirror node REST API
	NftTransfer struct {
		ReceiverAccountID string `json:"receiver_account_id"`
		SenderAccountID   string `json:"sender_account_id"`
		SerialNumber      int64  `json:"serial_number"`
		Token             string `json:"token_id"`
	}
	// Response struct used by the Hedera Mirror node REST API and returned once
	// account transactions are queried
	Response struct {
		Transactions []Transaction
		Status       `json:"_status"`
	}
	// Schedule struct used by the Hedera Mirror node REST API to return information
	// regarding a given Schedule entity
	Schedule struct {
		ConsensusTimestamp string `json:"consensus_timestamp"`
		CreatorAccountId   string `json:"creator_account_id"`
		ExecutedTimestamp  string `json:"executed_timestamp"`
		Memo               string `json:"memo"`
		PayerAccountId     string `json:"payer_account_id"`
		ScheduleId         string `json:"schedule_id"`
	}

	// Nft struct used by Hedera Mirror node REST API to return information
	// for a given Nft entity
	Nft struct {
		AccountID         string `json:"account_id"`         // The account ID of the account associated with the NFT
		CreatedTimestamp  string `json:"created_timestamp"`  // The timestamp of when the NFT was created
		Deleted           bool   `json:"deleted"`            // Whether the token was deleted or not
		Metadata          string `json:"metadata"`           // The metadata of the NFT, in base64
		ModifiedTimestamp string `json:"modified_timestamp"` // The last time the token properties were modified
		SerialNumber      int64  `json:"serial_number"`      // The serial number of the NFT
		TokenID           string `json:"token_id"`           // The token ID of the NFT
	}
	// NftTransactionsResponse struct used by Hedera Mirror node REST API to return information
	// about an NFT's transaction
	NftTransactionsResponse struct {
		Transactions []NftTransaction `json:"transactions"`
		Links        Pagination       `json:"links"`
	}
	NftTransaction struct {
		TransactionID     string `json:"transaction_id"`      // The transaction ID of the transaction
		Type              string `json:"type"`                // The type of transaction TOKENBURN, TOKEMINT, CRYPTOTRANSFER
		SenderAccountID   string `json:"sender_account_id"`   // The account that sent the NFT
		ReceiverAccountID string `json:"receiver_account_id"` // The account that received the NFT
	}
	Pagination struct {
		Next string `json:"next"` // Hyperlink to the next page of results
	}
	// ParsedTransfer Used in GetIncomingTransfer to return the information about an Incoming Transfer
	ParsedTransfer struct {
		IsNft             bool
		AmountOrSerialNum int64
		Asset             string
	}
)

// getIncomingAmountFor returns the amount that is credited to the specified
// account for the given transaction
func (t Transaction) getIncomingAmountFor(account string) (int64, string, error) {
	for _, tr := range t.Transfers {
		if tr.Account == account {
			return tr.Amount, constants.Hbar, nil
		}
	}
	return 0, "", errors.New("no incoming transfer found")
}

// getIncomingTokenAmountFor returns the token amount that is credited to the specified
// account for the given transaction
func (t Transaction) getIncomingTokenAmountFor(account string) (int64, string, error) {
	for _, tr := range t.TokenTransfers {
		if tr.Account == account {
			return tr.Amount, tr.Token, nil
		}
	}
	return 0, "", errors.New("no incoming token transfer found")
}

func (t Transaction) getIncomingNftTransferFor(account string) (serialNum int64, token string, err error) {
	for _, ntr := range t.NftTransfers {
		if ntr.ReceiverAccountID == account {
			return ntr.SerialNumber, ntr.Token, nil
		}
	}

	return 0, "", errors.New("no incoming nft transfer found")
}

// GetHBARTransfer gets the HBAR transfer for an Account
func (t Transaction) GetHBARTransfer(account string) (amount int64, isFound bool) {
	for _, tr := range t.Transfers {
		if tr.Account == account {
			return tr.Amount, true
		}
	}

	return 0, false
}

// GetIncomingTransfer returns the transfer to an account in the following order:
// 1. Checks if there is an NFT transfer
// 2. Checks if there is a Fungible Token transfer
// 3. Checks if there is an HBAR transfer
func (t Transaction) GetIncomingTransfer(account string) (parsed ParsedTransfer, err error) {
	serialNum, asset, err := t.getIncomingNftTransferFor(account)
	if err == nil {
		return ParsedTransfer{
			IsNft:             true,
			AmountOrSerialNum: serialNum,
			Asset:             asset,
		}, nil
	}

	amount, asset, err := t.getIncomingTokenAmountFor(account)
	if err == nil {
		return ParsedTransfer{
			IsNft:             false,
			AmountOrSerialNum: amount,
			Asset:             asset,
		}, nil
	}

	amount, asset, err = t.getIncomingAmountFor(account)
	if err == nil {
		return ParsedTransfer{
			IsNft:             false,
			AmountOrSerialNum: amount,
			Asset:             asset,
		}, nil
	}

	return ParsedTransfer{}, err
}

// GetLatestTxnConsensusTime iterates all transactions and returns the consensus timestamp of the latest one
func (r Response) GetLatestTxnConsensusTime() (int64, error) {
	var max int64 = 0
	for _, t := range r.Transactions {
		ts, err := timestamp.FromString(t.ConsensusTimestamp)
		if err != nil {
			return 0, err
		}
		if ts > max {
			max = ts
		}
	}
	return max, nil
}

// IsNotFound traverses all Error messages and searches for Not Found message
func (r Response) IsNotFound() bool {
	for _, m := range r.Messages {
		if m.IsNotFound() {
			return true
		}
	}
	return false
}
