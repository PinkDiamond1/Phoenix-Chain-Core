package light

import (
	"context"
	"errors"
	"fmt"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/state"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/ethdb"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/trie"
)

func NewState(ctx context.Context, head *types.Header, odr OdrBackend) *state.StateDB {
	state, _ := state.New(head.Root, NewStateDatabase(ctx, head, odr))
	return state
}

func NewStateDatabase(ctx context.Context, head *types.Header, odr OdrBackend) state.Database {
	return &odrDatabase{ctx, StateTrieID(head), odr}
}

type odrDatabase struct {
	ctx     context.Context
	id      *TrieID
	backend OdrBackend
}

func (db *odrDatabase) OpenTrie(root common.Hash) (state.Trie, error) {
	return &odrTrie{db: db, id: db.id}, nil
}

func (db *odrDatabase) OpenStorageTrie(addrHash, root common.Hash) (state.Trie, error) {
	return &odrTrie{db: db, id: StorageTrieID(db.id, addrHash, root)}, nil
}

func (db *odrDatabase) CopyTrie(t state.Trie) state.Trie {
	switch t := t.(type) {
	case *odrTrie:
		cpy := &odrTrie{db: t.db, id: t.id}
		if t.trie != nil {
			cpytrie := *t.trie
			cpy.trie = &cpytrie
		}
		return cpy
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

func (db *odrDatabase) NewTrie(t state.Trie) state.Trie {
	return nil
}

func (db *odrDatabase) ContractCode(addrHash, codeHash common.Hash) ([]byte, error) {
	if codeHash == sha3Nil {
		return nil, nil
	}
	if code, err := db.backend.Database().Get(codeHash[:]); err == nil {
		return code, nil
	}
	id := *db.id
	id.AccKey = addrHash[:]
	req := &CodeRequest{Id: &id, Hash: codeHash}
	err := db.backend.Retrieve(db.ctx, req)
	return req.Data, err
}

func (db *odrDatabase) ContractCodeSize(addrHash, codeHash common.Hash) (int, error) {
	code, err := db.ContractCode(addrHash, codeHash)
	return len(code), err
}

func (db *odrDatabase) TrieDB() *trie.Database {
	return nil
}

type odrTrie struct {
	db   *odrDatabase
	id   *TrieID
	trie *trie.Trie
}

func (t *odrTrie) TryGet(key []byte) ([]byte, error) {
	key = crypto.Keccak256(key)
	var res []byte
	err := t.do(key, func() (err error) {
		res, err = t.trie.TryGet(key)
		return err
	})
	return res, err
}

func (t *odrTrie) TryUpdate(key, value []byte) error {
	key = crypto.Keccak256(key)
	return t.do(key, func() error {
		return t.trie.TryUpdate(key, value)
	})
}

func (t *odrTrie) TryDelete(key []byte) error {
	key = crypto.Keccak256(key)
	return t.do(key, func() error {
		return t.trie.TryDelete(key)
	})
}

func (t *odrTrie) Commit(onleaf trie.LeafCallback) (common.Hash, error) {
	if t.trie == nil {
		return t.id.Root, nil
	}
	return t.trie.Commit(onleaf)
}

func (t *odrTrie) ParallelCommit(onleaf trie.LeafCallback) (common.Hash, error) {
	return t.Commit(onleaf)
}

func (t *odrTrie) Hash() common.Hash {
	if t.trie == nil {
		return t.id.Root
	}
	return t.trie.Hash()
}

func (t *odrTrie) ParallelHash() common.Hash {
	return t.Hash()
}

func (t *odrTrie) NodeIterator(startkey []byte) trie.NodeIterator {
	return newNodeIterator(t, startkey)
}

func (t *odrTrie) GetKey(sha []byte) []byte {
	return nil
}

func (t *odrTrie) Prove(key []byte, fromLevel uint, proofDb ethdb.KeyValueWriter) error {
	return errors.New("not implemented, needs client/server interface split")
}

// do tries and retries to execute a function until it returns with no error or
// an error type other than MissingNodeError
func (t *odrTrie) do(key []byte, fn func() error) error {
	for {
		var err error
		if t.trie == nil {
			t.trie, err = trie.New(t.id.Root, trie.NewDatabase(t.db.backend.Database()))
		}
		if err == nil {
			err = fn()
		}
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return err
		}
		r := &TrieRequest{Id: t.id, Key: key}
		if err := t.db.backend.Retrieve(t.db.ctx, r); err != nil {
			return err
		}
	}
}

type nodeIterator struct {
	trie.NodeIterator
	t   *odrTrie
	err error
}

func newNodeIterator(t *odrTrie, startkey []byte) trie.NodeIterator {
	it := &nodeIterator{t: t}
	// Open the actual non-ODR trie if that hasn't happened yet.
	if t.trie == nil {
		it.do(func() error {
			t, err := trie.New(t.id.Root, trie.NewDatabase(t.db.backend.Database()))
			if err == nil {
				it.t.trie = t
			}
			return err
		})
	}
	it.do(func() error {
		it.NodeIterator = it.t.trie.NodeIterator(startkey)
		return it.NodeIterator.Error()
	})
	return it
}

func (it *nodeIterator) Next(descend bool) bool {
	var ok bool
	it.do(func() error {
		ok = it.NodeIterator.Next(descend)
		return it.NodeIterator.Error()
	})
	return ok
}

// do runs fn and attempts to fill in missing nodes by retrieving.
func (it *nodeIterator) do(fn func() error) {
	var lasthash common.Hash
	for {
		it.err = fn()
		missing, ok := it.err.(*trie.MissingNodeError)
		if !ok {
			return
		}
		if missing.NodeHash == lasthash {
			it.err = fmt.Errorf("retrieve loop for trie node %x", missing.NodeHash)
			return
		}
		lasthash = missing.NodeHash
		r := &TrieRequest{Id: it.t.id, Key: nibblesToKey(missing.Path)}
		if it.err = it.t.db.backend.Retrieve(it.t.db.ctx, r); it.err != nil {
			return
		}
	}
}

func (it *nodeIterator) Error() error {
	if it.err != nil {
		return it.err
	}
	return it.NodeIterator.Error()
}

func nibblesToKey(nib []byte) []byte {
	if len(nib) > 0 && nib[len(nib)-1] == 0x10 {
		nib = nib[:len(nib)-1] // drop terminator
	}
	if len(nib)&1 == 1 {
		nib = append(nib, 0) // make even
	}
	key := make([]byte, len(nib)/2)
	for bi, ni := 0, 0; ni < len(nib); bi, ni = bi+1, ni+2 {
		key[bi] = nib[ni]<<4 | nib[ni+1]
	}
	return key
}
