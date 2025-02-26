package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"gopkg.in/urfave/cli.v1"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/commands/utils"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto"
)

type outputGenkeypair struct {
	PrivateKey string
	PublicKey  string
}

var commandGenkeypair = cli.Command{
	Name:      "genkeypair",
	Usage:     "generate new private key pair",
	ArgsUsage: "[ ]",
	Description: `
Generate a new private key pair.
.
`,
	Flags: []cli.Flag{
		jsonFlag,
	},
	Action: func(ctx *cli.Context) error {
		// Check if keyfile path given and make sure it doesn't already exist.
		var privateKey *ecdsa.PrivateKey
		var err error
		// generate random.
		privateKey, err = crypto.GenerateKey()
		if err != nil {
			utils.Fatalf("Failed to generate random private key: %v", err)
		}

		// Output some information.
		out := outputGenkeypair{
			PublicKey:  hex.EncodeToString(crypto.FromECDSAPub(&privateKey.PublicKey)[1:]),
			PrivateKey: hex.EncodeToString(crypto.FromECDSA(privateKey)),
		}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			fmt.Println("PrivateKey: ", out.PrivateKey)
			fmt.Println("PublicKey : ", out.PublicKey)
		}
		return nil
	},
}
