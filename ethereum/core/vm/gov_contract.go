package vm

import (
	"math/big"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/xutil"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/configs"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/p2p/discover"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common/vm"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/log"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/gov"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/plugin"
)

const (
	SubmitText            = uint16(2000)
	SubmitVersion         = uint16(2001)
	SubmitParam           = uint16(2002)
	Vote                  = uint16(2003)
	Declare               = uint16(2004)
	SubmitCancel          = uint16(2005)
	GetProposal           = uint16(2100)
	GetResult             = uint16(2101)
	ListProposal          = uint16(2102)
	GetActiveVersion      = uint16(2103)
	GetGovernParamValue   = uint16(2104)
	GetAccuVerifiersCount = uint16(2105)
	ListGovernParam       = uint16(2106)
)

var (
	Delimiter = []byte("")
)

type GovContract struct {
	Plugin   *plugin.GovPlugin
	Contract *Contract
	Evm      *EVM
}

func (gc *GovContract) RequiredGas(input []byte) uint64 {
	if checkInputEmpty(input) {
		return 0
	}
	return configs.GovGas
}

func (gc *GovContract) Run(input []byte) ([]byte, error) {
	if checkInputEmpty(input) {
		return nil, nil
	}
	return execPhoenixchainContract(input, gc.FnSigns())
}

func (gc *GovContract) FnSigns() map[uint16]interface{} {
	return map[uint16]interface{}{
		// Set
		SubmitText:    gc.submitText,
		SubmitVersion: gc.submitVersion,
		Vote:          gc.vote,
		Declare:       gc.declareVersion,
		SubmitCancel:  gc.submitCancel,
		SubmitParam:   gc.submitParam,

		// Get
		GetProposal:           gc.getProposal,
		GetResult:             gc.getTallyResult,
		ListProposal:          gc.listProposal,
		GetActiveVersion:      gc.getActiveVersion,
		GetGovernParamValue:   gc.getGovernParamValue,
		GetAccuVerifiersCount: gc.getAccuVerifiersCount,
		ListGovernParam:       gc.listGovernParam,
	}
}

func (gc *GovContract) CheckGasPrice(gasPrice *big.Int, fcode uint16) error {
	switch fcode {
	case SubmitText:
		if gasPrice.Cmp(configs.SubmitTextProposalGasPrice) < 0 {
			return common.InvalidParameter.Wrap(ErrUnderPrice.Error())
		}
	case SubmitVersion:
		if gasPrice.Cmp(configs.SubmitVersionProposalGasPrice) < 0 {
			return common.InvalidParameter.Wrap(ErrUnderPrice.Error())
		}
	case SubmitCancel:
		if gasPrice.Cmp(configs.SubmitCancelProposalGasPrice) < 0 {
			return common.InvalidParameter.Wrap(ErrUnderPrice.Error())
		}
	case SubmitParam:
		if gasPrice.Cmp(configs.SubmitParamProposalGasPrice) < 0 {
			return common.InvalidParameter.Wrap(ErrUnderPrice.Error())
		}
	}

	return nil
}

func (gc *GovContract) submitText(verifier discover.NodeID, pipID string) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()

	log.Debug("call submitText of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"PIPID", pipID,
		"verifierID", verifier.TerminalString())

	if !gc.Contract.UseGas(configs.SubmitTextProposalGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}
	if gc.Evm.GasPrice.Cmp(configs.SubmitTextProposalGasPrice) < 0 {
		return nil, ErrUnderPrice
	}

	p := &gov.TextProposal{
		PIPID:        pipID,
		ProposalType: gov.Text,
		SubmitBlock:  blockNumber,
		ProposalID:   txHash,
		Proposer:     verifier,
	}
	err := gov.Submit(from, p, blockHash, blockNumber, plugin.StakingInstance(), gc.Evm.StateDB, gc.Evm.chainConfig.ChainID)
	return gc.nonCallHandler("submitText", SubmitText, err)
}

func (gc *GovContract) submitVersion(verifier discover.NodeID, pipID string, newVersion uint32, endVotingRounds uint64) ([]byte, error) {
	from := gc.Contract.CallerAddress

	blockNumber := gc.Evm.BlockNumber.Uint64()
	blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()

	log.Debug("call submitVersion of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"PIPID", pipID,
		"verifierID", verifier.TerminalString(),
		"newVersion", newVersion,
		"newVersionString", xutil.ProgramVersion2Str(newVersion),
		"endVotingRounds", endVotingRounds)

	if !gc.Contract.UseGas(configs.SubmitVersionProposalGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	if gc.Evm.GasPrice.Cmp(configs.SubmitVersionProposalGasPrice) < 0 {
		return nil, ErrUnderPrice
	}

	p := &gov.VersionProposal{
		PIPID:           pipID,
		ProposalType:    gov.Version,
		EndVotingRounds: endVotingRounds,
		SubmitBlock:     blockNumber,
		ProposalID:      txHash,
		Proposer:        verifier,
		NewVersion:      newVersion,
	}
	err := gov.Submit(from, p, blockHash, blockNumber, plugin.StakingInstance(), gc.Evm.StateDB, gc.Evm.chainConfig.ChainID)
	return gc.nonCallHandler("submitVersion", SubmitVersion, err)
}

func (gc *GovContract) submitCancel(verifier discover.NodeID, pipID string, endVotingRounds uint64, tobeCanceledProposalID common.Hash) ([]byte, error) {
	from := gc.Contract.CallerAddress

	blockNumber := gc.Evm.BlockNumber.Uint64()
	blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()

	log.Debug("call submitCancel of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"PIPID", pipID,
		"verifierID", verifier.TerminalString(),
		"endVotingRounds", endVotingRounds,
		"tobeCanceled", tobeCanceledProposalID)

	if !gc.Contract.UseGas(configs.SubmitCancelProposalGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	if gc.Evm.GasPrice.Cmp(configs.SubmitCancelProposalGasPrice) < 0 {
		return nil, ErrUnderPrice
	}

	p := &gov.CancelProposal{
		PIPID:           pipID,
		EndVotingRounds: endVotingRounds,
		ProposalType:    gov.Cancel,
		SubmitBlock:     blockNumber,
		ProposalID:      txHash,
		Proposer:        verifier,
		TobeCanceled:    tobeCanceledProposalID,
	}
	err := gov.Submit(from, p, blockHash, blockNumber, plugin.StakingInstance(), gc.Evm.StateDB, gc.Evm.chainConfig.ChainID)
	return gc.nonCallHandler("submitCancel", SubmitCancel, err)
}

func (gc *GovContract) submitParam(verifier discover.NodeID, pipID string, module, name, newValue string) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()

	log.Debug("call submitParam of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"PIPID", pipID,
		"verifierID", verifier.TerminalString(),
		"module", module,
		"name", name,
		"newValue", newValue)

	if !gc.Contract.UseGas(configs.SubmitParamProposalGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	if gc.Evm.GasPrice.Cmp(configs.SubmitParamProposalGasPrice) < 0 {
		return nil, ErrUnderPrice
	}

	p := &gov.ParamProposal{
		PIPID:        pipID,
		ProposalType: gov.Param,
		SubmitBlock:  blockNumber,
		ProposalID:   txHash,
		Proposer:     verifier,
		Module:       module,
		Name:         name,
		NewValue:     newValue,
	}
	err := gov.Submit(from, p, blockHash, blockNumber, plugin.StakingInstance(), gc.Evm.StateDB, gc.Evm.chainConfig.ChainID)
	return gc.nonCallHandler("submitParam", SubmitParam, err)
}

func (gc *GovContract) vote(verifier discover.NodeID, proposalID common.Hash, op uint8, programVersion uint32, programVersionSign common.VersionSign) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()

	log.Debug("call vote of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"verifierID", verifier.TerminalString(),
		"option", op,
		"programVersion", programVersion,
		"programVersionString", xutil.ProgramVersion2Str(programVersion),
		"programVersionSign", programVersionSign)

	if !gc.Contract.UseGas(configs.VoteGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	option := gov.ParseVoteOption(op)

	v := gov.VoteInfo{}
	v.ProposalID = proposalID
	v.VoteNodeID = verifier
	v.VoteOption = option

	err := gov.Vote(from, v, blockHash, blockNumber, programVersion, programVersionSign, plugin.StakingInstance(), gc.Evm.StateDB)

	return gc.nonCallHandler("vote", Vote, err)
}

func (gc *GovContract) declareVersion(activeNode discover.NodeID, programVersion uint32, programVersionSign common.VersionSign) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()
	log.Debug("call declareVersion of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"activeNode", activeNode.TerminalString(),
		"programVersion", programVersion,
		"programVersionString", xutil.ProgramVersion2Str(programVersion))

	if !gc.Contract.UseGas(configs.DeclareVersionGas) {
		return nil, ErrOutOfGas
	}

	if txHash == common.ZeroHash {
		return nil, nil
	}

	err := gov.DeclareVersion(from, activeNode, programVersion, programVersionSign, blockHash, blockNumber, plugin.StakingInstance(), gc.Evm.StateDB)

	return gc.nonCallHandler("declareVersion", Declare, err)
}

func (gc *GovContract) getProposal(proposalID common.Hash) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	//blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()
	log.Debug("call getProposal of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"proposalID", proposalID)

	proposal, err := gov.GetExistProposal(proposalID, gc.Evm.StateDB)

	return gc.callHandler("getProposal", proposal, err)
}

func (gc *GovContract) getTallyResult(proposalID common.Hash) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	//blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()
	log.Debug("call getTallyResult of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"proposalID", proposalID)

	tallyResult, err := gov.GetTallyResult(proposalID, gc.Evm.StateDB)

	if tallyResult == nil {
		err = gov.TallyResultNotFound
	}
	return gc.callHandler("getTallyResult", tallyResult, err)
}

func (gc *GovContract) listProposal() ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	//blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()
	log.Debug("call listProposal of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber)

	proposalList, err := gov.ListProposal(gc.Evm.BlockHash, gc.Evm.StateDB)

	return gc.callHandler("listProposal", proposalList, err)
}

func (gc *GovContract) getActiveVersion() ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	//blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()
	log.Debug("call getActiveVersion of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber)

	activeVersion := gov.GetCurrentActiveVersion(gc.Evm.StateDB)

	return gc.callHandler("getActiveVersion", activeVersion, nil)
}

func (gc *GovContract) getAccuVerifiersCount(proposalID, blockHash common.Hash) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	//blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()
	log.Debug("call getAccuVerifiesCount of GovContract",
		"from", from,
		"txHash", txHash,
		"blockNumber", blockNumber,
		"blockHash", blockHash,
		"proposalID", proposalID)

	proposal, err := gov.GetProposal(proposalID, gc.Evm.StateDB)
	if err != nil {
		return gc.callHandler("getAccuVerifiesCount", nil, common.InternalError.Wrap(err.Error()))
	} else if proposal == nil {
		return gc.callHandler("getAccuVerifiesCount", nil, gov.ProposalNotFound)
	}

	list, err := gov.ListAccuVerifier(blockHash, proposalID)
	if err != nil {
		return gc.callHandler("getAccuVerifiesCount", nil, common.InternalError.Wrap(err.Error()))
	}

	yeas, nays, abstentions, err := gov.TallyVoteValue(proposalID, blockHash)
	if err != nil {
		return gc.callHandler("getAccuVerifiesCount", nil, common.InternalError.Wrap(err.Error()))
	}

	returnValue := []uint64{uint64(len(list)), yeas, nays, abstentions}
	return gc.callHandler("getAccuVerifiesCount", returnValue, nil)
}

// getGovernParamValue returns the govern parameter's value in current block.
func (gc *GovContract) getGovernParamValue(module, name string) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()
	log.Debug("call getGovernParamValue of GovContract",
		"from", from,
		"txHash", txHash,
		"module", module,
		"name", name,
		"blockNumber", blockNumber)

	value, err := gov.GetGovernParamValue(module, name, blockNumber, blockHash)

	return gc.callHandler("getGovernParamValue", value, err)
}

// listGovernParam returns the module's govern parameters; if module is empty, return all govern parameters
func (gc *GovContract) listGovernParam(module string) ([]byte, error) {
	from := gc.Contract.CallerAddress
	blockNumber := gc.Evm.BlockNumber.Uint64()
	blockHash := gc.Evm.BlockHash
	txHash := gc.Evm.StateDB.TxHash()
	log.Debug("call listGovernParam of GovContract",
		"from", from,
		"txHash", txHash,
		"module", module,
		"blockNumber", blockNumber)

	paramList, err := gov.ListGovernParam(module, blockHash)

	return gc.callHandler("listGovernParam", paramList, err)
}

func (gc *GovContract) nonCallHandler(funcName string, fcode uint16, err error) ([]byte, error) {
	if err != nil {
		if bizErr, ok := err.(*common.BizError); ok {
			return txResultHandler(vm.GovContractAddr, gc.Evm, funcName+" of GovContract",
				bizErr.Error(), int(fcode), bizErr)
		} else {
			log.Error("Execute GovContract failed.(System error)", "method", funcName, "blockNumber", gc.Evm.BlockNumber.Uint64(),
				"txHash", gc.Evm.StateDB.TxHash(), "err", err)
			return nil, err
		}
	} else {
		return txResultHandler(vm.GovContractAddr, gc.Evm, "", "", int(fcode), common.NoErr)
	}
}

func (gc *GovContract) callHandler(funcName string, resultValue interface{}, err error) ([]byte, error) {
	if err == nil {
		return callResultHandler(gc.Evm, funcName+" of GovContract", resultValue, nil), nil
	}
	switch typed := err.(type) {
	case *common.BizError:
		return callResultHandler(gc.Evm, funcName+" of GovContract", resultValue, typed), nil
	default:
		return callResultHandler(gc.Evm, funcName+" of GovContract", resultValue, common.InternalError.Wrap(err.Error())), nil
	}
}
