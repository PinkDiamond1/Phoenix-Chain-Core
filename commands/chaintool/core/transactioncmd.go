package core

import (
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/configs"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"gopkg.in/urfave/cli.v1"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common/hexutil"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/rlp"
)

var (
	SendTransactionCmd = cli.Command{
		Name:   "sendTransaction",
		Usage:  "send a transaction",
		Action: sendTransactionCmd,
		Flags:  sendTransactionCmdFlags,
	}
	SendRawTransactionCmd = cli.Command{
		Name:   "sendRawTransaction",
		Usage:  "send a raw transaction",
		Action: sendRawTransactionCmd,
		Flags:  sendRawTransactionCmdFlags,
	}
	GetTxReceiptCmd = cli.Command{
		Name:   "getTxReceipt",
		Usage:  "get transaction receipt by hash",
		Action: getTxReceiptCmd,
		Flags:  getTxReceiptCmdFlags,
	}
)

func getTxReceiptCmd(c *cli.Context) {
	hash := c.String(TransactionHashFlag.Name)
	parseConfigJson(c.String(ConfigPathFlag.Name))
	GetTxReceipt(hash)
}

func GetTxReceipt(txHash string) (Receipt, error) {
	var receipt = Receipt{}
	res, _ := Send([]string{txHash}, "phoenixchain_getTransactionReceipt")
	e := json.Unmarshal([]byte(res), &receipt)
	if e != nil {
		panic(fmt.Sprintf("parse get receipt result error ! \n %s", e.Error()))
	}

	if receipt.Result.BlockHash == "" {
		panic("no receipt found")
	}
	out, _ := json.MarshalIndent(receipt, "", "  ")
	fmt.Println(string(out))
	return receipt, nil
}

func sendTransactionCmd(c *cli.Context) error {
	from := c.String(TxFromFlag.Name)
	to := c.String(TxToFlag.Name)
	value := c.String(TransferValueFlag.Name)
	parseConfigJson(c.String(ConfigPathFlag.Name))

	hash, err := SendTransaction(from, to, value)
	if err != nil {
		return fmt.Errorf("Send transaction error: %v", err)
	}

	fmt.Printf("tx hash: %s", hash)
	return nil
}

func sendRawTransactionCmd(c *cli.Context) error {
	from := c.String(TxFromFlag.Name)
	to := c.String(TxToFlag.Name)
	value := c.String(TransferValueFlag.Name)
	pkFile := c.String(PKFilePathFlag.Name)

	parseConfigJson(c.String(ConfigPathFlag.Name))

	hash, err := SendRawTransaction(from, to, value, pkFile)
	if err != nil {
		return fmt.Errorf("Send transaction error: %v", err)
	}

	fmt.Printf("tx hash: %s", hash)
	return nil
}

func SendTransaction(from, to, value string) (string, error) {
	var tx TxParams
	if from == "" {
		from = config.From
	}
	tx.From = from
	tx.To = to
	tx.Gas = config.Gas
	tx.GasPrice = config.GasPrice

	//todo
	if !strings.HasPrefix(value, "0x") {
		intValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("transfer value to int error.%s", err))
		}
		value = hexutil.EncodeBig(big.NewInt(intValue))
	}
	tx.Value = value

	params := make([]TxParams, 1)
	params[0] = tx

	res, _ := Send(params, "phoenixchain_sendTransaction")
	response := parseResponse(res)

	return response.Result, nil
}

func SendRawTransaction(from, to, value string, pkFilePath string) (string, error) {
	if len(accountPool) == 0 {
		parsePkFile(pkFilePath)
	}
	var v int64
	var err error
	if strings.HasPrefix(value, "0x") {
		bigValue, _ := hexutil.DecodeBig(value)
		v = bigValue.Int64()
	} else {
		v, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("transfer value to int error.%s", err))
		}
	}

	////
	//
	//for k, v := range accountPool {
	//	fmt.Println("acc", k.Hex())
	//	fmt.Println("value", fmt.Sprintf("%+v", v))
	//}

	acc, ok := accountPool[common.MustStringToAddress(from)]
	if !ok {
		return "", fmt.Errorf("private key not found in private key file,addr:%s", from)
	}
	nonce := GetNonce(from)
	nonce++

	//// getBalance
	//
	//unlock := JsonParam{
	//	Jsonrpc: "2.0",
	//	Method:  "personal_unlockAccount",
	//	// {"method": "phoenixchain_getBalance", "params": [account, pwd, expire]}
	//	// {"jsonrpc":"2.0", "method":"eth_getBalance","params":["0xde1e758511a7c67e7db93d1c23c1060a21db4615","latest"],"id":67}
	//	Params: []interface{}{from, "latest"},
	//	Id:     1,
	//}
	//
	//// unlock
	//s, e := HttpPost(unlock)
	//if nil != e {
	//	fmt.Println("the gat balance err:", e)
	//}
	//fmt.Println("the balance:", s)

	newTx := getSignedTransaction(from, to, v, acc.Priv, nonce)

	hash, err := sendRawTransaction(newTx)
	if err != nil {
		panic(err)
	}
	return hash, nil
}

func SendRawTransactionWithData(configPath string,from string, to string, value int64, priv *ecdsa.PrivateKey,data []byte) (string, error) {
	parseConfigJson(configPath)

	nonce := GetNonce(from)
	nonce++

	newTx := getSignedTransactionWithData(from, to, value, priv, nonce,data)

	hash, err := sendRawTransaction(newTx)
	if err != nil {
		panic(err)
	}
	return hash, nil
}

func sendRawTransaction(transaction *types.Transaction) (string, error) {
	bytes, _ := rlp.EncodeToBytes(transaction)
	res, err := Send([]string{hexutil.Encode(bytes)}, "phoenixchain_sendRawTransaction")
	if err != nil {
		panic(err)
	}
	response := parseResponse(res)

	return response.Result, nil
}

func getSignedTransaction(from, to string, value int64, priv *ecdsa.PrivateKey, nonce uint64) *types.Transaction {
	gas, _ := strconv.Atoi(config.Gas)
	gasPrice, _ := new(big.Int).SetString(config.GasPrice, 10)
	newTx, err := types.SignTx(types.NewTransaction(nonce, common.MustStringToAddress(to), big.NewInt(value), uint64(gas), gasPrice, []byte{}), types.NewEIP155Signer(new(big.Int).SetInt64(configs.ChainId)), priv)
	if err != nil {
		panic(fmt.Errorf("sign error,%s", err.Error()))
	}
	return newTx
}

func getSignedTransactionWithData(from, to string, value int64, priv *ecdsa.PrivateKey, nonce uint64,data []byte) *types.Transaction {
	gas, _ := strconv.Atoi(config.Gas)
	gasPrice, _ := new(big.Int).SetString(config.GasPrice, 10)
	fmt.Println("gas,gasPrice are ",gas,gasPrice)
	newTx, err := types.SignTx(types.NewTransaction(nonce, common.MustStringToAddress(to), big.NewInt(value), uint64(gas), gasPrice, data), types.NewEIP155Signer(new(big.Int).SetInt64(configs.ChainId)), priv)
	if err != nil {
		panic(fmt.Errorf("sign error,%s", err.Error()))
	}
	return newTx
}

func GetNonce(addr string) uint64 {
	res, _ := Send([]string{addr, "latest"}, "phoenixchain_getTransactionCount")
	response := parseResponse(res)
	nonce, _ := hexutil.DecodeBig(response.Result)
	fmt.Println("the nonce of "+addr +" is ", nonce)
	return nonce.Uint64()
}
