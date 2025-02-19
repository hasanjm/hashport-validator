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

package evm

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/limechain/hedera-eth-bridge-validator/app/clients/evm/contracts/router"
	"github.com/limechain/hedera-eth-bridge-validator/app/core/queue"
	"github.com/limechain/hedera-eth-bridge-validator/app/model/transfer"
	"github.com/limechain/hedera-eth-bridge-validator/config"
	"github.com/limechain/hedera-eth-bridge-validator/constants"
	testConstants "github.com/limechain/hedera-eth-bridge-validator/test/constants"
	"github.com/limechain/hedera-eth-bridge-validator/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"math/big"
	"strings"
	"testing"
)

var (
	w       = &Watcher{}
	lockLog = &router.RouterLock{
		TargetChain: big.NewInt(0),
		Token:       common.HexToAddress("0x0000000000000000000000000000000000000000"),
		Receiver:    hederaAcc.ToBytes(),
		Amount:      big.NewInt(1),
		ServiceFee:  big.NewInt(0),
	}
	burnLog = &router.RouterBurn{
		TargetChain: big.NewInt(0),
		Token:       common.HexToAddress("0x0000000000000000000000000000000000000001"),
		Receiver:    hederaAcc.ToBytes(),
		Amount:      big.NewInt(1),
	}

	hederaAcc, _   = hedera.AccountIDFromString("0.0.123456")
	hederaBytes    = hederaAcc.ToBytes()
	dbIdentifier   = "3-0x0000000000000000000000000000000000000001"
	mintHash       = common.HexToHash("0579df6e9dbf066ba9fbd51ef5241e2b9f9c042a70289e8e5333d714ed4e5787")
	burnHash       = common.HexToHash("97715804dcd62a721835eaba4356dc90eaf6d442a12fe944f01bbf5f8c0b8992")
	lockHash       = common.HexToHash("aa3a3bc72b8c754ca6ee8425a5531bafec37569ec012d62d5f682ca909ae06f1")
	unlockHash     = common.HexToHash("483dd9d090112259cd3c44a9af4b3386be4b4b87145e6bf85bc0964a06062a73")
	membersHash    = common.HexToHash("30f1d11f11278ba2cc669fd4c95ee8d46ede2c82f6af0b74e4f427369b3522d3")
	burnERC721Hash = common.HexToHash("eb703661daf51ce0c247ebbf71a8747e6a79f36b2e93a4e5a22f191321e5750e")
	topics         = [][]common.Hash{
		{
			mintHash,
			burnHash,
			lockHash,
			unlockHash,
			membersHash,
			burnERC721Hash,
		},
	}
	filterConfig = FilterConfig{
		abi:    abi.ABI{},
		topics: topics,
		addresses: []common.Address{
			common.HexToAddress("0x0000000000000000000000000000000000000000"),
		},
		mintHash:          mintHash,
		burnHash:          burnHash,
		lockHash:          lockHash,
		unlockHash:        unlockHash,
		memberUpdatedHash: membersHash,
	}
)

func Test_HandleLockLog_Removed_Fails(t *testing.T) {
	setup()

	lockLog.Raw.Removed = true
	w.handleLockLog(lockLog, mocks.MQueue)
	lockLog.Raw.Removed = false

	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
}

func Test_HandleLockLog_EmptyReceiver_Fails(t *testing.T) {
	setup()

	lockLog.Receiver = []byte{}
	w.handleLockLog(lockLog, mocks.MQueue)
	lockLog.Receiver = hederaBytes

	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
}

func Test_HandleLockLog_InvalidReceiver_Fails(t *testing.T) {
	setup()
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(1), nil)

	lockLog.Receiver = []byte{1}
	w.handleLockLog(lockLog, mocks.MQueue)
	lockLog.Receiver = hederaBytes

	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
}

func Test_HandleLockLog_EmptyWrappedAsset_Fails(t *testing.T) {
	setup()
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(2), nil)

	w.handleLockLog(lockLog, mocks.MQueue)

	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
}

func Test_HandleLockLog_HappyPath(t *testing.T) {
	setup()
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)
	mocks.MBridgeContractService.On("RemoveDecimals", lockLog.Amount, lockLog.Token.String()).Return(lockLog.Amount, nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	parsedLockLog := &transfer.Transfer{
		TransactionId: fmt.Sprintf("%s-%d", lockLog.Raw.TxHash, lockLog.Raw.Index),
		SourceChainId: 33,
		TargetChainId: lockLog.TargetChain.Uint64(),
		NativeChainId: 33,
		SourceAsset:   lockLog.Token.String(),
		TargetAsset:   constants.Hbar,
		NativeAsset:   lockLog.Token.String(),
		Receiver:      hederaAcc.String(),
		Amount:        lockLog.Amount.String(),
	}

	mocks.MStatusRepository.On("Update", mocks.MBridgeContractService.Address().String(), int64(0)).Return(nil)
	mocks.MQueue.On("Push", &queue.Message{Payload: parsedLockLog, Topic: constants.HederaMintHtsTransfer}).Return()

	w.handleLockLog(lockLog, mocks.MQueue)
}

func Test_HandleLockLog_ReadOnlyHederaMintHtsTransfer(t *testing.T) {
	mocks.Setup()
	mocks.MBridgeContractService.On("RemoveDecimals", lockLog.Amount, lockLog.Token.String()).Return(lockLog.Amount, nil)
	mocks.MEVMClient.On("GetBlockTimestamp", big.NewInt(0)).Return(uint64(1))
	mocks.MStatusRepository.On("Get", mock.Anything).Return(int64(0), nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	w = &Watcher{
		repository:        mocks.MStatusRepository,
		contracts:         mocks.MBridgeContractService,
		evmClient:         mocks.MEVMClient,
		logger:            config.GetLoggerFor(fmt.Sprintf("EVM Router Watcher [%s]", dbIdentifier)),
		mappings:          config.LoadAssets(testConstants.Networks),
		validator:         false,
		prometheusService: mocks.MPrometheusService,
	}

	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)
	parsedLockLog := &transfer.Transfer{
		TransactionId: fmt.Sprintf("%s-%d", lockLog.Raw.TxHash, lockLog.Raw.Index),
		SourceChainId: 33,
		TargetChainId: lockLog.TargetChain.Uint64(),
		NativeChainId: 33,
		SourceAsset:   lockLog.Token.String(),
		TargetAsset:   constants.Hbar,
		NativeAsset:   lockLog.Token.String(),
		Receiver:      hederaAcc.String(),
		Amount:        lockLog.Amount.String(),
		Timestamp:     "1",
	}

	mocks.MStatusRepository.On("Update", mocks.MBridgeContractService.Address().String(), int64(0)).Return(nil)
	mocks.MQueue.On("Push", &queue.Message{Payload: parsedLockLog, Topic: constants.ReadOnlyHederaMintHtsTransfer}).Return()

	w.handleLockLog(lockLog, mocks.MQueue)
}

func Test_HandleLockLog_ReadOnlyTransferSave(t *testing.T) {
	mocks.Setup()
	mocks.MEVMClient.On("GetBlockTimestamp", big.NewInt(0)).Return(uint64(1))
	mocks.MStatusRepository.On("Get", mock.Anything).Return(int64(0), nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	lockLog.TargetChain = big.NewInt(1)
	w = &Watcher{
		repository:        mocks.MStatusRepository,
		contracts:         mocks.MBridgeContractService,
		prometheusService: mocks.MPrometheusService,
		evmClient:         mocks.MEVMClient,
		logger:            config.GetLoggerFor(fmt.Sprintf("EVM Router Watcher [%s]", dbIdentifier)),
		mappings:          config.LoadAssets(testConstants.Networks),
		validator:         false,
	}

	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)
	parsedLockLog := &transfer.Transfer{
		TransactionId: fmt.Sprintf("%s-%d", lockLog.Raw.TxHash, lockLog.Raw.Index),
		SourceChainId: 33,
		TargetChainId: lockLog.TargetChain.Uint64(),
		NativeChainId: 33,
		SourceAsset:   lockLog.Token.String(),
		TargetAsset:   "0xsome-other-eth-address",
		NativeAsset:   lockLog.Token.String(),
		Receiver:      common.BytesToAddress(hederaAcc.ToBytes()).String(),
		Amount:        lockLog.Amount.String(),
		Timestamp:     "1",
	}

	mocks.MStatusRepository.On("Update", mocks.MBridgeContractService.Address().String(), int64(0)).Return(nil)
	mocks.MQueue.On("Push", &queue.Message{Payload: parsedLockLog, Topic: constants.ReadOnlyTransferSave}).Return()

	w.handleLockLog(lockLog, mocks.MQueue)
	lockLog.TargetChain = big.NewInt(0)
}

func Test_HandleLockLog_TopicMessageSubmission(t *testing.T) {
	setup()
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	lockLog.TargetChain = big.NewInt(1)
	parsedLockLog := &transfer.Transfer{
		TransactionId: fmt.Sprintf("%s-%d", lockLog.Raw.TxHash, lockLog.Raw.Index),
		SourceChainId: 33,
		TargetChainId: lockLog.TargetChain.Uint64(),
		NativeChainId: 33,
		SourceAsset:   lockLog.Token.String(),
		TargetAsset:   "0xsome-other-eth-address",
		NativeAsset:   lockLog.Token.String(),
		Receiver:      common.BytesToAddress(hederaAcc.ToBytes()).String(),
		Amount:        lockLog.Amount.String(),
	}

	mocks.MStatusRepository.On("Update", mocks.MBridgeContractService.Address().String(), int64(0)).Return(nil)
	mocks.MQueue.On("Push", &queue.Message{Payload: parsedLockLog, Topic: constants.TopicMessageSubmission}).Return()

	w.handleLockLog(lockLog, mocks.MQueue)
	lockLog.TargetChain = big.NewInt(0)
}

func Test_HandleBurnLog_HappyPath(t *testing.T) {
	setup()
	mocks.MBridgeContractService.On("RemoveDecimals", burnLog.Amount, burnLog.Token.String()).Return(lockLog.Amount, nil)
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	parsedBurnLog := &transfer.Transfer{
		TransactionId: fmt.Sprintf("%s-%d", burnLog.Raw.TxHash, burnLog.Raw.Index),
		SourceChainId: 33,
		TargetChainId: burnLog.TargetChain.Uint64(),
		NativeChainId: 0,
		SourceAsset:   burnLog.Token.String(),
		TargetAsset:   constants.Hbar,
		NativeAsset:   constants.Hbar,
		Receiver:      hederaAcc.String(),
		Amount:        burnLog.Amount.String(),
	}

	mocks.MStatusRepository.On("Update", mocks.MBridgeContractService.Address().String(), int64(0)).Return(nil)
	mocks.MQueue.On("Push", &queue.Message{Payload: parsedBurnLog, Topic: constants.HederaFeeTransfer}).Return()

	w.handleBurnLog(burnLog, mocks.MQueue)
}

func Test_HandleBurnLog_InvalidHederaRecipient(t *testing.T) {
	setup()
	defaultReceiver := burnLog.Receiver
	burnLog.Receiver = []byte{1, 2, 3, 4}
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)
	w.handleBurnLog(burnLog, mocks.MQueue)
	burnLog.Receiver = defaultReceiver
}

func Test_HandleBurnLog_TopicMessageSubmission(t *testing.T) {
	setup()
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	burnLog.TargetChain = big.NewInt(1)
	defaultToken := burnLog.Token
	burnLog.Token = common.HexToAddress("0x123")
	receiver := common.BytesToAddress(burnLog.Receiver).String()
	parsedBurnLog := &transfer.Transfer{
		TransactionId: fmt.Sprintf("%s-%d", burnLog.Raw.TxHash, burnLog.Raw.Index),
		SourceChainId: 33,
		TargetChainId: 1,
		NativeChainId: 1,
		SourceAsset:   burnLog.Token.String(),
		TargetAsset:   "0xb083879B1e10C8476802016CB12cd2F25a896691",
		NativeAsset:   "0xb083879B1e10C8476802016CB12cd2F25a896691",
		Receiver:      receiver,
		Amount:        burnLog.Amount.String(),
	}

	mocks.MStatusRepository.On("Update", mocks.MBridgeContractService.Address().String(), int64(0)).Return(nil)
	mocks.MQueue.On("Push", &queue.Message{Payload: parsedBurnLog, Topic: constants.TopicMessageSubmission}).Return()

	w.handleBurnLog(burnLog, mocks.MQueue)
	burnLog.TargetChain = big.NewInt(0)
	burnLog.Token = defaultToken
}

func Test_HandleBurnLog_ReadOnlyTransferSave(t *testing.T) {
	mocks.Setup()
	mocks.MEVMClient.On("GetBlockTimestamp", big.NewInt(0)).Return(uint64(1))
	mocks.MStatusRepository.On("Get", mock.Anything).Return(int64(0), nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	burnLog.TargetChain = big.NewInt(1)
	w = &Watcher{
		repository:        mocks.MStatusRepository,
		contracts:         mocks.MBridgeContractService,
		prometheusService: mocks.MPrometheusService,
		evmClient:         mocks.MEVMClient,
		logger:            config.GetLoggerFor(fmt.Sprintf("EVM Router Watcher [%s]", dbIdentifier)),
		mappings:          config.LoadAssets(testConstants.Networks),
		validator:         false,
	}

	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)

	burnLog.TargetChain = big.NewInt(1)
	defaultToken := burnLog.Token
	burnLog.Token = common.HexToAddress("0x123")
	receiver := common.BytesToAddress(burnLog.Receiver).String()
	parsedBurnLog := &transfer.Transfer{
		TransactionId: fmt.Sprintf("%s-%d", burnLog.Raw.TxHash, burnLog.Raw.Index),
		SourceChainId: 33,
		TargetChainId: 1,
		NativeChainId: 1,
		SourceAsset:   burnLog.Token.String(),
		TargetAsset:   "0xb083879B1e10C8476802016CB12cd2F25a896691",
		NativeAsset:   "0xb083879B1e10C8476802016CB12cd2F25a896691",
		Receiver:      receiver,
		Amount:        burnLog.Amount.String(),
		Timestamp:     "1",
	}

	mocks.MStatusRepository.On("Update", mocks.MBridgeContractService.Address().String(), int64(0)).Return(nil)
	mocks.MQueue.On("Push", &queue.Message{Payload: parsedBurnLog, Topic: constants.ReadOnlyTransferSave}).Return()

	w.handleBurnLog(burnLog, mocks.MQueue)
	burnLog.TargetChain = big.NewInt(0)
	burnLog.Token = defaultToken
}

func Test_HandleBurnLog_ReadOnlyHederaTransfer(t *testing.T) {
	mocks.Setup()
	mocks.MBridgeContractService.On("RemoveDecimals", burnLog.Amount, burnLog.Token.String()).Return(lockLog.Amount, nil)
	mocks.MEVMClient.On("GetBlockTimestamp", big.NewInt(0)).Return(uint64(1))
	mocks.MStatusRepository.On("Get", mock.Anything).Return(int64(0), nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	w = &Watcher{
		repository:        mocks.MStatusRepository,
		contracts:         mocks.MBridgeContractService,
		prometheusService: mocks.MPrometheusService,
		evmClient:         mocks.MEVMClient,
		logger:            config.GetLoggerFor(fmt.Sprintf("EVM Router Watcher [%s]", dbIdentifier)),
		mappings:          config.LoadAssets(testConstants.Networks),
		validator:         false,
	}

	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)
	parsedBurnLog := &transfer.Transfer{
		TransactionId: fmt.Sprintf("%s-%d", burnLog.Raw.TxHash, burnLog.Raw.Index),
		SourceChainId: 33,
		TargetChainId: 0,
		NativeChainId: 0,
		SourceAsset:   burnLog.Token.String(),
		TargetAsset:   constants.Hbar,
		NativeAsset:   constants.Hbar,
		Receiver:      hederaAcc.String(),
		Amount:        burnLog.Amount.String(),
		Timestamp:     "1",
	}

	mocks.MStatusRepository.On("Update", mocks.MBridgeContractService.Address().String(), int64(0)).Return(nil)
	mocks.MQueue.On("Push", &queue.Message{Payload: parsedBurnLog, Topic: constants.ReadOnlyHederaTransfer}).Return()

	w.handleBurnLog(burnLog, mocks.MQueue)
}

func Test_HandleBurnLog_Token_Not_Supported(t *testing.T) {
	setup()
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)

	defaultToken := burnLog.Token
	burnLog.Token = common.HexToAddress("0x0123123")
	w.handleBurnLog(burnLog, mocks.MQueue)
	mocks.MStatusRepository.AssertNotCalled(t, "Update", mocks.MBridgeContractService.Address().String(), int64(0))
	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
	burnLog.Token = defaultToken
}

func Test_HandleBurnLog_WrappedToWrapped_Not_Supported(t *testing.T) {
	setup()
	mocks.MEVMClient.On("ChainID", context.Background()).Return(big.NewInt(33), nil)

	defaultTargetChain := burnLog.TargetChain
	burnLog.TargetChain = big.NewInt(1)
	w.handleBurnLog(burnLog, mocks.MQueue)
	mocks.MStatusRepository.AssertNotCalled(t, "Update", mocks.MBridgeContractService.Address().String(), int64(0))
	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
	burnLog.TargetChain = defaultTargetChain
}

func Test_HandleBurnLog_Raw_Removed(t *testing.T) {
	setup()
	burnLog.Raw.Removed = true

	w.handleBurnLog(burnLog, mocks.MQueue)

	mocks.MStatusRepository.AssertNotCalled(t, "Update", mocks.MBridgeContractService.Address().String(), int64(0))
	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
	burnLog.Raw.Removed = false
}

func Test_HandleBurnLog_No_Receivers(t *testing.T) {
	setup()
	receiver := burnLog.Receiver
	burnLog.Receiver = []byte{}

	w.handleBurnLog(burnLog, mocks.MQueue)

	mocks.MStatusRepository.AssertNotCalled(t, "Update", mocks.MBridgeContractService.Address().String(), int64(0))
	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
	burnLog.Receiver = receiver
}

func TestNewWatcher(t *testing.T) {
	mocks.Setup()

	mocks.MStatusRepository.On("Get", mock.Anything).Return(int64(0), nil)
	mocks.MEVMClient.On("RetryBlockNumber").Return(uint64(10), nil)
	mocks.MEVMClient.On("BlockConfirmations", mock.Anything).Return(uint64(5))
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	abi, err := abi.JSON(strings.NewReader(router.RouterABI))
	if err != nil {
		t.Fatalf("Failed to parse router ABI. Error: [%s]", err)
	}
	mintHashFromAbi := abi.Events["Mint"].ID
	burnHashFromAbi := abi.Events["Burn"].ID
	lockHashFromAbi := abi.Events["Lock"].ID
	unlockHashFromAbi := abi.Events["Unlock"].ID
	burnERC721HashAbi := abi.Events["BurnERC721"].ID
	memberUpdatedHash := abi.Events["MemberUpdated"].ID

	addresses := []common.Address{
		{},
	}

	filterCfg := FilterConfig{
		abi:               abi,
		topics:            topics,
		addresses:         addresses,
		mintHash:          mintHashFromAbi,
		burnHash:          burnHashFromAbi,
		lockHash:          lockHashFromAbi,
		unlockHash:        unlockHashFromAbi,
		burnERC721Hash:    burnERC721HashAbi,
		memberUpdatedHash: memberUpdatedHash,
		maxLogsBlocks:     220,
	}

	assets := config.LoadAssets(testConstants.Networks)
	w = &Watcher{
		repository:        mocks.MStatusRepository,
		contracts:         mocks.MBridgeContractService,
		prometheusService: mocks.MPrometheusService,
		evmClient:         mocks.MEVMClient,
		dbIdentifier:      dbIdentifier,
		logger:            config.GetLoggerFor(fmt.Sprintf("EVM Router Watcher [%s]", dbIdentifier)),
		mappings:          assets,
		validator:         true,
		targetBlock:       5,
		sleepDuration:     defaultSleepDuration,
		filterConfig:      filterCfg,
	}

	actual := NewWatcher(mocks.MStatusRepository, mocks.MBridgeContractService, mocks.MPrometheusService, mocks.MEVMClient, assets, dbIdentifier, 0, true, 15, 220)
	assert.Equal(t, w, actual)
}

// TODO: Test_NewWatcher_Fails

func Test_ProcessLogs_ParseBurnLogFails(t *testing.T) {
	setup()

	query := &ethereum.FilterQuery{
		FromBlock: new(big.Int).SetInt64(0),
		Addresses: []common.Address{
			common.HexToAddress("0x0000000000000000000000000000000000000000"),
		},
		ToBlock: new(big.Int).SetInt64(0),
		Topics:  topics,
	}

	mocks.MEVMClient.On("RetryFilterLogs", *query).
		Return([]types.Log{
			{
				Topics: []common.Hash{
					burnHash,
				},
			},
		}, nil)

	mocks.MBridgeContractService.On("ParseBurnLog", types.Log{
		Topics: []common.Hash{
			burnHash,
		},
	}).Return(burnLog, errors.New("some-error"))
	mocks.MStatusRepository.On("Update", dbIdentifier, int64(1)).Return(nil)
	w.processLogs(0, 0, mocks.MQueue)
	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
}

func Test_ProcessLogs_ParseLockLogFails(t *testing.T) {
	setup()

	query := &ethereum.FilterQuery{
		FromBlock: new(big.Int).SetInt64(0),
		Addresses: []common.Address{
			common.HexToAddress("0x0000000000000000000000000000000000000000"),
		},
		ToBlock: new(big.Int).SetInt64(0),
		Topics:  topics,
	}

	mocks.MEVMClient.On("RetryFilterLogs", *query).
		Return([]types.Log{
			{
				Topics: []common.Hash{
					lockHash,
				},
			},
		}, nil)

	mocks.MBridgeContractService.On("ParseLockLog", types.Log{
		Topics: []common.Hash{
			lockHash,
		},
	}).Return(lockLog, errors.New("some-error"))
	mocks.MStatusRepository.On("Update", dbIdentifier, int64(1)).Return(nil)
	w.processLogs(0, 0, mocks.MQueue)
	mocks.MQueue.AssertNotCalled(t, "Push", mock.Anything)
}

func Test_ProcessLogs_FilterLogsFails(t *testing.T) {
	setup()

	query := &ethereum.FilterQuery{
		FromBlock: new(big.Int).SetInt64(0),
		Addresses: []common.Address{
			common.HexToAddress("0x0000000000000000000000000000000000000000"),
		},
		ToBlock: new(big.Int).SetInt64(5),
		Topics:  topics,
	}

	mocks.MEVMClient.On("RetryFilterLogs", *query).
		Return([]types.Log{}, errors.New("some-error"))

	w.processLogs(0, 5, mocks.MQueue)
}

func Test_ProcessLogs_RepoUpdateFails(t *testing.T) {
	mocks.Setup()
	setup()

	query := &ethereum.FilterQuery{
		FromBlock: new(big.Int).SetInt64(0),
		Addresses: []common.Address{
			common.HexToAddress("0x0000000000000000000000000000000000000000"),
		},
		ToBlock: new(big.Int).SetInt64(0),
		Topics:  topics,
	}
	expectedErr := errors.New("some-error")

	mocks.MEVMClient.On("RetryFilterLogs", *query).
		Return([]types.Log{}, nil)
	mocks.MStatusRepository.On("Update", dbIdentifier, int64(1)).Return(expectedErr)
	res := w.processLogs(0, 0, mocks.MQueue)
	assert.Equal(t, expectedErr, res)
}

func setup() {
	mocks.Setup()

	mocks.MStatusRepository.On("Get", mock.Anything).Return(int64(0), nil)
	mocks.MPrometheusService.On("GetIsMonitoringEnabled").Return(false)

	w = &Watcher{
		repository:        mocks.MStatusRepository,
		contracts:         mocks.MBridgeContractService,
		prometheusService: mocks.MPrometheusService,
		evmClient:         mocks.MEVMClient,
		dbIdentifier:      dbIdentifier,
		logger:            config.GetLoggerFor(fmt.Sprintf("EVM Router Watcher [%s]", dbIdentifier)),
		mappings:          config.LoadAssets(testConstants.Networks),
		validator:         true,
		sleepDuration:     defaultSleepDuration,
		filterConfig:      filterConfig,
	}
}
