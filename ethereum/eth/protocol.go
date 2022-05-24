package eth

import (
	"fmt"
	"io"
	"math/big"

	"Phoenix-Chain-Core/libs/common"
	"Phoenix-Chain-Core/ethereum/core"
	"Phoenix-Chain-Core/ethereum/core/types"
	"Phoenix-Chain-Core/libs/event"
	"Phoenix-Chain-Core/libs/rlp"
)

// Constants to match up protocol versions and messages
const (
	eth62 = 62
	eth63 = 63
	eth65 = 65
)

// ProtocolName is the official short name of the protocol used during capability negotiation.
var ProtocolName = "phoenixchain"

// ProtocolVersions are the upported versions of the eth protocol (first is primary).
var ProtocolVersions = []uint{eth65, eth63, eth62}

// ProtocolLengths are the number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{40, 23, 8}

const ProtocolMaxMsgSize = 10 * 1024 * 1024 // Maximum cap on the size of a protocol message

// eth protocol message codes
const (
	// Protocol messages belonging to eth/62
	StatusMsg          = 0x00
	NewBlockHashesMsg  = 0x01
	TxMsg              = 0x02
	GetBlockHeadersMsg = 0x03
	BlockHeadersMsg    = 0x04
	GetBlockBodiesMsg  = 0x05
	BlockBodiesMsg     = 0x06
	NewBlockMsg        = 0x07
	PrepareBlockMsg    = 0x08
	BlockSignatureMsg  = 0x09

	PongMsg = 0x0a

	// Protocol messages belonging to eth/63
	GetNodeDataMsg       = 0x0d
	NodeDataMsg          = 0x0e
	GetReceiptsMsg       = 0x0f
	ReceiptsMsg          = 0x10
	GetDPOSStorageMsg    = 0x11
	DPOSStorageMsg       = 0x12
	GetOriginAndPivotMsg = 0x13
	OriginAndPivotMsg    = 0x14
	DPOSInfoMsg          = 0x15

	// For transaction fetcher
	NewPooledTransactionHashesMsg = 0x16
	GetPooledTransactionsMsg      = 0x17
	PooledTransactionsMsg         = 0x18
)

type errCode int

const (
	ErrMsgTooLarge = iota
	ErrDecode
	ErrInvalidMsgCode
	ErrProtocolVersionMismatch
	ErrNetworkIdMismatch
	ErrGenesisBlockMismatch
	ErrBlockMismatch
	ErrNoStatusMsg
	ErrExtraStatusMsg
	ErrSuspendedPeer
)

func (e errCode) String() string {
	return errorToString[int(e)]
}

// XXX change once legacy code is out
var errorToString = map[int]string{
	ErrMsgTooLarge:             "Message too long",
	ErrDecode:                  "Invalid message",
	ErrInvalidMsgCode:          "Invalid message code",
	ErrProtocolVersionMismatch: "Protocol version mismatch",
	ErrNetworkIdMismatch:       "NetworkId mismatch",
	ErrGenesisBlockMismatch:    "Genesis block mismatch",
	ErrNoStatusMsg:             "No status message",
	ErrExtraStatusMsg:          "Extra status message",
	ErrSuspendedPeer:           "Suspended peer",
}

// NewPooledTransactionHashesPacket represents a transaction announcement packet.
type NewPooledTransactionHashesPacket []common.Hash

// GetPooledTransactionsPacket represents a transaction query.
type GetPooledTransactionsPacket []common.Hash

// PooledTransactionsPacket is the network packet for transaction distribution.
type PooledTransactionsPacket []*types.Transaction

type txPool interface {
	// Has returns an indicator whether txpool has a transaction
	// cached with the given hash.
	Has(hash common.Hash) bool

	// Get retrieves the transaction from local txpool with given
	// tx hash.
	Get(hash common.Hash) *types.Transaction

	// AddRemotes should add the given transactions to the pool.
	AddRemotes([]*types.Transaction) []error

	// Pending should return pending transactions.
	// The slice should be modifiable by the caller.
	Pending() (map[common.Address]types.Transactions, error)

	// SubscribeNewTxsEvent should return an event subscription of
	// NewTxsEvent and send events to the given channel.
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription
}

// statusData is the network packet for the status message.
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64
	BN              *big.Int
	CurrentBlock    common.Hash
	GenesisBlock    common.Hash
}

// newBlockHashesData is the network packet for the block announcements.
type newBlockHashesData []struct {
	Hash   common.Hash // Hash of one particular block being announced
	Number uint64      // Number of one particular block being announced
}

// getBlockHeadersData represents a block header query.
type getBlockHeadersData struct {
	Origin  hashOrNumber // Block from which to retrieve headers
	Amount  uint64       // Maximum number of headers to retrieve
	Skip    uint64       // Blocks to skip between consecutive headers
	Reverse bool         // Query direction (false = rising towards latest, true = falling towards genesis)
}

// hashOrNumber is a combined field for specifying an origin block.
type hashOrNumber struct {
	Hash   common.Hash // Block hash from which to retrieve headers (excludes Number)
	Number uint64      // Block hash from which to retrieve headers (excludes Hash)
}

// EncodeRLP is a specialized encoder for hashOrNumber to encode only one of the
// two contained union fields.
func (hn *hashOrNumber) EncodeRLP(w io.Writer) error {
	if hn.Hash == (common.Hash{}) {
		return rlp.Encode(w, hn.Number)
	}
	if hn.Number != 0 {
		return fmt.Errorf("both origin hash (%x) and number (%d) provided", hn.Hash, hn.Number)
	}
	return rlp.Encode(w, hn.Hash)
}

// DecodeRLP is a specialized decoder for hashOrNumber to decode the contents
// into either a block hash or a block number.
func (hn *hashOrNumber) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	origin, err := s.Raw()
	if err == nil {
		switch {
		case size == 32:
			err = rlp.DecodeBytes(origin, &hn.Hash)
		case size <= 8:
			err = rlp.DecodeBytes(origin, &hn.Number)
		default:
			err = fmt.Errorf("invalid input size %d for origin", size)
		}
	}
	return err
}

// newBlockData is the network packet for the block propagation message.
type newBlockData struct {
	Block *types.Block
}

type prepareBlockData struct {
	Block *types.Block
}

type blockSignature struct {
	SignHash  common.Hash // signature hash，header[0:32]
	Hash      common.Hash // blokc hash，header[:]
	Number    *big.Int
	Signature *common.BlockConfirmSign
}

// blockBody represents the data content of a single block.
type blockBody struct {
	Transactions []*types.Transaction // Transactions contained within a block
	ExtraData    []byte
}

// blockBodiesData is the network packet for block content distribution.
type blockBodiesData []*blockBody
