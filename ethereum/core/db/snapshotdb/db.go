package snapshotdb

import (
	"errors"
	"fmt"
	"path"
	"sort"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rlp"

	"github.com/syndtr/goleveldb/leveldb"

	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
)

func getBaseDBPath(dbpath string) string {
	return path.Join(dbpath, DBBasePath)
}

func (s *snapshotDB) getBlockFromWal(block []byte) (*blockData, error) {
	bk := new(blockData)
	if err := rlp.DecodeBytes(block, bk); err != nil {
		return nil, err
	}
	return bk, nil
}

type blockOrigin []struct {
	Number uint64
	key    []byte
	Val    []byte
}

func (u blockOrigin) Len() int {
	return len(u)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (u blockOrigin) Less(i, j int) bool {
	return u[i].Number < u[j].Number
}

// Swap swaps the elements with indexes i and j.
func (u blockOrigin) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}

func (s *snapshotDB) recover() error {
	baseNum := s.current.GetBase(false).Num
	highestNum := s.current.GetHighest(false).Num

	walNeedDelete := leveldb.MakeBatch(0)
	itr := s.baseDB.NewIterator(util.BytesPrefix([]byte(WalKeyPrefix)), nil)
	defer itr.Release()

	var sortBlockWals blockOrigin
	for itr.Next() {
		blockNum := DecodeWalKey(itr.Key())
		sortBlockWals = append(sortBlockWals, struct {
			Number uint64
			key    []byte
			Val    []byte
		}{Number: blockNum.Uint64(), key: common.CopyBytes(itr.Key()), Val: common.CopyBytes(itr.Value())})
	}
	sort.Sort(sortBlockWals)
	if len(sortBlockWals) > 0 {
		if sortBlockWals[0].Number > baseNum.Uint64()+1 {
			return fmt.Errorf("wal is not enough,want recover  from %v,have %v", baseNum.Uint64()+1, sortBlockWals[0].Number)
		}
		var journalBrokenNum uint64

		for _, wal := range sortBlockWals {
			if journalBrokenNum != 0 && wal.Number >= journalBrokenNum {
				logger.Info("recovering, block is wrong,remove it", "num", wal.Number)
				walNeedDelete.Delete(common.CopyBytes(wal.key))
				continue
			}
			if baseNum.Uint64() < wal.Number && wal.Number <= highestNum.Uint64() {
				if blockchain != nil {
					if header := blockchain.GetHeaderByNumber(wal.Number); header == nil {
						logger.Info("recovering, block is not  exist in chain,remove it", "num", wal.Number)
						walNeedDelete.Delete(common.CopyBytes(wal.key))
						journalBrokenNum = wal.Number
						continue
					}
				}
				block, err := s.getBlockFromWal(wal.Val)
				if err != nil {
					logger.Info("recovering, block is broken,remove it", "num", wal.Number, "err", err)
					walNeedDelete.Delete(common.CopyBytes(wal.key))
					journalBrokenNum = wal.Number
					continue
				} else {
					s.committed = append(s.committed, block)
					logger.Debug("recover block ", "num", block.Number, "hash", block.BlockHash.String())
				}
			} else {
				walNeedDelete.Delete(common.CopyBytes(wal.Val))
				logger.Info("recovering, block is less than baseNum or greater than  highestNum,remove it", "num", wal.Number)
			}
		}
	}

	if len(s.committed) > 0 {
		block := s.committed[len(s.committed)-1]
		if err := s.SetCurrent(block.BlockHash, *baseNum, *block.Number); err != nil {
			return err
		}
	} else {
		//no recover block,so set current highest and base the same
		if err := s.SetCurrent(common.ZeroHash, *baseNum, *baseNum); err != nil {
			return err
		}
	}
	if err := s.baseDB.Write(walNeedDelete, nil); err != nil {
		return err
	}
	return nil
}

func (s *snapshotDB) getUnRecognizedHash() common.Hash {
	return common.ZeroHash
}

func (s *snapshotDB) put(hash common.Hash, key, value []byte) error {
	s.unCommit.Lock()
	defer s.unCommit.Unlock()
	block, ok := s.unCommit.blocks[hash]
	if !ok {
		return fmt.Errorf("not find the block by hash:%v", hash.String())
	}
	if block.readOnly {
		return errors.New("can't put read only block")
	}
	return block.Write(key, value)
}
