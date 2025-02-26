package rawdb

import (
	"encoding/json"
	"testing"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/configs"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/xcom"

	"github.com/stretchr/testify/assert"
)

func TestReadWriteEconomicModel(t *testing.T) {

	chainDb := NewMemoryDatabase()
	ec := ReadEconomicModel(chainDb, common.ZeroHash)
	assert.Nil(t, ec, "the ec is not nil")

	WriteEconomicModel(chainDb, common.ZeroHash, xcom.GetEc(xcom.DefaultTestNet))
	ec = ReadEconomicModel(chainDb, common.ZeroHash)
	assert.NotNil(t, ec, "the ec is nil")

	b, _ := json.Marshal(ec)
	t.Log(string(b))
}

func TestReadWriteChainConfig(t *testing.T) {

	chainDb := NewMemoryDatabase()
	config := ReadChainConfig(chainDb, common.ZeroHash)
	assert.Nil(t, config, "the chainConfig is not nil")

	WriteChainConfig(chainDb, common.ZeroHash, configs.MainnetChainConfig)
	config = ReadChainConfig(chainDb, common.ZeroHash)
	assert.NotNil(t, config, "the chainConfig is nil")

}

func TestReadWritePreimages(t *testing.T) {
	blob := []byte("test")
	hash := crypto.Keccak256Hash(blob)

	chainDb := NewMemoryDatabase()
	preimage := ReadPreimage(chainDb, hash)
	assert.Equal(t, 0, len(preimage), "the preimage is not nil")

	preimages := make(map[common.Hash][]byte)
	preimages[hash] = common.CopyBytes(blob)
	WritePreimages(chainDb, preimages)

	preimage = ReadPreimage(chainDb, hash)
	assert.NotEqual(t, 0, len(preimage), "the preimage is nil")
}
