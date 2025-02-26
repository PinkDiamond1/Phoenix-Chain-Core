package rules

import (
	core2 "github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/signer/core"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/signer/storage"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/internal/ethapi"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/accounts"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common/hexutil"
)

const JS = `
/**
This is an example implementation of a Javascript rule file.

When the signer receives a request over the external API, the corresponding method is evaluated.
Three things can happen:

1. The method returns "Approve". This means the operation is permitted.
2. The method returns "Reject". This means the operation is rejected.
3. Anything else; other return values [*], method not implemented or exception occurred during processing. This means
that the operation will continue to manual processing, via the regular UI method chosen by the user.

[*] Note: Future version of the ruleset may use more complex json-based returnvalues, making it possible to not
only respond Approve/Reject/Manual, but also modify responses. For example, choose to list only one, but not all
accounts in a list-request. The points above will continue to hold for non-json based responses ("Approve"/"Reject").

**/

function ApproveListing(request){
	console.log("In js approve listing");
	console.log(request.accounts[3].Address)
	console.log(request.meta.Remote)
	return "Approve"
}

function ApproveTx(request){
	console.log("test");
	console.log("from");
	return "Reject";
}

function test(thing){
	console.log(thing.String())
}

`

func mixAddr(a string) (*common.MixedcaseAddress, error) {
	return common.NewMixedcaseAddressFromString(a)
}

type alwaysDenyUI struct{}

func (alwaysDenyUI) OnSignerStartup(info core2.StartupInfo) {
}

func (alwaysDenyUI) ApproveTx(request *core2.SignTxRequest) (core2.SignTxResponse, error) {
	return core2.SignTxResponse{Transaction: request.Transaction, Approved: false, Password: ""}, nil
}

func (alwaysDenyUI) ApproveSignData(request *core2.SignDataRequest) (core2.SignDataResponse, error) {
	return core2.SignDataResponse{Approved: false, Password: ""}, nil
}

func (alwaysDenyUI) ApproveExport(request *core2.ExportRequest) (core2.ExportResponse, error) {
	return core2.ExportResponse{Approved: false}, nil
}

func (alwaysDenyUI) ApproveImport(request *core2.ImportRequest) (core2.ImportResponse, error) {
	return core2.ImportResponse{Approved: false, OldPassword: "", NewPassword: ""}, nil
}

func (alwaysDenyUI) ApproveListing(request *core2.ListRequest) (core2.ListResponse, error) {
	return core2.ListResponse{Accounts: nil}, nil
}

func (alwaysDenyUI) ApproveNewAccount(request *core2.NewAccountRequest) (core2.NewAccountResponse, error) {
	return core2.NewAccountResponse{Approved: false, Password: ""}, nil
}

func (alwaysDenyUI) ShowError(message string) {
	panic("implement me")
}

func (alwaysDenyUI) ShowInfo(message string) {
	panic("implement me")
}

func (alwaysDenyUI) OnApprovedTx(tx ethapi.SignTransactionResult) {
	panic("implement me")
}

func initRuleEngine(js string) (*rulesetUI, error) {
	r, err := NewRuleEvaluator(&alwaysDenyUI{}, storage.NewEphemeralStorage(), storage.NewEphemeralStorage())
	if err != nil {
		return nil, fmt.Errorf("failed to create js engine: %v", err)
	}
	if err = r.Init(js); err != nil {
		return nil, fmt.Errorf("failed to load bootstrap js: %v", err)
	}
	return r, nil
}

func TestListRequest(t *testing.T) {
	accs := make([]core2.Account, 5)

	for i := range accs {
		addr := fmt.Sprintf("000000000000000000000000000000000000000%x", i)
		acc := core2.Account{
			Address: common.BytesToAddress(common.Hex2Bytes(addr)),
			URL:     accounts.URL{Scheme: "test", Path: fmt.Sprintf("acc-%d", i)},
		}
		accs[i] = acc
	}

	js := `function ApproveListing(){ return "Approve" }`

	r, err := initRuleEngine(js)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}
	resp, err := r.ApproveListing(&core2.ListRequest{
		Accounts: accs,
		Meta:     core2.Metadata{Remote: "remoteip", Local: "localip", Scheme: "inproc"},
	})
	if len(resp.Accounts) != len(accs) {
		t.Errorf("Expected check to resolve to 'Approve'")
	}
}

func TestSignTxRequest(t *testing.T) {

	js := `
	function ApproveTx(r){
		console.log("transaction.from", r.transaction.from);
		console.log("transaction.to", r.transaction.to);
		console.log("transaction.value", r.transaction.value);
		console.log("transaction.nonce", r.transaction.nonce);
		if(r.transaction.from.toLowerCase()=="0x0000000000000000000000000000000000001337"){ return "Approve"}
		if(r.transaction.from.toLowerCase()=="0x000000000000000000000000000000000000dead"){ return "Reject"}
	}`

	r, err := initRuleEngine(js)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}
	to, err := mixAddr("000000000000000000000000000000000000dead")
	if err != nil {
		t.Error(err)
		return
	}
	from, err := mixAddr("0000000000000000000000000000000000001337")

	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("to %v", to.Address().String())
	resp, err := r.ApproveTx(&core2.SignTxRequest{
		Transaction: core2.SendTxArgs{
			From: *from,
			To:   to},
		Callinfo: nil,
		Meta:     core2.Metadata{Remote: "remoteip", Local: "localip", Scheme: "inproc"},
	})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if !resp.Approved {
		t.Errorf("Expected check to resolve to 'Approve'")
	}
}

type dummyUI struct {
	calls []string
}

func (d *dummyUI) ApproveTx(request *core2.SignTxRequest) (core2.SignTxResponse, error) {
	d.calls = append(d.calls, "ApproveTx")
	return core2.SignTxResponse{}, core2.ErrRequestDenied
}

func (d *dummyUI) ApproveSignData(request *core2.SignDataRequest) (core2.SignDataResponse, error) {
	d.calls = append(d.calls, "ApproveSignData")
	return core2.SignDataResponse{}, core2.ErrRequestDenied
}

func (d *dummyUI) ApproveExport(request *core2.ExportRequest) (core2.ExportResponse, error) {
	d.calls = append(d.calls, "ApproveExport")
	return core2.ExportResponse{}, core2.ErrRequestDenied
}

func (d *dummyUI) ApproveImport(request *core2.ImportRequest) (core2.ImportResponse, error) {
	d.calls = append(d.calls, "ApproveImport")
	return core2.ImportResponse{}, core2.ErrRequestDenied
}

func (d *dummyUI) ApproveListing(request *core2.ListRequest) (core2.ListResponse, error) {
	d.calls = append(d.calls, "ApproveListing")
	return core2.ListResponse{}, core2.ErrRequestDenied
}

func (d *dummyUI) ApproveNewAccount(request *core2.NewAccountRequest) (core2.NewAccountResponse, error) {
	d.calls = append(d.calls, "ApproveNewAccount")
	return core2.NewAccountResponse{}, core2.ErrRequestDenied
}

func (d *dummyUI) ShowError(message string) {
	d.calls = append(d.calls, "ShowError")
}

func (d *dummyUI) ShowInfo(message string) {
	d.calls = append(d.calls, "ShowInfo")
}

func (d *dummyUI) OnApprovedTx(tx ethapi.SignTransactionResult) {
	d.calls = append(d.calls, "OnApprovedTx")
}
func (d *dummyUI) OnSignerStartup(info core2.StartupInfo) {
}

//TestForwarding tests that the rule-engine correctly dispatches requests to the next caller
func TestForwarding(t *testing.T) {

	js := ""
	ui := &dummyUI{make([]string, 0)}
	jsBackend := storage.NewEphemeralStorage()
	credBackend := storage.NewEphemeralStorage()
	r, err := NewRuleEvaluator(ui, jsBackend, credBackend)
	if err != nil {
		t.Fatalf("Failed to create js engine: %v", err)
	}
	if err = r.Init(js); err != nil {
		t.Fatalf("Failed to load bootstrap js: %v", err)
	}
	r.ApproveSignData(nil)
	r.ApproveTx(nil)
	r.ApproveImport(nil)
	r.ApproveNewAccount(nil)
	r.ApproveListing(nil)
	r.ApproveExport(nil)
	r.ShowError("test")
	r.ShowInfo("test")

	//This one is not forwarded
	r.OnApprovedTx(ethapi.SignTransactionResult{})

	expCalls := 8
	if len(ui.calls) != expCalls {

		t.Errorf("Expected %d forwarded calls, got %d: %s", expCalls, len(ui.calls), strings.Join(ui.calls, ","))

	}

}

func TestMissingFunc(t *testing.T) {
	r, err := initRuleEngine(JS)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}

	_, err = r.execute("MissingMethod", "test")

	if err == nil {
		t.Error("Expected error")
	}

	approved, err := r.checkApproval("MissingMethod", nil, nil)
	if err == nil {
		t.Errorf("Expected missing method to yield error'")
	}
	if approved {
		t.Errorf("Expected missing method to cause non-approval")
	}
	fmt.Printf("Err %v", err)

}
func TestStorage(t *testing.T) {

	js := `
	function testStorage(){
		storage.Put("mykey", "myvalue")
		a = storage.Get("mykey")

		storage.Put("mykey", ["a", "list"])  	// Should result in "a,list"
		a += storage.Get("mykey")


		storage.Put("mykey", {"an": "object"}) 	// Should result in "[object Object]"
		a += storage.Get("mykey")


		storage.Put("mykey", JSON.stringify({"an": "object"})) // Should result in '{"an":"object"}'
		a += storage.Get("mykey")

		a += storage.Get("missingkey")		//Missing keys should result in empty string
		storage.Put("","missing key==noop") // Can't store with 0-length key
		a += storage.Get("")				// Should result in ''

		var b = new BigNumber(2)
		var c = new BigNumber(16)//"0xf0",16)
		var d = b.plus(c)
		console.log(d)
		return a
	}
`
	r, err := initRuleEngine(js)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}

	v, err := r.execute("testStorage", nil)

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	retval, err := v.ToString()

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	exp := `myvaluea,list[object Object]{"an":"object"}`
	if retval != exp {
		t.Errorf("Unexpected data, expected '%v', got '%v'", exp, retval)
	}
	fmt.Printf("Err %v", err)

}

const ExampleTxWindow = `
	function big(str){
		if(str.slice(0,2) == "0x"){ return new BigNumber(str.slice(2),16)}
		return new BigNumber(str)
	}

	// Time window: 1 week
	var window = 1000* 3600*24*7;

	// Limit : 1 ether
	var limit = new BigNumber("1e18");

	function isLimitOk(transaction){
		var value = big(transaction.value)
		// Start of our window function
		var windowstart = new Date().getTime() - window;

		var txs = [];
		var stored = storage.Get('txs');

		if(stored != ""){
			txs = JSON.parse(stored)
		}
		// First, remove all that have passed out of the time-window
		var newtxs = txs.filter(function(tx){return tx.tstamp > windowstart});
		console.log(txs, newtxs.length);

		// Secondly, aggregate the current sum
		sum = new BigNumber(0)

		sum = newtxs.reduce(function(agg, tx){ return big(tx.value).plus(agg)}, sum);
		console.log("ApproveTx > Sum so far", sum);
		console.log("ApproveTx > Requested", value.toNumber());

		// Would we exceed weekly limit ?
		return sum.plus(value).lt(limit)

	}
	function ApproveTx(r){
		console.log(r)
		console.log(typeof(r))
		if (isLimitOk(r.transaction)){
			return "Approve"
		}
		return "Nope"
	}

	/**
	* OnApprovedTx(str) is called when a transaction has been approved and signed. The parameter
 	* 'response_str' contains the return value that will be sent to the external caller.
	* The return value from this method is ignore - the reason for having this callback is to allow the
	* ruleset to keep track of approved transactions.
	*
	* When implementing rate-limited rules, this callback should be used.
	* If a rule responds with neither 'Approve' nor 'Reject' - the tx goes to manual processing. If the user
	* then accepts the transaction, this method will be called.
	*
	* TLDR; Use this method to keep track of signed transactions, instead of using the data in ApproveTx.
	*/
 	function OnApprovedTx(resp){
		var value = big(resp.tx.value)
		var txs = []
		// Load stored transactions
		var stored = storage.Get('txs');
		if(stored != ""){
			txs = JSON.parse(stored)
		}
		// Add this to the storage
		txs.push({tstamp: new Date().getTime(), value: value});
		storage.Put("txs", JSON.stringify(txs));
	}

`

func dummyTx(value hexutil.Big) *core2.SignTxRequest {

	to, _ := mixAddr("000000000000000000000000000000000000dead")
	from, _ := mixAddr("000000000000000000000000000000000000dead")
	n := hexutil.Uint64(3)
	gas := hexutil.Uint64(21000)
	gasPrice := hexutil.Big(*big.NewInt(2000000))

	return &core2.SignTxRequest{
		Transaction: core2.SendTxArgs{
			From:     *from,
			To:       to,
			Value:    value,
			Nonce:    n,
			GasPrice: gasPrice,
			Gas:      gas,
		},
		Callinfo: []core2.ValidationInfo{
			{Typ: "Warning", Message: "All your base are bellong to us"},
		},
		Meta: core2.Metadata{Remote: "remoteip", Local: "localip", Scheme: "inproc"},
	}
}
func dummyTxWithV(value uint64) *core2.SignTxRequest {

	v := big.NewInt(0).SetUint64(value)
	h := hexutil.Big(*v)
	return dummyTx(h)
}
func dummySigned(value *big.Int) *types.Transaction {
	to := common.HexToAddress("000000000000000000000000000000000000dead")
	gas := uint64(21000)
	gasPrice := big.NewInt(2000000)
	data := make([]byte, 0)
	return types.NewTransaction(3, to, value, gas, gasPrice, data)

}
func TestLimitWindow(t *testing.T) {

	r, err := initRuleEngine(ExampleTxWindow)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}

	// 0.3 ether: 429D069189E0000 wei
	v := big.NewInt(0).SetBytes(common.Hex2Bytes("0429D069189E0000"))
	h := hexutil.Big(*v)
	// The first three should succeed
	for i := 0; i < 3; i++ {
		unsigned := dummyTx(h)
		resp, err := r.ApproveTx(unsigned)
		if err != nil {
			t.Errorf("Unexpected error %v", err)
		}
		if !resp.Approved {
			t.Errorf("Expected check to resolve to 'Approve'")
		}
		// Create a dummy signed transaction

		response := ethapi.SignTransactionResult{
			Tx:  dummySigned(v),
			Raw: common.Hex2Bytes("deadbeef"),
		}
		r.OnApprovedTx(response)
	}
	// Fourth should fail
	resp, err := r.ApproveTx(dummyTx(h))
	if resp.Approved {
		t.Errorf("Expected check to resolve to 'Reject'")
	}

}

// dontCallMe is used as a next-handler that does not want to be called - it invokes test failure
type dontCallMe struct {
	t *testing.T
}

func (d *dontCallMe) OnSignerStartup(info core2.StartupInfo) {
}

func (d *dontCallMe) ApproveTx(request *core2.SignTxRequest) (core2.SignTxResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core2.SignTxResponse{}, core2.ErrRequestDenied
}

func (d *dontCallMe) ApproveSignData(request *core2.SignDataRequest) (core2.SignDataResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core2.SignDataResponse{}, core2.ErrRequestDenied
}

func (d *dontCallMe) ApproveExport(request *core2.ExportRequest) (core2.ExportResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core2.ExportResponse{}, core2.ErrRequestDenied
}

func (d *dontCallMe) ApproveImport(request *core2.ImportRequest) (core2.ImportResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core2.ImportResponse{}, core2.ErrRequestDenied
}

func (d *dontCallMe) ApproveListing(request *core2.ListRequest) (core2.ListResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core2.ListResponse{}, core2.ErrRequestDenied
}

func (d *dontCallMe) ApproveNewAccount(request *core2.NewAccountRequest) (core2.NewAccountResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core2.NewAccountResponse{}, core2.ErrRequestDenied
}

func (d *dontCallMe) ShowError(message string) {
	d.t.Fatalf("Did not expect next-handler to be called")
}

func (d *dontCallMe) ShowInfo(message string) {
	d.t.Fatalf("Did not expect next-handler to be called")
}

func (d *dontCallMe) OnApprovedTx(tx ethapi.SignTransactionResult) {
	d.t.Fatalf("Did not expect next-handler to be called")
}

//TestContextIsCleared tests that the rule-engine does not retain variables over several requests.
// if it does, that would be bad since developers may rely on that to store data,
// instead of using the disk-based data storage
func TestContextIsCleared(t *testing.T) {

	js := `
	function ApproveTx(){
		if (typeof foobar == 'undefined') {
			foobar = "Approve"
 		}
		console.log(foobar)
		if (foobar == "Approve"){
			foobar = "Reject"
		}else{
			foobar = "Approve"
		}
		return foobar
	}
	`
	ui := &dontCallMe{t}
	r, err := NewRuleEvaluator(ui, storage.NewEphemeralStorage(), storage.NewEphemeralStorage())
	if err != nil {
		t.Fatalf("Failed to create js engine: %v", err)
	}
	if err = r.Init(js); err != nil {
		t.Fatalf("Failed to load bootstrap js: %v", err)
	}
	tx := dummyTxWithV(0)
	r1, err := r.ApproveTx(tx)
	r2, err := r.ApproveTx(tx)
	if r1.Approved != r2.Approved {
		t.Errorf("Expected execution context to be cleared between executions")
	}
}

func TestSignData(t *testing.T) {

	js := `function ApproveListing(){
    return "Approve"
}
function ApproveSignData(r){
    if( r.address.toLowerCase() == "0x694267f14675d7e1b9494fd8d72fefe1755710fa")
    {
        if(r.message.indexOf("bazonk") >= 0){
            return "Approve"
        }
        return "Reject"
    }
    // Otherwise goes to manual processing
}`
	r, err := initRuleEngine(js)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}
	message := []byte("baz bazonk foo")
	hash, msg := core2.SignHash(message)
	raw := hexutil.Bytes(message)
	addr, _ := mixAddr("0x694267f14675d7e1b9494fd8d72fefe1755710fa")

	fmt.Printf("address %v %v\n", addr.String(), addr.Original())
	resp, err := r.ApproveSignData(&core2.SignDataRequest{
		Address: *addr,
		Message: msg,
		Hash:    hash,
		Meta:    core2.Metadata{Remote: "remoteip", Local: "localip", Scheme: "inproc"},
		Rawdata: raw,
	})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if !resp.Approved {
		t.Fatalf("Expected approved")
	}
}
