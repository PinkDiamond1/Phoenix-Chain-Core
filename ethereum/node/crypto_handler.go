package node

import (
	"crypto/ecdsa"
	"encoding/hex"
	"golang.org/x/crypto/sha3"
	"sync"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rlp"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/log"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/p2p/discover"
)

var (
	cryptoHandlerOnce = sync.Once{}
)

var chandler *CryptoHandler

type CryptoHandler struct {
	privateKey *ecdsa.PrivateKey
}

func GetCryptoHandler() *CryptoHandler {
	cryptoHandlerOnce.Do(func() {
		log.Info("Init CryptoHandler ...")
		chandler = &CryptoHandler{}
	})
	return chandler
}

func (chandler *CryptoHandler) SetPrivateKey(privateKey *ecdsa.PrivateKey) {
	chandler.privateKey = privateKey
}

func (chandler *CryptoHandler) Sign(data interface{}) ([]byte, error) {
	if chandler == nil || chandler.privateKey == nil {
		return nil, common.InternalError.Wrap("PrivateKey missed")
	}
	return crypto.Sign(RlpHash(data).Bytes(), chandler.privateKey)
}

func (chandler *CryptoHandler) MustSign(data interface{}) []byte {
	if chandler == nil || chandler.privateKey == nil {
		panic("Private key is missed")
	}
	sig, err := crypto.Sign(RlpHash(data).Bytes(), chandler.privateKey)
	if err != nil {
		panic(err)
	}
	return sig
}

func (chandler *CryptoHandler) IsSignedByNodeID(data interface{}, sig []byte, nodeID discover.NodeID) bool {
	pubKey, err := crypto.SigToPub(RlpHash(data).Bytes(), sig)
	if err != nil {
		log.Error("Check if the signature is signed by a node", "err", err)
		return false
	}
	id := discover.PubkeyID(pubKey)

	if id == nodeID {
		return true
	}
	log.Error("the signature is not signed by the node", "nodeID", hex.EncodeToString(nodeID.Bytes()[:8]))
	return false
}

func RlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
