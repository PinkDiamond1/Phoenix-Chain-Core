package vm

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common/hexutil"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rlp"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/plugin"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/restricting"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/pos/xcom"
)

// build input data for testing the create restricting plan successfully
func buildRestrictingPlanData() ([]byte, error) {
	var plan restricting.RestrictingPlan
	var plans = make([]restricting.RestrictingPlan, 5)

	var epoch uint64
	for index := 0; index < len(plans); index++ {
		epoch = uint64(index + 1)
		plan.Epoch = uint64(epoch)
		plan.Amount = xcom.FloorMinimumRelease
		plans[index] = plan
	}

	var params [][]byte
	param0, _ := rlp.EncodeToBytes(common.Uint16ToBytes(4000)) // function_type
	param1, _ := rlp.EncodeToBytes(addrArr[0].Bytes())         // restricting account
	param2, _ := rlp.EncodeToBytes(plans)                      // restricting plan

	params = append(params, param0)
	params = append(params, param1)
	params = append(params, param2)

	return rlp.EncodeToBytes(params)
}

// build input data for testing the create restricting plan failed
func buildErrorRestrictingPlanData() ([]byte, error) {
	var plan restricting.RestrictingPlan
	var plans = make([]restricting.RestrictingPlan, 1)

	plan.Epoch = uint64(0)
	plan.Amount = xcom.FloorMinimumRelease
	plans[0] = plan

	var params [][]byte
	param0, _ := rlp.EncodeToBytes(common.Uint16ToBytes(4000)) // function_type
	param1, _ := rlp.EncodeToBytes(addrArr[0].Bytes())         // restricting account
	param2, _ := rlp.EncodeToBytes(plans)                      // restricting plan

	params = append(params, param0)
	params = append(params, param1)
	params = append(params, param2)

	return rlp.EncodeToBytes(params)
}

func TestRestrictingContract_createRestrictingPlan(t *testing.T) {
	contract := &RestrictingContract{
		Plugin:   plugin.RestrictingInstance(),
		Contract: newContract(common.Big0, sender),
		Evm:      newEvm(blockNumber, blockHash, nil),
	}

	{
		// case1: create plan success
		input, err := buildRestrictingPlanData()
		if err != nil {
			t.Fatal("fail to rlp encode restricting input, error:", err.Error())
		} else {
			t.Log("rlp encode restricting input: ", hexutil.Encode(input))
		}

		if result, err := contract.Run(input); err != nil {
			t.Fatal("create restricting input failed, error:", err.Error())
		} else {
			t.Log(string(result))
		}
	}

	{
		// case2: create plan failed
		input, err := buildErrorRestrictingPlanData()
		if err != nil {
			t.Fatal("fail to rlp encode restricting input, error:", err.Error())
		} else {
			t.Log("rlp encode restricting input: ", hexutil.Encode(input))
		}

		if result, err := contract.Run(input); err == nil {
			t.Error("the restricting plan must failed")
		} else {
			t.Log(string(result))
		}
	}

}

func TestRestrictingContract_getRestrictingInfo(t *testing.T) {
	// build db data for getting info
	account := addrArr[0]
	stateDb, _, _ := newChainState()
	balance, _ := new(big.Int).SetString("20000000000000000000000000", 10)
	buildDbRestrictingPlan(t, account, balance, 5, stateDb)

	contract := &RestrictingContract{
		Plugin:   plugin.RestrictingInstance(),
		Contract: newContract(common.Big0, sender),
		Evm:      newEvm(blockNumber, blockHash, stateDb),
	}

	var params [][]byte
	param0, _ := rlp.EncodeToBytes(common.Uint16ToBytes(4100))
	param1, _ := rlp.EncodeToBytes(account)
	params = append(params, param0)
	params = append(params, param1)
	input, err := rlp.EncodeToBytes(params)
	if err != nil {
		t.Log(err.Error())
		t.Errorf("fail to rlp encode restricting input")
	} else {
		t.Log("rlp encode restricting input: ", hexutil.Encode(input))
	}

	t.Log("restricting account is", addrArr[0].String())

	if result, err := contract.Run(input); err != nil {
		t.Errorf("getRestrictingInfo returns error! error is: %s", err.Error())
	} else {

		t.Log(string(result))

		var res xcom.Result
		if err = json.Unmarshal(result, &res); err != nil {
			t.Fatalf("failed to json unmarshal result of restricting info , error: %s", err.Error())

		} else {
			t.Logf("%v", res.Code)
			t.Logf("%v", res.Ret)
		}
		t.Log("test pass!")
	}
}
