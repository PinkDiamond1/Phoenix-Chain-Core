package xcom

import (
	"bytes"
	"encoding/json"
	"math/big"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/log"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rlp"
)

// StateDB is an Plugin database for full state querying.
type StateDB interface {
	CreateAccount(common.Address)

	SubBalance(common.Address, *big.Int)
	AddBalance(common.Address, *big.Int)
	GetBalance(common.Address) *big.Int

	GetNonce(common.Address) uint64
	SetNonce(common.Address, uint64)

	GetCodeHash(common.Address) common.Hash
	GetCode(common.Address) []byte
	SetCode(common.Address, []byte)
	GetCodeSize(common.Address) int

	AddRefund(uint64)
	SubRefund(uint64)
	GetRefund() uint64

	GetCommittedState(common.Address, []byte) []byte
	//GetState(common.Address, common.Hash) common.Hash
	//SetState(common.Address, common.Hash, common.Hash)
	GetState(common.Address, []byte) []byte
	SetState(common.Address, []byte, []byte)

	Suicide(common.Address) bool
	HasSuicided(common.Address) bool

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for suicided accounts.
	Exist(common.Address) bool
	// Empty returns whether the given account is empty. Empty
	// is defined according to EIP161 (balance = nonce = code = 0).
	Empty(common.Address) bool

	RevertToSnapshot(int)
	Snapshot() int

	AddLog(*types.Log)
	AddPreimage(common.Hash, []byte)

	ForEachStorage(common.Address, func([]byte, []byte) bool)

	//dpos add
	TxHash() common.Hash
	TxIdx() uint32

	IntermediateRoot(deleteEmptyObjects bool) common.Hash
}

type Result struct {
	Code uint32
	Ret  interface{}
}

func NewResult(err *common.BizError, data interface{}) []byte {
	var res *Result
	if err != nil && err != common.NoErr {
		res = &Result{err.Code, err.Msg}
	} else {
		res = &Result{common.NoErr.Code, data}
	}
	bs, _ := json.Marshal(res)
	return bs
}

// addLog let the result add to event.
func AddLog(state StateDB, blockNumber uint64, contractAddr common.Address, event, data string) {
	AddLogWithRes(state, blockNumber, contractAddr, event, data, nil)
}

// addLog let the result add to event.
func AddLogWithRes(state StateDB, blockNumber uint64, contractAddr common.Address, event, code string, res interface{}) {
	buf := new(bytes.Buffer)
	if res == nil {
		if err := rlp.Encode(buf, [][]byte{[]byte(code)}); nil != err {
			log.Error("Cannot RlpEncode the log data", "data", code, "err", err)
			panic("Cannot RlpEncode the log data")
		}
	} else {
		resByte, err := rlp.EncodeToBytes(res)
		if err != nil {
			log.Error("Cannot RlpEncode the log res", "res", res, "err", err, "event", event)
			panic("Cannot RlpEncode the log data")
		}
		if err := rlp.Encode(buf, [][]byte{[]byte(code), resByte}); nil != err {
			log.Error("Cannot RlpEncode the log data", "data", code, "err", err, "event", event)
			panic("Cannot RlpEncode the log data")
		}

	}

	state.AddLog(&types.Log{
		Address:     contractAddr,
		Topics:      nil, //[]common.Hash{common.BytesToHash(crypto.Keccak256([]byte(event)))},
		Data:        buf.Bytes(),
		BlockNumber: blockNumber,
	})
}
