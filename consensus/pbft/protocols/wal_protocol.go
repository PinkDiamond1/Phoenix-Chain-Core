package protocols

import (
	"fmt"
	"reflect"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"

	ctypes "github.com/PhoenixGlobal/Phoenix-Chain-Core/consensus/pbft/types"
)

const (
	ConfirmedViewChangeMsg = 0x01
	SendViewChangeMsg      = 0x02
	SendPrepareBlockMsg    = 0x03
	SendPrepareVoteMsg     = 0x04
	SendPreCommitMsg     = 0x05
)

const (
	CommitState = 0x05
	LockState   = 0x06
	QCState     = 0x07
	//PreCommitQCState     = 0x08
)

type State struct {
	Block      *types.Block
	QuorumCert *ctypes.QuorumCert
}

func (s *State) ValidState() bool {
	return s.Block != nil && s.QuorumCert != nil && s.Block.NumberU64() == s.QuorumCert.BlockNumber && s.Block.Hash() == s.QuorumCert.BlockHash
}

func (s *State) EqualState(c *State) bool {
	return s.QuorumCert.BlockNumber == c.QuorumCert.BlockNumber && s.QuorumCert.BlockHash == c.QuorumCert.BlockHash
}

func (s *State) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("[blockNumber:%d, blockHash:%s, qcNumber:%d, qcHash:%s]",
		s.Block.NumberU64(), s.Block.Hash().String(), s.QuorumCert.BlockNumber, s.QuorumCert.BlockHash.String())
}

// ChainState indicates the latest consensus state.
type ChainState struct {
	Commit *State
	Lock   *State
	QC     []*State   //PreCommitQC
}

func (cs *ChainState) ValidChainState() bool {
	return cs != nil && cs.Commit != nil && cs.Lock != nil && len(cs.QC) > 0 && cs.Commit.ValidState() && cs.Lock.ValidState()
}

func (cs *ChainState) String() string {
	if cs == nil {
		return ""
	}
	return fmt.Sprintf("[commitState:%s, lockState:%s, qcState:%s]", cs.Commit.String(), cs.Lock.String(), cs.QC[0].String())
}

type WalMsg interface {
	Epoch() uint64
	ViewNumber() uint64
	BlockNumber() uint64
}

// ConfirmedViewChange indicates the latest confirmed view.
type ConfirmedViewChange struct {
	Epoch        uint64
	ViewNumber   uint64
	Block        *types.Block
	QC           *ctypes.QuorumCert   `rlp:"nil"`
	ViewChangeQC *ctypes.ViewChangeQC `rlp:"nil"`
}

func (c *ConfirmedViewChange) String() string {
	if c == nil {
		return ""
	}
	if c.QC != nil {
		return fmt.Sprintf("[epoch:%d, viewNumber:%d, blockIndex:%d, blockNumber:%d, blockHash:%s]",
			c.Epoch, c.ViewNumber, c.QC.BlockIndex, c.QC.BlockNumber, c.QC.BlockHash.String())
	}
	return fmt.Sprintf("[epoch:%d, viewNumber:%d, blockNumber:%d, blockHash:%s]",
		c.Epoch, c.ViewNumber, c.Block.NumberU64(), c.Block.Hash().String())
}

// SendViewChange indicates the viewChange sent by the local node.
type SendViewChange struct {
	ViewChange *ViewChange
}

func (s SendViewChange) Epoch() uint64 {
	return s.ViewChange.Epoch
}

func (s SendViewChange) ViewNumber() uint64 {
	return s.ViewChange.ViewNumber
}

func (s SendViewChange) BlockNumber() uint64 {
	return s.ViewChange.BlockNumber
}

func (s *SendViewChange) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("[epoch:%d, viewNumber:%d, blockNumber:%d, blockHash:%s]",
		s.ViewChange.Epoch, s.ViewChange.ViewNumber, s.ViewChange.BlockNumber, s.ViewChange.BlockHash.String())
}

// SendPrepareBlock indicates the prepareBlock sent by the local node.
type SendPrepareBlock struct {
	Prepare *PrepareBlock
}

func (s SendPrepareBlock) Epoch() uint64 {
	return s.Prepare.Epoch
}

func (s SendPrepareBlock) ViewNumber() uint64 {
	return s.Prepare.ViewNumber
}

func (s SendPrepareBlock) BlockNumber() uint64 {
	return s.Prepare.Block.NumberU64()
}

func (s *SendPrepareBlock) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("[epoch:%d, viewNumber:%d, blockIndex:%d, blockNumber:%d, blockHash:%s]",
		s.Prepare.Epoch, s.Prepare.ViewNumber, s.Prepare.BlockIndex, s.Prepare.Block.NumberU64(), s.Prepare.Block.Hash().String())
}

// SendPrepareVote indicates the prepareVote sent by the local node.
type SendPrepareVote struct {
	Block *types.Block
	Vote  *PrepareVote
}

func (s SendPrepareVote) Epoch() uint64 {
	return s.Vote.Epoch
}

func (s SendPrepareVote) ViewNumber() uint64 {
	return s.Vote.ViewNumber
}

func (s SendPrepareVote) BlockNumber() uint64 {
	return s.Vote.BlockNumber
}

func (s *SendPrepareVote) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("[epoch:%d, viewNumber:%d, blockIndex:%d, blockNumber:%d, blockHash:%s]",
		s.Vote.Epoch, s.Vote.ViewNumber, s.Vote.BlockIndex, s.Vote.BlockNumber, s.Vote.BlockHash.String())
}

// SendPreCommit indicates the prepareVote sent by the local node.
type SendPreCommit struct {
	Block *types.Block
	Vote  *PreCommit
}

func (s SendPreCommit) Epoch() uint64 {
	return s.Vote.Epoch
}

func (s SendPreCommit) ViewNumber() uint64 {
	return s.Vote.ViewNumber
}

func (s SendPreCommit) BlockNumber() uint64 {
	return s.Vote.BlockNumber
}

func (s *SendPreCommit) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("[epoch:%d, viewNumber:%d, blockIndex:%d, blockNumber:%d, blockHash:%s]",
		s.Vote.Epoch, s.Vote.ViewNumber, s.Vote.BlockIndex, s.Vote.BlockNumber, s.Vote.BlockHash.String())
}

var (
	WalMessages = []interface{}{
		ConfirmedViewChange{},
		SendViewChange{},
		SendPrepareBlock{},
		SendPrepareVote{},
		SendPreCommit{},
	}
)

func WalMessageType(msg interface{}) uint64 {
	switch msg.(type) {
	case *ConfirmedViewChange:
		return ConfirmedViewChangeMsg
	case *SendViewChange:
		return SendViewChangeMsg
	case *SendPrepareBlock:
		return SendPrepareBlockMsg
	case *SendPrepareVote:
		return SendPrepareVoteMsg
	case *SendPreCommit:
		return SendPreCommitMsg
	}

	panic(fmt.Sprintf("invalid wal msg type %v", reflect.TypeOf(msg)))
}
