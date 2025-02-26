package web

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	accountsWallet "github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/accounts/wallet"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/log"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rpc"
	"github.com/robertkrimen/otto"
)

// bridge is a collection of JavaScript utility methods to bride the .js runtime
// environment and the Go RPC connection backing the remote method calls.
type bridge struct {
	client   *rpc.Client  // RPC client to execute Ethereum requests through
	prompter UserPrompter // Input prompter to allow interactive user feedback
	printer  io.Writer    // Output writer to serialize any display strings to
}

// newBridge creates a new JavaScript wrapper around an RPC client.
func newBridge(client *rpc.Client, prompter UserPrompter, printer io.Writer) *bridge {
	return &bridge{
		client:   client,
		prompter: prompter,
		printer:  printer,
	}
}

// NewAccount is a wrapper around the personal.newAccount RPC method that uses a
// non-echoing password prompt to acquire the passphrase and executes the original
// RPC method (saved in jphoenixchain.newAccount) with it to actually execute the RPC call.
func (b *bridge) NewAccount(call otto.FunctionCall) (response otto.Value) {
	var (
		password string
		confirm  string
		err      error
	)
	switch {
	// No password was specified, prompt the user for it
	case len(call.ArgumentList) == 0:
		if password, err = b.prompter.PromptPassword("Passphrase: "); err != nil {
			throwJSException(err.Error())
		}
		if confirm, err = b.prompter.PromptPassword("Repeat passphrase: "); err != nil {
			throwJSException(err.Error())
		}
		if password != confirm {
			throwJSException("passphrases don't match!")
		}

	// A single string password was specified, use that
	case len(call.ArgumentList) == 1 && call.Argument(0).IsString():
		password, _ = call.Argument(0).ToString()

	// Otherwise fail with some error
	default:
		throwJSException("expected 0 or 1 string argument")
	}
	// Password acquired, execute the call and return
	ret, err := call.Otto.Call("jphoenixchain.newAccount", nil, password)
	if err != nil {
		throwJSException(err.Error())
	}
	return ret
}

// OpenWallet is a wrapper around personal.openWallet which can interpret and
// react to certain error messages, such as the Trezor PIN matrix request.
func (b *bridge) OpenWallet(call otto.FunctionCall) (response otto.Value) {
	// Make sure we have a wallet specified to open
	if !call.Argument(0).IsString() {
		throwJSException("first argument must be the wallet URL to open")
	}
	wallet := call.Argument(0)

	var passwd otto.Value
	if call.Argument(1).IsUndefined() || call.Argument(1).IsNull() {
		passwd, _ = otto.ToValue("")
	} else {
		passwd = call.Argument(1)
	}
	// Open the wallet and return if successful in itself
	val, err := call.Otto.Call("jphoenixchain.openWallet", nil, wallet, passwd)
	if err == nil {
		return val
	}
	// Wallet open failed, report error unless it's a PIN entry
	if !strings.HasSuffix(err.Error(), accountsWallet.ErrTrezorPINNeeded.Error()) {
		throwJSException(err.Error())
	}
	// Trezor PIN matrix input requested, display the matrix to the user and fetch the data
	fmt.Fprintf(b.printer, "Look at the device for number positions\n\n")
	fmt.Fprintf(b.printer, "7 | 8 | 9\n")
	fmt.Fprintf(b.printer, "--+---+--\n")
	fmt.Fprintf(b.printer, "4 | 5 | 6\n")
	fmt.Fprintf(b.printer, "--+---+--\n")
	fmt.Fprintf(b.printer, "1 | 2 | 3\n\n")

	if input, err := b.prompter.PromptPassword("Please enter current PIN: "); err != nil {
		throwJSException(err.Error())
	} else {
		passwd, _ = otto.ToValue(input)
	}
	if val, err = call.Otto.Call("jphoenixchain.openWallet", nil, wallet, passwd); err != nil {
		throwJSException(err.Error())
	}
	return val
}

// UnlockAccount is a wrapper around the personal.unlockAccount RPC method that
// uses a non-echoing password prompt to acquire the passphrase and executes the
// original RPC method (saved in jphoenixchain.unlockAccount) with it to actually execute
// the RPC call.
func (b *bridge) UnlockAccount(call otto.FunctionCall) (response otto.Value) {
	// Make sure we have an account specified to unlock
	if !call.Argument(0).IsString() {
		throwJSException("first argument must be the account to unlock")
	}
	account := call.Argument(0)

	// If password is not given or is the null value, prompt the user for it
	var passwd otto.Value

	if call.Argument(1).IsUndefined() || call.Argument(1).IsNull() {
		fmt.Fprintf(b.printer, "Unlock account %s\n", account)
		if input, err := b.prompter.PromptPassword("Passphrase: "); err != nil {
			throwJSException(err.Error())
		} else {
			passwd, _ = otto.ToValue(input)
		}
	} else {
		if !call.Argument(1).IsString() {
			throwJSException("password must be a string")
		}
		passwd = call.Argument(1)
	}
	// Third argument is the duration how long the account must be unlocked.
	duration := otto.NullValue()
	if call.Argument(2).IsDefined() && !call.Argument(2).IsNull() {
		if !call.Argument(2).IsNumber() {
			throwJSException("unlock duration must be a number")
		}
		duration = call.Argument(2)
	}
	// Send the request to the backend and return
	val, err := call.Otto.Call("jphoenixchain.unlockAccount", nil, account, passwd, duration)
	if err != nil {
		throwJSException(err.Error())
	}
	return val
}

// Sign is a wrapper around the personal.sign RPC method that uses a non-echoing password
// prompt to acquire the passphrase and executes the original RPC method (saved in
// jphoenixchain.sign) with it to actually execute the RPC call.
func (b *bridge) Sign(call otto.FunctionCall) (response otto.Value) {
	var (
		message = call.Argument(0)
		account = call.Argument(1)
		passwd  = call.Argument(2)
	)

	if !message.IsString() {
		throwJSException("first argument must be the message to sign")
	}
	if !account.IsString() {
		throwJSException("second argument must be the account to sign with")
	}

	// if the password is not given or null ask the user and ensure password is a string
	if passwd.IsUndefined() || passwd.IsNull() {
		fmt.Fprintf(b.printer, "Give password for account %s\n", account)
		if input, err := b.prompter.PromptPassword("Passphrase: "); err != nil {
			throwJSException(err.Error())
		} else {
			passwd, _ = otto.ToValue(input)
		}
	}
	if !passwd.IsString() {
		throwJSException("third argument must be the password to unlock the account")
	}

	// Send the request to the backend and return
	val, err := call.Otto.Call("jphoenixchain.sign", nil, message, account, passwd)
	if err != nil {
		throwJSException(err.Error())
	}
	return val
}

// Sleep will block the console for the specified number of seconds.
func (b *bridge) Sleep(call otto.FunctionCall) (response otto.Value) {
	if call.Argument(0).IsNumber() {
		sleep, _ := call.Argument(0).ToInteger()
		time.Sleep(time.Duration(sleep) * time.Second)
		return otto.TrueValue()
	}
	return throwJSException("usage: sleep(<number of seconds>)")
}

// SleepBlocks will block the console for a specified number of new blocks optionally
// until the given timeout is reached.
func (b *bridge) SleepBlocks(call otto.FunctionCall) (response otto.Value) {
	var (
		blocks = int64(0)
		sleep  = int64(9999999999999999) // indefinitely
	)
	// Parse the input parameters for the sleep
	nArgs := len(call.ArgumentList)
	if nArgs == 0 {
		throwJSException("usage: sleepBlocks(<n blocks>[, max sleep in seconds])")
	}
	if nArgs >= 1 {
		if call.Argument(0).IsNumber() {
			blocks, _ = call.Argument(0).ToInteger()
		} else {
			throwJSException("expected number as first argument")
		}
	}
	if nArgs >= 2 {
		if call.Argument(1).IsNumber() {
			sleep, _ = call.Argument(1).ToInteger()
		} else {
			throwJSException("expected number as second argument")
		}
	}
	// go through the console, this will allow web3 to call the appropriate
	// callbacks if a delayed response or notification is received.
	blockNumber := func() int64 {
		result, err := call.Otto.Run("eth.blockNumber")
		if err != nil {
			throwJSException(err.Error())
		}
		block, err := result.ToInteger()
		if err != nil {
			throwJSException(err.Error())
		}
		return block
	}
	// Poll the current block number until either it ot a timeout is reached
	targetBlockNr := blockNumber() + blocks
	deadline := time.Now().Add(time.Duration(sleep) * time.Second)

	for time.Now().Before(deadline) {
		if blockNumber() >= targetBlockNr {
			return otto.TrueValue()
		}
		time.Sleep(time.Second)
	}
	return otto.FalseValue()
}

type jsonrpcCall struct {
	ID     int64
	Method string
	Params []interface{}
}

// Send implements the web3 provider "send" method.
func (b *bridge) Send(call otto.FunctionCall) (response otto.Value) {
	// Remarshal the request into a Go value.
	JSON, _ := call.Otto.Object("JSON")
	reqVal, err := JSON.Call("stringify", call.Argument(0))
	if err != nil {
		throwJSException(err.Error())
	}
	var (
		rawReq = reqVal.String()
		dec    = json.NewDecoder(strings.NewReader(rawReq))
		reqs   []jsonrpcCall
		batch  bool
	)
	dec.UseNumber() // avoid float64s
	if rawReq[0] == '[' {
		batch = true
		dec.Decode(&reqs)
	} else {
		batch = false
		reqs = make([]jsonrpcCall, 1)
		dec.Decode(&reqs[0])
	}

	// Execute the requests.
	resps, _ := call.Otto.Object("new Array()")
	for _, req := range reqs {
		resp, _ := call.Otto.Object(`({"jsonrpc":"2.0"})`)
		resp.Set("id", req.ID)
		var result json.RawMessage
		if err = b.client.Call(&result, req.Method, req.Params...); err == nil {
			if result == nil {
				// Special case null because it is decoded as an empty
				// raw message for some reason.
				resp.Set("result", otto.NullValue())
			} else {
				resultVal, err := JSON.Call("parse", string(result))
				if err != nil {
					setError(resp, -32603, err.Error(), nil)
				} else {
					resp.Set("result", resultVal)
				}
			}
		} else {
			code := -32603
			var data interface{}
			if err, ok := err.(rpc.Error); ok {
				code = err.ErrorCode()
			}
			if err, ok := err.(rpc.DataError); ok {
				data = err.ErrorData()
			}
			setError(resp, code, err.Error(), data)
		}
		resps.Call("push", resp)
	}

	// Return the responses either to the callback (if supplied)
	// or directly as the return value.
	if batch {
		response = resps.Value()
	} else {
		response, _ = resps.Get("0")
	}
	if fn := call.Argument(1); fn.Class() == "Function" {
		fn.Call(otto.NullValue(), otto.NullValue(), response)
		return otto.UndefinedValue()
	}
	return response
}

func setError(resp *otto.Object, code int, msg string, data interface{}) {
	err := make(map[string]interface{})
	err["code"] = code
	err["message"] = msg
	if data != nil {
		err["data"] = data
	}
	resp.Set("error", err)
}

// throwJSException panics on an otto.Value. The Otto VM will recover from the
// Go panic and throw msg as a JavaScript error.
func throwJSException(msg interface{}) otto.Value {
	val, err := otto.ToValue(msg)
	if err != nil {
		log.Error("Failed to serialize JavaScript exception", "exception", msg, "err", err)
	}
	panic(val)
}
