// This file contains some shares testing functionality, common to  multiple
// different files and modules being tested.

package les

import (
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/eth"
	"crypto/rand"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/db/rawdb"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/configs"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/consensus"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/vm"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/les/flowcontrol"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/light"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/p2p"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/p2p/discover"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/ethdb"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/event"
	_ "github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/xcom"
)

var (
	testBankKey, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds   = big.NewInt(1000000000000000000)

	acc1Key, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	acc2Key, _ = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
	acc1Addr   = crypto.PubkeyToAddress(acc1Key.PublicKey)
	acc2Addr   = crypto.PubkeyToAddress(acc2Key.PublicKey)

	testContractCode         = common.Hex2Bytes("606060405260cc8060106000396000f360606040526000357c01000000000000000000000000000000000000000000000000000000009004806360cd2685146041578063c16431b914606b57603f565b005b6055600480803590602001909190505060a9565b6040518082815260200191505060405180910390f35b60886004808035906020019091908035906020019091905050608a565b005b80600060005083606481101560025790900160005b50819055505b5050565b6000600060005082606481101560025790900160005b5054905060c7565b91905056")
	testContractAddr         common.Address
	testContractCodeDeployed = testContractCode[16:]
	testContractDeployed     = uint64(2)

	testEventEmitterCode = common.Hex2Bytes("60606040523415600e57600080fd5b7f57050ab73f6b9ebdd9f76b8d4997793f48cf956e965ee070551b9ca0bb71584e60405160405180910390a160358060476000396000f3006060604052600080fd00a165627a7a723058203f727efcad8b5811f8cb1fc2620ce5e8c63570d697aef968172de296ea3994140029")
	testEventEmitterAddr common.Address

	testBufLimit = uint64(100)
)

/*
contract test {

    uint256[100] data;

    function Put(uint256 addr, uint256 value) {
        data[addr] = value;
    }

    function Get(uint256 addr) constant returns (uint256 value) {
        return data[addr];
    }
}
*/

func testChainGen(i int, block *core.BlockGen) {
	signer := types.NewEIP155Signer(configs.TestChainConfig.ChainID)

	switch i {
	case 0:
		// In block 1, the test bank sends account #1 some ether.
		tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), acc1Addr, big.NewInt(10000), configs.TxGas, nil, nil), signer, testBankKey)
		block.AddTx(tx)
	case 1:
		// In block 2, the test bank sends some more ether to account #1.
		// acc1Addr passes it on to account #2.
		// acc1Addr creates a test contract.
		// acc1Addr creates a test event.
		nonce := block.TxNonce(acc1Addr)

		tx1, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), acc1Addr, big.NewInt(1000), configs.TxGas, nil, nil), signer, testBankKey)
		tx2, _ := types.SignTx(types.NewTransaction(nonce, acc2Addr, big.NewInt(1000), configs.TxGas, nil, nil), signer, acc1Key)

		block.AddTx(tx1)
		block.AddTx(tx2)
	case 2:
		// Block 3 is empty but was mined by account #2.
		block.SetCoinbase(acc2Addr)
		block.SetExtra(common.FromHex("0xd782070186706c61746f6e86676f312e3131856c696e757800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"))
		data := common.Hex2Bytes("C16431B900000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001")
		tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), testContractAddr, big.NewInt(0), 100000, nil, data), signer, testBankKey)
		block.AddTx(tx)
	case 3:
		// Block 4 includes blocks 2 and 3 as uncle headers (with modified extra data).
		b2 := block.PrevBlock(1).Header()
		b2.Extra = common.FromHex("0xd782070186706c61746f6e86676f312e3131856c696e757800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
		b3 := block.PrevBlock(2).Header()
		b3.Extra = common.FromHex("0xd782070186706c61746f6e86676f312e3131856c696e757800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
		data := common.Hex2Bytes("C16431B900000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000002")
		tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), testContractAddr, big.NewInt(0), 100000, nil, data), signer, testBankKey)
		block.AddTx(tx)
	}
}

// testIndexers creates a set of indexers with specified params for testing purpose.
func testIndexers(db ethdb.Database, odr light.OdrBackend, iConfig *light.IndexerConfig) (*core.ChainIndexer, *core.ChainIndexer, *core.ChainIndexer) {
	chtIndexer := light.NewChtIndexer(db, odr, iConfig.ChtSize, iConfig.ChtConfirms)
	bloomIndexer := eth.NewBloomIndexer(db, iConfig.BloomSize, iConfig.BloomConfirms)
	bloomTrieIndexer := light.NewBloomTrieIndexer(db, odr, iConfig.BloomSize, iConfig.BloomTrieSize)
	bloomIndexer.AddChildIndexer(bloomTrieIndexer)
	return chtIndexer, bloomIndexer, bloomTrieIndexer
}

func testRCL() RequestCostList {
	cl := make(RequestCostList, len(reqList))
	for i, code := range reqList {
		cl[i].MsgCode = code
		cl[i].BaseCost = 0
		cl[i].ReqCost = 0
	}
	return cl
}

// newTestProtocolManager creates a new protocol manager for testing purposes,
// with the given number of blocks already known, potential notification
// channels for different events and relative chain indexers array.
func newTestProtocolManager(lightSync bool, blocks int, generator func(int, *core.BlockGen), odr *LesOdr, peers *peerSet, db ethdb.Database) (*ProtocolManager, error) {
	var (
		evmux  = new(event.TypeMux)
		engine = consensus.NewFaker()
		gspec  = core.Genesis{
			Config: configs.TestChainConfig,
			Alloc:  core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
		}
		genesis = gspec.MustCommit(db)
		chain   BlockChain
	)
	if peers == nil {
		peers = newPeerSet()
	}

	if lightSync {
		chain, _ = light.NewLightChain(odr, gspec.Config, engine)
	} else {
		blockchain, _ := core.NewBlockChain(db, nil, gspec.Config, engine, vm.Config{}, nil)
		gchain, _ := core.GenerateChain(gspec.Config, genesis, consensus.NewFaker(), db, blocks, generator)
		if _, err := blockchain.InsertChain(gchain); err != nil {
			panic(err)
		}
		chain = blockchain
	}

	indexConfig := light.TestServerIndexerConfig
	if lightSync {
		indexConfig = light.TestClientIndexerConfig
	}
	pm, err := NewProtocolManager(gspec.Config, indexConfig, lightSync, NetworkId, evmux, engine, peers, chain, nil, db, odr, nil, nil, make(chan struct{}), new(sync.WaitGroup))
	if err != nil {
		return nil, err
	}
	if !lightSync {
		srv := &LesServer{lesCommons: lesCommons{protocolManager: pm}}
		pm.server = srv

		srv.defParams = &flowcontrol.ServerParams{
			BufLimit:    testBufLimit,
			MinRecharge: 1,
		}

		srv.fcManager = flowcontrol.NewClientManager(50, 10, 1000000000)
		srv.fcCostStats = newCostStats(nil)
	}
	pm.Start(1000)
	return pm, nil
}

// newTestProtocolManagerMust creates a new protocol manager for testing purposes,
// with the given number of blocks already known, potential notification
// channels for different events and relative chain indexers array. In case of an error, the constructor force-
// fails the test.
func newTestProtocolManagerMust(t *testing.T, lightSync bool, blocks int, generator func(int, *core.BlockGen), odr *LesOdr, peers *peerSet, db ethdb.Database) *ProtocolManager {
	pm, err := newTestProtocolManager(lightSync, blocks, generator, odr, peers, db)
	if err != nil {
		t.Fatalf("Failed to create protocol manager: %v", err)
	}
	return pm
}

// testPeer is a simulated peer to allow testing direct network calls.
type testPeer struct {
	net p2p.MsgReadWriter // Network layer reader/writer to simulate remote messaging
	app *p2p.MsgPipeRW    // Application layer reader/writer to simulate the local side
	*peer
}

// newTestPeer creates a new peer registered at the given protocol manager.
func newTestPeer(t *testing.T, name string, version int, pm *ProtocolManager, shake bool) (*testPeer, <-chan error) {
	// Create a message pipe to communicate through
	app, net := p2p.MsgPipe()

	// Generate a random id and create the peer
	var id discover.NodeID
	rand.Read(id[:])

	peer := pm.newPeer(version, NetworkId, p2p.NewPeer(id, name, nil), net)

	// Start the peer on a new thread
	errc := make(chan error, 1)
	go func() {
		select {
		case pm.newPeerCh <- peer:
			errc <- pm.handle(peer)
		case <-pm.quitSync:
			errc <- p2p.DiscQuitting
		}
	}()
	tp := &testPeer{
		app:  app,
		net:  net,
		peer: peer,
	}
	// Execute any implicitly requested handshakes and return
	if shake {
		var (
			genesis = pm.blockchain.Genesis()
			head    = pm.blockchain.CurrentHeader()
		)
		tp.handshake(t, head.Hash(), head.Number.Uint64(), genesis.Hash())
	}
	return tp, errc
}

func newTestPeerPair(name string, version int, pm, pm2 *ProtocolManager) (*peer, <-chan error, *peer, <-chan error) {
	// Create a message pipe to communicate through
	app, net := p2p.MsgPipe()

	// Generate a random id and create the peer
	var id discover.NodeID
	rand.Read(id[:])

	peer := pm.newPeer(version, NetworkId, p2p.NewPeer(id, name, nil), net)
	peer2 := pm2.newPeer(version, NetworkId, p2p.NewPeer(id, name, nil), app)

	// Start the peer on a new thread
	errc := make(chan error, 1)
	errc2 := make(chan error, 1)
	go func() {
		select {
		case pm.newPeerCh <- peer:
			errc <- pm.handle(peer)
		case <-pm.quitSync:
			errc <- p2p.DiscQuitting
		}
	}()
	go func() {
		select {
		case pm2.newPeerCh <- peer2:
			errc2 <- pm2.handle(peer2)
		case <-pm2.quitSync:
			errc2 <- p2p.DiscQuitting
		}
	}()
	return peer, errc, peer2, errc2
}

// handshake simulates a trivial handshake that expects the same state from the
// remote side as we are simulating locally.
func (p *testPeer) handshake(t *testing.T, head common.Hash, headNum uint64, genesis common.Hash) {
	var expList keyValueList
	expList = expList.add("protocolVersion", uint64(p.version))
	expList = expList.add("networkId", uint64(NetworkId))
	expList = expList.add("headHash", head)
	expList = expList.add("headNum", headNum)
	expList = expList.add("genesisHash", genesis)
	sendList := make(keyValueList, len(expList))
	copy(sendList, expList)
	expList = expList.add("serveHeaders", nil)
	expList = expList.add("serveChainSince", uint64(0))
	expList = expList.add("serveStateSince", uint64(0))
	expList = expList.add("txRelay", nil)
	expList = expList.add("flowControl/BL", testBufLimit)
	expList = expList.add("flowControl/MRR", uint64(1))
	expList = expList.add("flowControl/MRC", testRCL())

	if err := p2p.ExpectMsg(p.app, StatusMsg, expList); err != nil {
		t.Fatalf("status recv: %v", err)
	}
	if err := p2p.Send(p.app, StatusMsg, sendList); err != nil {
		t.Fatalf("status send: %v", err)
	}

	p.fcServerParams = &flowcontrol.ServerParams{
		BufLimit:    testBufLimit,
		MinRecharge: 1,
	}
}

// close terminates the local side of the peer, notifying the remote protocol
// manager of termination.
func (p *testPeer) close() {
	p.app.Close()
}

// TestEntity represents a network entity for testing with necessary auxiliary fields.
type TestEntity struct {
	db    ethdb.Database
	rPeer *peer
	tPeer *testPeer
	peers *peerSet
	pm    *ProtocolManager
	// Indexers
	chtIndexer       *core.ChainIndexer
	bloomIndexer     *core.ChainIndexer
	bloomTrieIndexer *core.ChainIndexer
}

// newServerEnv creates a server testing environment with a connected test peer for testing purpose.
func newServerEnv(t *testing.T, blocks int, protocol int, waitIndexers func(*core.ChainIndexer, *core.ChainIndexer, *core.ChainIndexer)) (*TestEntity, func()) {
	db := rawdb.NewMemoryDatabase()
	cIndexer, bIndexer, btIndexer := testIndexers(db, nil, light.TestServerIndexerConfig)

	pm := newTestProtocolManagerMust(t, false, blocks, testChainGen, nil, nil, db)
	peer, _ := newTestPeer(t, "peer", protocol, pm, true)

	cIndexer.Start(pm.blockchain.(*core.BlockChain))
	bIndexer.Start(pm.blockchain.(*core.BlockChain))

	// Wait until indexers generate enough index data.
	if waitIndexers != nil {
		waitIndexers(cIndexer, bIndexer, btIndexer)
	}

	return &TestEntity{
			db:               db,
			tPeer:            peer,
			pm:               pm,
			chtIndexer:       cIndexer,
			bloomIndexer:     bIndexer,
			bloomTrieIndexer: btIndexer,
		}, func() {
			peer.close()
			// Note bloom trie indexer will be closed by it parent recursively.
			cIndexer.Close()
			bIndexer.Close()
		}
}

// newClientServerEnv creates a client/server arch environment with a connected les server and light client pair
// for testing purpose.
func newClientServerEnv(t *testing.T, blocks int, protocol int, waitIndexers func(*core.ChainIndexer, *core.ChainIndexer, *core.ChainIndexer), newPeer bool) (*TestEntity, *TestEntity, func()) {
	db, ldb := rawdb.NewMemoryDatabase(), rawdb.NewMemoryDatabase()
	peers, lPeers := newPeerSet(), newPeerSet()

	dist := newRequestDistributor(lPeers, make(chan struct{}))
	rm := newRetrieveManager(lPeers, dist, nil)
	odr := NewLesOdr(ldb, light.TestClientIndexerConfig, rm)

	cIndexer, bIndexer, btIndexer := testIndexers(db, nil, light.TestServerIndexerConfig)
	lcIndexer, lbIndexer, lbtIndexer := testIndexers(ldb, odr, light.TestClientIndexerConfig)
	odr.SetIndexers(lcIndexer, lbtIndexer, lbIndexer)

	pm := newTestProtocolManagerMust(t, false, blocks, testChainGen, nil, peers, db)
	lpm := newTestProtocolManagerMust(t, true, 0, nil, odr, lPeers, ldb)

	startIndexers := func(clientMode bool, pm *ProtocolManager) {
		if clientMode {
			lcIndexer.Start(pm.blockchain.(*light.LightChain))
			lbIndexer.Start(pm.blockchain.(*light.LightChain))
		} else {
			cIndexer.Start(pm.blockchain.(*core.BlockChain))
			bIndexer.Start(pm.blockchain.(*core.BlockChain))
		}
	}

	startIndexers(false, pm)
	startIndexers(true, lpm)

	// Execute wait until function if it is specified.
	if waitIndexers != nil {
		waitIndexers(cIndexer, bIndexer, btIndexer)
	}

	var (
		peer, lPeer *peer
		err1, err2  <-chan error
	)
	if newPeer {
		peer, err1, lPeer, err2 = newTestPeerPair("peer", protocol, pm, lpm)
		select {
		case <-time.After(time.Millisecond * 100):
		case err := <-err1:
			t.Fatalf("peer 1 handshake error: %v", err)
		case err := <-err2:
			t.Fatalf("peer 2 handshake error: %v", err)
		}
	}

	return &TestEntity{
			db:               db,
			pm:               pm,
			rPeer:            peer,
			peers:            peers,
			chtIndexer:       cIndexer,
			bloomIndexer:     bIndexer,
			bloomTrieIndexer: btIndexer,
		}, &TestEntity{
			db:               ldb,
			pm:               lpm,
			rPeer:            lPeer,
			peers:            lPeers,
			chtIndexer:       lcIndexer,
			bloomIndexer:     lbIndexer,
			bloomTrieIndexer: lbtIndexer,
		}, func() {
			// Note bloom trie indexers will be closed by their parents recursively.
			cIndexer.Close()
			bIndexer.Close()
			lcIndexer.Close()
			lbIndexer.Close()
		}
}
