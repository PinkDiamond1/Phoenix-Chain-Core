package vm

import (
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/xcom"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common/vm"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/log"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/reward"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/configs"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rlp"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/plugin"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/db/snapshotdb"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/p2p/discover"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common/mock"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/staking"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/xutil"
)

func generateStk(rewardPer uint16, delegateTotal *big.Int, blockNumber uint64) (staking.ValArrIndexQueue, staking.ValidatorQueue, staking.Candidate, staking.Delegation) {
	var canMu staking.CandidateMutable
	canMu.Released = big.NewInt(10000)
	canMu.RewardPer = rewardPer
	canMu.DelegateTotal = delegateTotal
	canMu.CurrentEpochDelegateReward = new(big.Int)

	var canBase staking.CandidateBase
	privateKey, err := crypto.GenerateKey()
	if nil != err {
		panic(err)
	}
	nodeID, add := discover.PubkeyID(&privateKey.PublicKey), crypto.PubkeyToAddress(privateKey.PublicKey)
	canBase.BenefitAddress = add
	canBase.NodeId = nodeID
	canBase.StakingBlockNum = 100

	var delegation staking.Delegation
	delegation.Released = delegateTotal
	delegation.DelegateEpoch = uint32(xutil.CalculateEpoch(blockNumber))

	stakingValIndex := make(staking.ValArrIndexQueue, 0)
	stakingValIndex = append(stakingValIndex, &staking.ValArrIndex{
		Start: 0,
		End:   xutil.CalcBlocksEachEpoch(),
	})
	stakingValIndex = append(stakingValIndex, &staking.ValArrIndex{
		Start: xutil.CalcBlocksEachEpoch(),
		End:   xutil.CalcBlocksEachEpoch() * 2,
	})
	stakingValIndex = append(stakingValIndex, &staking.ValArrIndex{
		Start: xutil.CalcBlocksEachEpoch() * 2,
		End:   xutil.CalcBlocksEachEpoch() * 3,
	})
	stakingValIndex = append(stakingValIndex, &staking.ValArrIndex{
		Start: xutil.CalcBlocksEachEpoch() * 3,
		End:   xutil.CalcBlocksEachEpoch() * 4,
	})
	validatorQueue := make(staking.ValidatorQueue, 0)
	validatorQueue = append(validatorQueue, &staking.Validator{
		NodeId:          nodeID,
		NodeAddress:     common.NodeAddress(canBase.BenefitAddress),
		StakingBlockNum: canBase.StakingBlockNum,
	})

	return stakingValIndex, validatorQueue, staking.Candidate{&canBase, &canMu}, delegation
}

func TestWithdrawDelegateRewardWithReward(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if nil != err {
		panic(err)
	}
	delegateRewardAdd := crypto.PubkeyToAddress(privateKey.PublicKey)

	privateKey2, err := crypto.GenerateKey()
	if nil != err {
		panic(err)
	}
	coinBase := crypto.PubkeyToAddress(privateKey2.PublicKey)

	chain := mock.NewChain()
	defer chain.SnapDB.Clear()

	chain.SetCoinbaseGenerate(func() common.Address {
		return coinBase
	})

	stkDB := staking.NewStakingDBWithDB(chain.SnapDB)
	index, queue, can, delegate := generateStk(1000, big.NewInt(configs.PHC*3), 10)
	chain.AddBlockWithSnapDB(true, func(hash common.Hash, header *types.Header, sdb snapshotdb.DB) error {
		if err := stkDB.SetEpochValIndex(hash, index); err != nil {
			return err
		}
		if err := stkDB.SetEpochValList(hash, index[0].Start, index[0].End, queue); err != nil {
			return err
		}
		if err := stkDB.SetCanBaseStore(hash, queue[0].NodeAddress, can.CandidateBase); err != nil {
			return err
		}
		if err := stkDB.SetCanMutableStore(hash, queue[0].NodeAddress, can.CandidateMutable); err != nil {
			return err
		}
		if err := stkDB.SetDelegateStore(hash, delegateRewardAdd, can.CandidateBase.NodeId, can.CandidateBase.StakingBlockNum, &delegate); err != nil {
			return err
		}
		return nil
	}, nil, nil)
	initGas := uint64(10000)

	contact := newRewardContact(delegateRewardAdd, chain, initGas)

	contact.Plugin.SetCurrentNodeID(can.NodeId)

	blockReward, stakingReward := big.NewInt(100000), big.NewInt(200000)

	for i := 0; i < int(xutil.CalcBlocksEachEpoch()); i++ {
		if err := chain.AddBlockWithSnapDB(true, func(hash common.Hash, header *types.Header, sdb snapshotdb.DB) error {
			if xutil.IsBeginOfEpoch(header.Number.Uint64()) {
				can.CandidateMutable.CleanCurrentEpochDelegateReward()
				if err := stkDB.SetCanMutableStore(hash, queue[0].NodeAddress, can.CandidateMutable); err != nil {
					return err
				}
			}

			if err := contact.Plugin.AllocatePackageBlock(hash, header, blockReward, chain.StateDB); err != nil {
				return err
			}
			if xutil.IsEndOfEpoch(header.Number.Uint64()) {

				verifierList, err := contact.Plugin.AllocateStakingReward(header.Number.Uint64(), hash, stakingReward, chain.StateDB)
				if err != nil {
					return err
				}
				if err := contact.Plugin.HandleDelegatePerReward(hash, header.Number.Uint64(), verifierList, chain.StateDB); err != nil {
					return err
				}

				if err := stkDB.SetEpochValList(hash, index[xutil.CalculateEpoch(header.Number.Uint64())].Start, index[xutil.CalculateEpoch(header.Number.Uint64())].End, queue); err != nil {
					return err
				}

			}
			return nil
		}, nil, nil); err != nil {
			t.Error(err)
			return
		}

	}

	txhash := common.HexToHash("0x00000000000000000000000000000000000000886d5ba2d3dfb2e2f6a1814f22")

	if err := chain.AddBlockWithTxHashAndCommit(txhash, true, func(hash common.Hash, header *types.Header, sdb snapshotdb.DB) error {
		contact.Evm.BlockHash = hash
		contact.Evm.BlockNumber = chain.CurrentHeader().Number
		if _, err := contact.withdrawDelegateReward(); err != nil {
			t.Error(err)
			return err
		}
		var m [][]byte
		if err := rlp.DecodeBytes(chain.StateDB.GetLogs(txhash)[0].Data, &m); err != nil {
			return err
		}
		var code string
		rewards := make([]reward.NodeDelegateReward, 0)

		if err := rlp.DecodeBytes(m[0], &code); err != nil {
			return err
		}
		if err := rlp.DecodeBytes(m[1], &rewards); err != nil {
			return err
		}
		//if contact.Contract.Gas != 700 {
		//	return errors.New("gas must be 700 left")
		//}

		if code != "0" {
			return errors.New("code must same")
		}
		if len(rewards) == 0 {
			return errors.New("rewards must not be zero")
		}
		if rewards[0].NodeID != can.NodeId {
			return errors.New("node id should be same")
		}
		if rewards[0].StakingNum != can.StakingBlockNum {
			return errors.New("StakingNum  should be same")
		}
		log.Debug("reward", "coinbase", chain.StateDB.GetBalance(coinBase), "delegateRewardAdd", chain.StateDB.GetBalance(delegateRewardAdd), "delegateReward poll",
			chain.StateDB.GetBalance(vm.DelegateRewardPoolAddr), "can address", chain.StateDB.GetBalance(can.BenefitAddress), "reward_pool",
			chain.StateDB.GetBalance(vm.RewardManagerPoolAddr))
		return nil
	}); err != nil {
		t.Error(err)
	}

}

func newRewardContact(add common.Address, chain *mock.Chain, initGas uint64) *DelegateRewardContract {
	callerAddress := AccountRef(sender)
	contact := new(DelegateRewardContract)
	contact.Contract = NewContract(callerAddress, callerAddress, nil, initGas)
	contact.Contract.CallerAddress = add
	contact.Evm = &EVM{
		StateDB: chain.StateDB,
		Context: Context{
			BlockNumber: chain.CurrentHeader().Number,
			BlockHash:   chain.CurrentHeader().Hash(),
		},
	}
	contact.Plugin = plugin.RewardMgrInstance()
	contact.stkPlugin = plugin.StakingInstance()
	return contact
}

func TestWithdrawDelegateRewardWithEmptyReward(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if nil != err {
		panic(err)
	}
	add := crypto.PubkeyToAddress(privateKey.PublicKey)
	chain := mock.NewChain()
	defer chain.SnapDB.Clear()
	initGas := uint64(10000)
	contact := newRewardContact(add, chain, initGas)
	txHash := common.HexToHash("0x00000000000000000000000000000000000000886d5ba2d3dfb2e2f6a1814f22")
	chain.AddBlockWithTxHash(txHash)
	if _, err := contact.withdrawDelegateReward(); err == nil {
		t.Error(err)
		return
	}

	var m [][]byte
	if err := rlp.DecodeBytes(chain.StateDB.GetLogs(txHash)[0].Data, &m); err != nil {
		t.Error(err)
		return
	}
	if contact.Contract.Gas != initGas-configs.WithdrawDelegateRewardGas {
		t.Error("empty gas use must WithdrawDelegateRewardGas")
		return
	}

	if string(m[0]) != "305001" {
		t.Error("code must same")
		return
	}
}

func TestWithdrawDelegateRewardWithMultiNode(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if nil != err {
		panic(err)
	}
	delegateRewardAdd := crypto.PubkeyToAddress(privateKey.PublicKey)

	privateKey2, err := crypto.GenerateKey()
	if nil != err {
		panic(err)
	}
	coinBase := crypto.PubkeyToAddress(privateKey2.PublicKey)

	chain := mock.NewChain()
	defer chain.SnapDB.Clear()

	chain.SetCoinbaseGenerate(func() common.Address {
		return coinBase
	})

	stkDB := staking.NewStakingDBWithDB(chain.SnapDB)
	index, queue, can, delegate := generateStk(1000, big.NewInt(configs.PHC*3), xutil.CalcBlocksEachEpoch()*2+10)
	_, queue2, can2, delegate2 := generateStk(1000, big.NewInt(configs.PHC*3), 10)
	queue = append(queue, queue2...)
	_, queue3, can3, delegate3 := generateStk(1000, big.NewInt(configs.PHC*3), xutil.CalcBlocksEachEpoch()+10)
	queue = append(queue, queue3...)
	chain.AddBlockWithSnapDB(true, func(hash common.Hash, header *types.Header, sdb snapshotdb.DB) error {
		if err := stkDB.SetEpochValIndex(hash, index); err != nil {
			return err
		}
		if err := stkDB.SetEpochValList(hash, index[0].Start, index[0].End, queue); err != nil {
			return err
		}

		if err := stkDB.SetCanBaseStore(hash, queue[0].NodeAddress, can.CandidateBase); err != nil {
			return err
		}
		if err := stkDB.SetCanMutableStore(hash, queue[0].NodeAddress, can.CandidateMutable); err != nil {
			return err
		}

		if err := stkDB.SetCanBaseStore(hash, queue[1].NodeAddress, can2.CandidateBase); err != nil {
			return err
		}
		if err := stkDB.SetCanMutableStore(hash, queue[1].NodeAddress, can2.CandidateMutable); err != nil {
			return err
		}

		if err := stkDB.SetCanBaseStore(hash, queue[2].NodeAddress, can3.CandidateBase); err != nil {
			return err
		}
		if err := stkDB.SetCanMutableStore(hash, queue[2].NodeAddress, can3.CandidateMutable); err != nil {
			return err
		}

		if err := stkDB.SetDelegateStore(hash, delegateRewardAdd, can.CandidateBase.NodeId, can.CandidateBase.StakingBlockNum, &delegate); err != nil {
			return err
		}
		if err := stkDB.SetDelegateStore(hash, delegateRewardAdd, can2.CandidateBase.NodeId, can2.CandidateBase.StakingBlockNum, &delegate2); err != nil {
			return err
		}
		if err := stkDB.SetDelegateStore(hash, delegateRewardAdd, can3.CandidateBase.NodeId, can3.CandidateBase.StakingBlockNum, &delegate3); err != nil {
			return err
		}
		return nil
	}, nil, nil)

	initGas := uint64(5000000)

	contact := newRewardContact(delegateRewardAdd, chain, initGas)

	contact.Plugin.SetCurrentNodeID(can.NodeId)

	blockReward, stakingReward := big.NewInt(100000), big.NewInt(200000)

	for i := 0; i < int(xutil.CalcBlocksEachEpoch()*3); i++ {
		if err := chain.AddBlockWithSnapDB(true, func(hash common.Hash, header *types.Header, sdb snapshotdb.DB) error {
			if xutil.IsBeginOfEpoch(header.Number.Uint64()) {
				can.CandidateMutable.CleanCurrentEpochDelegateReward()
				if err := stkDB.SetCanMutableStore(hash, queue[0].NodeAddress, can.CandidateMutable); err != nil {
					return err
				}
				if err := stkDB.SetCanMutableStore(hash, queue[1].NodeAddress, can2.CandidateMutable); err != nil {
					return err
				}
				if err := stkDB.SetCanMutableStore(hash, queue[2].NodeAddress, can3.CandidateMutable); err != nil {
					return err
				}
			}

			if err := contact.Plugin.AllocatePackageBlock(hash, header, blockReward, chain.StateDB); err != nil {
				return err
			}
			if xutil.IsEndOfEpoch(header.Number.Uint64()) {

				verifierList, err := contact.Plugin.AllocateStakingReward(header.Number.Uint64(), hash, stakingReward, chain.StateDB)
				if err != nil {
					return err
				}
				if err := contact.Plugin.HandleDelegatePerReward(hash, header.Number.Uint64(), verifierList, chain.StateDB); err != nil {
					return err
				}

				if err := stkDB.SetEpochValList(hash, index[xutil.CalculateEpoch(header.Number.Uint64())].Start, index[xutil.CalculateEpoch(header.Number.Uint64())].End, queue); err != nil {
					return err
				}

			}
			return nil
		}, nil, nil); err != nil {
			t.Error(err)
			return
		}

	}

	txhash := common.HexToHash("0x00000000000000000000000000000000000000886d5ba2d3dfb2e2f6a1814f22")
	if err := chain.AddBlockWithTxHashAndCommit(txhash, true, func(hash common.Hash, header *types.Header, sdb snapshotdb.DB) error {
		contact.Evm.BlockHash = hash
		contact.Evm.BlockNumber = chain.CurrentHeader().Number
		if _, err := contact.withdrawDelegateReward(); err != nil {
			t.Error(err)
			return err
		}
		var m [][]byte
		if err := rlp.DecodeBytes(chain.StateDB.GetLogs(txhash)[0].Data, &m); err != nil {
			return err
		}
		var code string
		rewards := make([]reward.NodeDelegateReward, 0)

		if err := rlp.DecodeBytes(m[0], &code); err != nil {
			return err
		}
		if err := rlp.DecodeBytes(m[1], &rewards); err != nil {
			return err
		}

		if code != "0" {
			return errors.New("code must same")
		}
		if len(rewards) == 0 {
			return errors.New("rewards must not be zero")
		}
		assert.True(t, len(rewards) == int(xcom.TheNumberOfDelegationsReward()))
		assert.True(t, rewards[0].NodeID == can2.NodeId)
		assert.True(t, rewards[1].NodeID == can3.NodeId)
		return nil
	}); err != nil {
		t.Error(err)
	}
}
