package pbft

import (
	"io/ioutil"
	"os"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/configs"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/consensus/pbft/network"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/consensus/pbft/protocols"
	ctypes "github.com/PhoenixGlobal/Phoenix-Chain-Core/consensus/pbft/types"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/consensus/pbft/utils"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto/bls"
)

const (
	testPeriod     = 10000
	testAmount     = 10
	testNodeNumber = 4
)

func path() string {
	name, err := ioutil.TempDir(os.TempDir(), "evidence")

	if err != nil {
		panic(err)
	}
	return name
}

func createPaths(number int) (paths []string) {
	for i := 0; i < number; i++ {
		p := path()
		paths = append(paths, p)
	}
	return
}

func removePaths(paths []string) {
	for _, path := range paths {
		os.RemoveAll(path)
	}
}

// Mock4NodePipe returns a list of TestPBFT for testing.
func Mock4NodePipe2(start bool) ([]*TestPBFT, []configs.PbftNode) {
	pk, sk, pbftnodes := GeneratePbftNode(4)
	nodes := make([]*TestPBFT, 0)
	for i := 0; i < 4; i++ {
		node := MockNode(pk[i], sk[i], pbftnodes, 10000, 10)

		nodes = append(nodes, node)
		//fmt.Println(i, node.engine.config.Option.NodeID.TerminalString())
		nodes[i].Start()
	}

	netHandler, nodeids := NewEngineManager(nodes)

	network.EnhanceEngineManager(nodeids, netHandler)
	if start {
		for i := 0; i < 4; i++ {
			netHandler[i].Testing()
		}
	}
	return nodes, pbftnodes
}

type testView struct {
	allPbft      []*Pbft
	allNode      []*TestPBFT
	nodeParams   []configs.PbftNode
	genesisBlock *types.Block
	firstPbft    *Pbft
}

func newTestView(start bool, nodeNumber int) *testView {
	nodes, nodeParams := Mock4NodePipe2(start)
	pbfts := make([]*Pbft, 0)
	for _, node := range nodes {
		pbfts = append(pbfts, node.engine)
	}
	return &testView{
		allPbft:      pbfts,
		allNode:      nodes,
		nodeParams:   nodeParams,
		genesisBlock: nodes[0].chain.Genesis(),
		firstPbft:    pbfts[0],
	}
}
func (tv *testView) firstProposer() *Pbft {
	for _, c := range tv.allPbft {
		index, err := c.validatorPool.GetIndexByNodeID(c.state.Epoch(), c.NodeID())
		if err != nil {
			panic("find proposer node failed")
		}
		if index == 0 {
			return c
		}
	}
	panic("find proposer node failed")
}
func (tv *testView) firstProposerIndex() uint32 {
	return 0
}
func (tv *testView) firstProposerBlsKey() *bls.SecretKey {
	return tv.firstProposer().config.Option.BlsPriKey
}
func (tv *testView) secondProposer() *Pbft {
	for _, c := range tv.allPbft {
		index, err := c.validatorPool.GetIndexByNodeID(c.state.Epoch(), c.NodeID())
		if err != nil {
			panic("find proposer node failed")
		}
		if index == 1 {
			return c
		}
	}
	panic("find proposer node failed")
}
func (tv *testView) secondProposerIndex() uint32 {
	return 1
}
func (tv *testView) secondProposerBlsKey() *bls.SecretKey {
	return tv.secondProposer().config.Option.BlsPriKey
}
func (tv *testView) thirdProposer() *Pbft {
	for _, c := range tv.allPbft {
		index, err := c.validatorPool.GetIndexByNodeID(c.state.Epoch(), c.NodeID())
		if err != nil {
			panic("find proposer node failed")
		}
		if index == 2 {
			return c
		}
	}
	panic("find proposer node failed")
}
func (tv *testView) thirdProposerIndex() uint32 {
	return 2
}

func (tv *testView) currentProposerInfo(pbft *Pbft) (uint32, uint64) {
	blockNumber := pbft.state.HighestQCBlock().NumberU64()
	numValidators := pbft.validatorPool.Len(blockNumber)
	currentProposer := uint32(pbft.state.ViewNumber()) % uint32(numValidators)
	return currentProposer, blockNumber
}
func (tv *testView) currentProposer(pbft *Pbft) *Pbft {
	currentProposer, _ := tv.currentProposerInfo(pbft)
	for _, c := range tv.allPbft {
		index, err := c.validatorPool.GetIndexByNodeID(c.state.Epoch(), c.NodeID())
		if err != nil {
			panic("find proposer node failed")
		}
		if index == uint32(currentProposer) {
			return c
		}
	}
	panic("find proposer node failed")
}

func (tv *testView) Epoch() uint64 {
	return tv.firstPbft.state.Epoch()
}

func (tv *testView) BlockNumber() uint64 {
	return tv.firstPbft.state.BlockNumber()
}

func (tv *testView) setBlockQC(number int, node *TestPBFT) {
	proposerNode := tv.currentProposer(tv.firstPbft)
	block := proposerNode.state.HighestQCBlock()
	if block.NumberU64() == 0 {
		b := NewBlockWithSign(block.Hash(), 1, node)
		newBlockQC := mockBlockQC(tv.allNode, b, proposerNode.state.NextViewBlockIndex(), nil)
		for _, pbft := range tv.allPbft {
			insertBlock(pbft, b, newBlockQC.BlockQC)
		}
		number--
		block = proposerNode.state.HighestQCBlock()
	}
	_, blockQC := proposerNode.blockTree.FindBlockAndQC(block.Hash(), block.NumberU64())
	var qc *ctypes.QuorumCert
	if blockQC != nil {
		qc = &ctypes.QuorumCert{
			Epoch:        blockQC.Epoch,
			ViewNumber:   blockQC.ViewNumber,
			BlockHash:    blockQC.BlockHash,
			BlockNumber:  blockQC.BlockNumber,
			BlockIndex:   blockQC.BlockIndex,
			Signature:    blockQC.Signature,
			ValidatorSet: blockQC.ValidatorSet,
		}
	}
	blockHash := block.Hash()
	blockNumber := block.NumberU64()
	for i := uint64(1); i <= uint64(number); i++ {
		b := NewBlockWithSign(blockHash, blockNumber+1, node)
		newBlockQC := mockBlockQC(tv.allNode, b, proposerNode.state.NextViewBlockIndex(), qc)
		qc = &ctypes.QuorumCert{
			Epoch:        newBlockQC.BlockQC.Epoch,
			ViewNumber:   newBlockQC.BlockQC.ViewNumber,
			BlockHash:    newBlockQC.BlockQC.BlockHash,
			BlockNumber:  newBlockQC.BlockQC.BlockNumber,
			BlockIndex:   newBlockQC.BlockQC.BlockIndex,
			Signature:    newBlockQC.BlockQC.Signature,
			ValidatorSet: newBlockQC.BlockQC.ValidatorSet,
		}
		for _, pbft := range tv.allPbft {
			insertBlock(pbft, b, qc)
		}
		blockHash = b.Hash()
		blockNumber = b.NumberU64()
	}
}

func (tv *testView) ResetView(start bool, nodeNumber int) {
	tv = newTestView(start, nodeNumber)
}

func insertBlock(pbft *Pbft, block *types.Block, qc *ctypes.QuorumCert) {
	pbft.state.AddQCBlock(block, qc)
	pbft.insertQCBlock(block, qc)
}
func mockNodeOfNumber(start bool, nodeNumber int) ([]*TestPBFT, []configs.PbftNode) {
	pk, sk, pbftnodes := GeneratePbftNode(nodeNumber)
	nodes := make([]*TestPBFT, 0)
	for i := 0; i < nodeNumber; i++ {
		node := MockValidator(pk[i], sk[i], pbftnodes, testPeriod, testAmount)
		nodes = append(nodes, node)
		//fmt.Println(i, node.engine.NodeID().TerminalString())
		if err := nodes[i].Start(); err != nil {
			panic("pbft start fail")
		}
	}

	netHandler, nodeids := NewEngineManager(nodes)

	network.EnhanceEngineManager(nodeids, netHandler)
	if start {
		for i := 0; i < nodeNumber; i++ {
			netHandler[i].Testing()
		}
	}
	return nodes, pbftnodes
}

func mockNotConsensusNode(start bool, pbftnodes []configs.PbftNode, number int) []*TestPBFT {
	pk, sk, _ := GeneratePbftNode(number)
	nodes := make([]*TestPBFT, 0)
	for i := 0; i < number; i++ {
		node := MockNode(pk[i], sk[i], pbftnodes, testPeriod, testAmount)

		nodes = append(nodes, node)
		//fmt.Println(i, node.engine.NodeID().TerminalString())
		if err := node.Start(); err != nil {
			panic("pbft start fail")
		}
	}

	// netHandler, nodeids := NewEngineManager(nodes)
	//
	// network.EnhanceEngineManager(nodeids, netHandler)
	// if start {
	// 	for i := 0; i < number; i++ {
	// 		netHandler[i].Testing()
	// 	}
	// }
	return nodes
}

func mockSign(msg ctypes.ConsensusMsg, priv *bls.SecretKey) error {
	buf, err := msg.CannibalizeBytes()
	if err != nil {
		return err
	}
	sign := priv.Sign(string(buf))
	msg.SetSign(sign.Serialize())
	return nil
}

func mockPrepareVote(priv *bls.SecretKey, epoch uint64, viewNumber uint64, blockIndex uint32, validatorIndex uint32,
	blockHash common.Hash, blockNumber uint64, qc *ctypes.QuorumCert) *protocols.PrepareVote {
	prepareVote := &protocols.PrepareVote{
		Epoch:          epoch,
		ViewNumber:     viewNumber,
		BlockHash:      blockHash,
		BlockNumber:    blockNumber,
		BlockIndex:     blockIndex,
		ValidatorIndex: validatorIndex,
		ParentQC:       qc,
	}
	if priv == nil {
		return prepareVote
	}
	if err := mockSign(prepareVote, priv); err != nil {
		panic("sign err")
	}
	return prepareVote
}

func mockPrepareBlock(priv *bls.SecretKey, epoch uint64, viewNumber uint64, blockIndex uint32, proposalIndex uint32,
	block *types.Block, qc *ctypes.QuorumCert, view *ctypes.ViewChangeQC) *protocols.PrepareBlock {
	prepareBlock := &protocols.PrepareBlock{
		Epoch:         epoch,
		ViewNumber:    viewNumber,
		Block:         block,
		BlockIndex:    blockIndex,
		ProposalIndex: proposalIndex,
		PrepareQC:     qc,
		ViewChangeQC:  view,
	}
	if priv == nil {
		return prepareBlock
	}
	if err := mockSign(prepareBlock, priv); err != nil {
		panic("sign err")
	}
	return prepareBlock
}

func mockViewChange(priv *bls.SecretKey, epoch uint64, viewNumber uint64, hash common.Hash, number uint64,
	index uint32, qc *ctypes.QuorumCert) *protocols.ViewChange {
	viewChange := &protocols.ViewChange{
		Epoch:          epoch,
		ViewNumber:     viewNumber,
		BlockHash:      hash,
		BlockNumber:    number,
		ValidatorIndex: index,
		PrepareQC:      qc,
	}
	if priv == nil {
		return viewChange
	}
	if err := mockSign(viewChange, priv); err != nil {
		panic("sign error")
	}
	return viewChange
}

func mockBlockQC(nodes []*TestPBFT, block *types.Block, blockIndex uint32, qc *ctypes.QuorumCert) *protocols.BlockQuorumCert {
	votes := make(map[uint32]*protocols.PrepareVote)
	for _, node := range nodes {
		index, err := node.engine.validatorPool.GetIndexByNodeID(node.engine.state.Epoch(), node.engine.NodeID())
		if err != nil {
			panic("find proposer node failed")
		}
		vote := mockPrepareVote(node.engine.config.Option.BlsPriKey,
			node.engine.state.Epoch(), node.engine.state.ViewNumber(), blockIndex, index, block.Hash(), block.NumberU64(), qc)
		votes[index] = vote
	}
	prepareQC := mockPrepareQC(uint32(len(nodes)), votes)
	return &protocols.BlockQuorumCert{BlockQC: prepareQC}
}
func mockBlockQCWithNotConsensus(nodes []*TestPBFT, block *types.Block, blockIndex uint32, qc *ctypes.QuorumCert) *protocols.BlockQuorumCert {
	votes := make(map[uint32]*protocols.PrepareVote)
	for i, node := range nodes {
		vote := mockPrepareVote(node.engine.config.Option.BlsPriKey,
			node.engine.state.Epoch(), node.engine.state.ViewNumber(), blockIndex, uint32(i), block.Hash(), block.NumberU64(), qc)
		votes[uint32(i)] = vote
	}
	prepareQC := mockPrepareQC(uint32(len(nodes)), votes)
	return &protocols.BlockQuorumCert{BlockQC: prepareQC}
}
func mockBlockQCWithViewNumber(nodes []*TestPBFT, block *types.Block, blockIndex uint32, qc *ctypes.QuorumCert, viewNumber uint64) *protocols.BlockQuorumCert {
	votes := make(map[uint32]*protocols.PrepareVote)
	for _, node := range nodes {
		index, err := node.engine.validatorPool.GetIndexByNodeID(node.engine.state.Epoch(), node.engine.NodeID())
		if err != nil {
			panic("find proposer node failed")
		}
		vote := mockPrepareVote(node.engine.config.Option.BlsPriKey,
			node.engine.state.Epoch(), viewNumber, blockIndex, index, block.Hash(), block.NumberU64(), qc)
		votes[index] = vote
	}
	prepareQC := mockPrepareQC(uint32(len(nodes)), votes)
	return &protocols.BlockQuorumCert{BlockQC: prepareQC}
}
func mockBlockQCWithEpoch(nodes []*TestPBFT, block *types.Block, blockIndex uint32, qc *ctypes.QuorumCert, epoch uint64) *protocols.BlockQuorumCert {
	votes := make(map[uint32]*protocols.PrepareVote)
	for _, node := range nodes {
		index, err := node.engine.validatorPool.GetIndexByNodeID(node.engine.state.Epoch(), node.engine.NodeID())
		if err != nil {
			panic("find proposer node failed")
		}
		vote := mockPrepareVote(node.engine.config.Option.BlsPriKey,
			epoch, node.engine.state.ViewNumber(), blockIndex, index, block.Hash(), block.NumberU64(), qc)
		votes[index] = vote
	}
	prepareQC := mockPrepareQC(uint32(len(nodes)), votes)
	return &protocols.BlockQuorumCert{BlockQC: prepareQC}
}
func mockErrBlockQC(nodes []*TestPBFT, block *types.Block, blockIndex uint32, qc *ctypes.QuorumCert) *protocols.BlockQuorumCert {
	votes := make(map[uint32]*protocols.PrepareVote)
	for index, node := range nodes {
		vote := mockPrepareVote(node.engine.config.Option.BlsPriKey,
			node.engine.state.Epoch(), node.engine.state.ViewNumber(), blockIndex, uint32(index), block.Hash(), block.NumberU64(), qc)
		votes[uint32(index)] = vote
	}
	prepareQC := mockPrepareQC(uint32(len(nodes)), votes)
	return &protocols.BlockQuorumCert{BlockQC: prepareQC}
}

func mockPrepareQC(total uint32, votes map[uint32]*protocols.PrepareVote) *ctypes.QuorumCert {
	if len(votes) == 0 {
		return nil
	}
	var vote *protocols.PrepareVote
	for _, v := range votes {
		vote = v
	}
	vSet := utils.NewBitArray(uint32(total))
	vSet.SetIndex(vote.NodeIndex(), true)
	var aggSig bls.Sign
	if err := aggSig.Deserialize(vote.Sign()); err != nil {
		return nil
	}
	qc := &ctypes.QuorumCert{
		Epoch:        vote.Epoch,
		ViewNumber:   vote.ViewNumber,
		BlockHash:    vote.BlockHash,
		BlockNumber:  vote.BlockNumber,
		BlockIndex:   vote.BlockIndex,
		ValidatorSet: utils.NewBitArray(vSet.Size()),
	}
	for _, p := range votes {
		if p.NodeIndex() != vote.NodeIndex() {
			var sig bls.Sign
			err := sig.Deserialize(p.Sign())
			if err != nil {
				return nil
			}

			aggSig.Add(&sig)
			vSet.SetIndex(p.NodeIndex(), true)
		}
	}
	qc.Signature.SetBytes(aggSig.Serialize())
	qc.ValidatorSet.Update(vSet)
	return qc
}
func mockViewQC(block *types.Block, nodes []*TestPBFT, qc *ctypes.QuorumCert) *ctypes.ViewChangeQC {
	votes := make(map[uint32]*protocols.ViewChange)
	for _, node := range nodes {
		index, err := node.engine.validatorPool.GetIndexByNodeID(node.engine.state.Epoch(), node.engine.NodeID())
		if err != nil {
			panic(err.Error())
		}
		vote := mockViewChange(node.engine.config.Option.BlsPriKey, node.engine.state.Epoch(), node.engine.state.ViewNumber(),
			block.Hash(), block.NumberU64(), index, qc)
		votes[index] = vote
	}
	return genViewChangeQC(uint32(len(nodes)), votes)
}

func genViewChangeQC(total uint32, viewChanges map[uint32]*protocols.ViewChange) *ctypes.ViewChangeQC {
	type ViewChangeQC struct {
		cert   *ctypes.ViewChangeQuorumCert
		aggSig *bls.Sign
		ba     *utils.BitArray
	}
	// total := uint32(pbft.validatorPool.Len(pbft.state.HighestQCBlock().NumberU64()))
	qcs := make(map[common.Hash]*ViewChangeQC)
	for _, v := range viewChanges {
		var aggSig bls.Sign
		if err := aggSig.Deserialize(v.Sign()); err != nil {
			return nil
		}

		if vc, ok := qcs[v.BlockHash]; !ok {
			blockEpoch, blockView := uint64(0), uint64(0)
			if v.PrepareQC != nil {
				blockEpoch, blockView = v.PrepareQC.Epoch, v.PrepareQC.ViewNumber
			}
			qc := &ViewChangeQC{
				cert: &ctypes.ViewChangeQuorumCert{
					Epoch:           v.Epoch,
					ViewNumber:      v.ViewNumber,
					BlockHash:       v.BlockHash,
					BlockNumber:     v.BlockNumber,
					BlockEpoch:      blockEpoch,
					BlockViewNumber: blockView,
					ValidatorSet:    utils.NewBitArray(total),
				},
				aggSig: &aggSig,
				ba:     utils.NewBitArray(total),
			}
			qc.ba.SetIndex(v.NodeIndex(), true)
			qcs[v.BlockHash] = qc
		} else {
			vc.aggSig.Add(&aggSig)
			vc.ba.SetIndex(v.NodeIndex(), true)
		}
	}
	qc := &ctypes.ViewChangeQC{QCs: make([]*ctypes.ViewChangeQuorumCert, 0)}
	for _, q := range qcs {
		q.cert.Signature.SetBytes(q.aggSig.Serialize())
		q.cert.ValidatorSet.Update(q.ba)
		qc.QCs = append(qc.QCs, q.cert)
	}
	return qc
}
