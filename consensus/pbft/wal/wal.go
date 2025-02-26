// Package wal implements the similar write-ahead logging for pbft consensus.
package wal

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/consensus/pbft/protocols"

	ctypes "github.com/PhoenixGlobal/Phoenix-Chain-Core/consensus/pbft/types"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/node"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/log"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rlp"
)

const (
	// Wal working directory
	walDir = "wal"
	// Wal database name
	metaDBName = "wal_meta"
)

var (
	chainStateKey      = []byte("chain-state") // Key of chainState to store leveldb
	viewChangeKey      = []byte("view-change") // Key of viewChange to store leveldb
	viewChangeQCSplit  = []byte("qs")
	viewChangeQCPrefix = []byte("view-change-qc") // viewChangeQCPrefix + epoch (uint64 big endian) + viewChangeQCSplit + blockNumber (uint64 big endian) + viewChangeQCSplit + viewNumber (uint64 big endian) -> viewChangeQC
)

var (
	errCreateWalDir         = errors.New("failed to create wal directory")
	errUpdateViewChangeMeta = errors.New("failed to update viewChange meta")
	errGetViewChangeMeta    = errors.New("failed to get viewChange meta")
	errGetChainState        = errors.New("failed to get chainState")
	errGetViewChangeQC      = errors.New("failed to get viewChangeQC")
)

// recoveryChainStateFn is a callback type for recovery chainState to consensus.
type recoveryChainStateFn func(chainState *protocols.ChainState) error

// recoveryConsensusMsgFn is a callback type for recovery message to consensus.
type recoveryConsensusMsgFn func(msg interface{}) error

type ViewChangeMessage struct {
	Epoch      uint64
	ViewNumber uint64
	BlockNumber uint64
	FileID     uint32
	Seq        uint64
}

// Wal encapsulates functions required to update and load consensus state.
type Wal interface {
	UpdateChainState(chainState *protocols.ChainState) error
	LoadChainState(fn recoveryChainStateFn) error
	Write(msg interface{}) error
	WriteSync(msg interface{}) error
	UpdateViewChange(info *ViewChangeMessage) error
	UpdateViewChangeQC(epoch uint64, blockNumber uint64,viewNumber uint64, viewChangeQC *ctypes.ViewChangeQC) error
	GetViewChangeQC(epoch uint64, blockNumber uint64,viewNumber uint64) (*ctypes.ViewChangeQC, error)
	Load(fn recoveryConsensusMsgFn) error
	Close()
	SetMockJournalLimitSize(limit uint64)
}

// emptyWal is a empty implementation for wal
type emptyWal struct {
}

func WalDir(ctx *node.ServiceContext) string {
	return ctx.ResolvePath(walDir)
}

func (w *emptyWal) UpdateChainState(chainState *protocols.ChainState) error {
	return nil
}

func (w *emptyWal) LoadChainState(fn recoveryChainStateFn) error {
	return nil
}

func (w *emptyWal) Write(msg interface{}) error {
	return nil
}

func (w *emptyWal) WriteSync(msg interface{}) error {
	return nil
}

func (w *emptyWal) UpdateViewChange(info *ViewChangeMessage) error {
	return nil
}

func (w *emptyWal) UpdateViewChangeQC(epoch uint64, blockNumber uint64,viewNumber uint64, viewChangeQC *ctypes.ViewChangeQC) error {
	return nil
}

func (w *emptyWal) GetViewChangeQC(epoch uint64, blockNumber uint64,viewNumber uint64) (*ctypes.ViewChangeQC, error) {
	return nil, nil
}

func (w *emptyWal) Load(fn recoveryConsensusMsgFn) error {
	return nil
}

func (w *emptyWal) Close() {
}

func (w *emptyWal) SetMockJournalLimitSize(limit uint64) {
}

// baseWal is a default implementation for wal
type baseWal struct {
	path    string // Wal working directory
	metaDB  IWALDatabase
	journal *journal

	cachedChainState atomic.Value //*protocols.ChainState
}

// NewWal creates a new wal to update and load consensus state.
func NewWal(ctx *node.ServiceContext, specifiedPath string) (Wal, error) {
	if ctx == nil && len(specifiedPath) == 0 {
		return &emptyWal{}, nil
	}
	var (
		originPath = specifiedPath
		metaDB     IWALDatabase
		walPath    string
		journal    *journal
		err        error
	)
	if originPath == "" {
		originPath = WalDir(ctx)
	}

	// Make sure the wal directory exists,If not exist create it.
	ensureWalDir := func(path string) (string, error) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := os.MkdirAll(path, 0700)
			if err != nil {
				return "", errCreateWalDir
			}
		}
		return path, nil
	}
	// Open or create WAL Database
	createMetaDB := func(path, name string) (IWALDatabase, error) {
		db, err := createWalDB(filepath.Join(path, name))
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	if walPath, err = ensureWalDir(originPath); err != nil {
		log.Error("Failed to create wal directory", "err", err)
		return nil, err
	}
	if journal, err = newJournal(walPath); err != nil {
		return nil, err
	}
	if metaDB, err = createMetaDB(walPath, metaDBName); err != nil {
		log.Error("Failed to create wal database", "err", err)
		return nil, err
	}

	wal := &baseWal{
		path:    walPath,
		metaDB:  metaDB,
		journal: journal,
	}

	return wal, nil
}

// UpdateChainState tries to update consensus state to leveldb
func (wal *baseWal) UpdateChainState(chainState *protocols.ChainState) error {
	data, err := rlp.EncodeToBytes(chainState)
	if err != nil {
		return err
	}
	// Write the chainState to the WAL database
	err = wal.metaDB.Put(chainStateKey, data, nil)
	if err != nil {
		return err
	}
	wal.cachedChainState.Store(chainState)
	log.Debug("Success to update chainState")
	return nil
}

// LoadChainState tries to load consensus state from leveldb
func (wal *baseWal) LoadChainState(recovery recoveryChainStateFn) error {
	if wal.cachedChainState.Load() != nil {
		return recovery(wal.cachedChainState.Load().(*protocols.ChainState))
	}
	// open wal database
	data, err := wal.metaDB.Get(chainStateKey)
	if err != nil {
		log.Warn("Failed to get chainState from db, may be the first time to run phoenixchain")
		return nil
	}
	var cs protocols.ChainState
	err = rlp.DecodeBytes(data, &cs)
	if err != nil {
		log.Error("Failed to decode chainState")
		return errGetChainState
	}
	wal.cachedChainState.Store(&cs)
	return recovery(&cs)
}

// Write adds the specified consensus msg to the local disk journal.
// the mode is asynchronous write,the msg will cache in bufio.Writer
func (wal *baseWal) Write(msg interface{}) error {
	return wal.journal.Insert(&Message{
		Timestamp: uint64(time.Now().UnixNano()),
		Data:      msg,
	}, false)
}

// WriteSync adds the specified consensus msg to the local disk journal.
// the mode is synchronous write,the msg will flush to disk immediately
func (wal *baseWal) WriteSync(msg interface{}) error {
	return wal.journal.Insert(&Message{
		Timestamp: uint64(time.Now().UnixNano()),
		Data:      msg,
	}, true)
}

// UpdateViewChange tries to update consensus confirm viewChange to leveldb
func (wal *baseWal) UpdateViewChange(info *ViewChangeMessage) error {
	return wal.updateViewChangeMeta(info)
}

// updateViewChangeMeta update the ViewChange Meta Data to the database.
func (wal *baseWal) updateViewChangeMeta(vc *ViewChangeMessage) error {
	fileID, seq, err := wal.journal.CurrentJournal()
	if err != nil {
		log.Error("Failed to update viewChange meta", "epoch", vc.Epoch, "viewNumber", vc.ViewNumber,"blockNumber", vc.BlockNumber, "err", err)
		return errUpdateViewChangeMeta
	}

	vc.FileID = fileID
	vc.Seq = seq
	data, err := rlp.EncodeToBytes(vc)
	if err != nil {
		return err
	}
	// Write the ViewChangeMeta to the WAL database
	err = wal.metaDB.Put(viewChangeKey, data, nil)
	if err != nil {
		return err
	}
	log.Debug("Success to update viewChange meta", "epoch", vc.Epoch, "viewNumber", vc.ViewNumber, "fileID", fileID, "seq", seq)
	// Delete previous journal logs
	go wal.journal.ExpireJournalFile(fileID)
	return nil
}

// Load tries to load consensus msg from the local disk journal.
// recovery is the callback function
func (wal *baseWal) Load(recovery recoveryConsensusMsgFn) error {
	// open wal database
	data, err := wal.metaDB.Get(viewChangeKey)
	if err != nil {
		log.Warn("Failed to get viewChange meta from db, may be the first time to run phoenixchain")
		return nil
	}
	var vc ViewChangeMessage
	err = rlp.DecodeBytes(data, &vc)
	if err != nil {
		log.Error("Failed to decode viewChange meta")
		return errGetViewChangeMeta
	}

	return wal.journal.LoadJournal(vc.FileID, vc.Seq, recovery)
}

// epochKey = viewChangeQCPrefix + epoch (uint64 big endian) + viewChangeQCSplit
func epochKey(epoch uint64) []byte {
	e := make([]byte, 8)
	binary.BigEndian.PutUint64(e, epoch)
	return append(append(viewChangeQCPrefix, e...), viewChangeQCSplit...)
}

// viewChangeQCKey = viewChangeQCPrefix + epoch (uint64 big endian) + viewChangeQCSplit + viewNumber (uint64 big endian)
func viewChangeQCKey(epoch uint64, blockNumber uint64,viewNumber uint64) []byte {
	e := make([]byte, 8)
	binary.BigEndian.PutUint64(e, epoch)
	b:= make([]byte, 8)
	binary.BigEndian.PutUint64(b, blockNumber)
	v := make([]byte, 8)
	binary.BigEndian.PutUint64(v, viewNumber)
	return append(append(append(append(append(viewChangeQCPrefix, e...), viewChangeQCSplit...), b...),viewChangeQCSplit...),v...)
}

// UpdateViewChangeQC tries to save consensus confirm viewChangeQC to leveldb
func (wal *baseWal) UpdateViewChangeQC(epoch uint64, blockNumber uint64,viewNumber uint64, viewChangeQC *ctypes.ViewChangeQC) error {
	data, err := rlp.EncodeToBytes(viewChangeQC)
	if err != nil {
		return err
	}
	// Write the ViewChangeQC to the WAL database
	err = wal.metaDB.Put(viewChangeQCKey(epoch, blockNumber,viewNumber), data, nil)
	if err != nil {
		return err
	}
	log.Debug("Success to update viewChangeQC", "epoch", epoch, "blockNumber", blockNumber,"viewNumber", viewNumber, "viewChangeQC", viewChangeQC.String())
	go wal.deleteViewChangeQC(epoch - 1)
	return nil
}

// deleteViewChangeQC tries to delete viewChangeQC by gaving epoch
// we keep viewChangeQC only one epoch
// if the higher epoch comes, the lower epoch will be deleted
func (wal *baseWal) deleteViewChangeQC(epoch uint64) {
	it := wal.metaDB.NewIterator(epochKey(epoch), nil)
	for it.Next() {
		key := it.Key()
		wal.metaDB.Delete(key)
	}
}

// GetViewChangeQC retrieves a viewChangeQC from the database by
// epoch, viewNumber if found.
func (wal *baseWal) GetViewChangeQC(epoch uint64, blockNumber uint64,viewNumber uint64) (*ctypes.ViewChangeQC, error) {
	// open wal database
	data, err := wal.metaDB.Get(viewChangeQCKey(epoch, blockNumber,viewNumber))
	if err != nil {
		return nil, err
	}
	var qc ctypes.ViewChangeQC
	err = rlp.DecodeBytes(data, &qc)
	if err != nil {
		log.Error("Failed to decode viewChangeQC")
		return nil, errGetViewChangeQC
	}
	return &qc, nil
}

func (wal *baseWal) Close() {
	wal.metaDB.Close()
	wal.journal.Close()
}

func (wal *baseWal) SetMockJournalLimitSize(limit uint64) {
	wal.journal.mockJournalLimitSize = limit
}
