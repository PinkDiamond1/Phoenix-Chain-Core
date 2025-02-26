package main

import (
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto/bls"
	"encoding/hex"
	"fmt"
	"io/ioutil"

	"gopkg.in/urfave/cli.v1"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/commands/utils"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/accounts/keystore"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto"
)

type outputSign struct {
	Signature string
}

var msgfileFlag = cli.StringFlag{
	Name:  "msgfile",
	Usage: "file containing the message to sign/verify",
}

var commandSignMessage = cli.Command{
	Name:      "signmessage",
	Usage:     "sign a message",
	ArgsUsage: "<keyfile> <message>",
	Description: `
Sign the message with a keyfile.

To sign a message contained in a file, use the --msgfile flag.
`,
	Flags: []cli.Flag{
		passphraseFlag,
		jsonFlag,
		msgfileFlag,
	},
	Action: func(ctx *cli.Context) error {
		message := getMessage(ctx, 1)

		// Load the keyfile.
		keyfilepath := ctx.Args().First()
		keyjson, err := ioutil.ReadFile(keyfilepath)
		if err != nil {
			utils.Fatalf("Failed to read the keyfile at '%s': %v", keyfilepath, err)
		}

		// Decrypt key with passphrase.
		passphrase := getPassphrase(ctx, false)
		key, err := keystore.DecryptKey(keyjson, passphrase)
		if err != nil {
			utils.Fatalf("Error decrypting key: %v", err)
		}

		signature, err := crypto.Sign(signHash(message), key.PrivateKey)
		if err != nil {
			utils.Fatalf("Failed to sign message: %v", err)
		}
		out := outputSign{Signature: hex.EncodeToString(signature)}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			fmt.Println("Signature:", out.Signature)
		}
		return nil
	},
}

type blsProof struct {
	BlsProof string
}

var blsKeyfileFlag = cli.StringFlag{
	Name:  "blsKey",
	Usage: "file containing the blsKey",
}

var commandBlsProof = cli.Command{
	Name:      "blsProof",
	Usage:     "generate a bls proof",
	ArgsUsage: "<blskeyfile>",
	Description: `
Generate a bls proof with a keyfile.
`,
	Flags: []cli.Flag{
		blsKeyfileFlag,
		jsonFlag,
	},
	Action: func(ctx *cli.Context) error {
		// Load the keyfile.
		keyfilepath := ctx.String(blsKeyfileFlag.Name)

		blsKey,err:=bls.LoadBLS(keyfilepath)
		if err != nil {
			return fmt.Errorf("bls.LoadBLS error,%s", err.Error())
		}

		proof, _ := blsKey.MakeSchnorrNIZKP()
		proofByte, _ := proof.MarshalText()
		var proofHex bls.SchnorrProofHex
		proofHex.UnmarshalText(proofByte)

		out := blsProof{BlsProof: proofHex.String()}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			fmt.Println("BlsProof:", out.BlsProof)
		}
		return nil
	},
}

type outputVerify struct {
	Success            bool
	RecoveredAddress   string
	RecoveredPublicKey string
}

var commandVerifyMessage = cli.Command{
	Name:      "verifymessage",
	Usage:     "verify the signature of a signed message",
	ArgsUsage: "<address> <signature> <message>",
	Description: `
Verify the signature of the message.
It is possible to refer to a file containing the message.`,
	Flags: []cli.Flag{
		jsonFlag,
		msgfileFlag,
		//utils.AddressHRPFlag,
	},
	Action: func(ctx *cli.Context) error {
		//hrp := ctx.String(utils.AddressHRPFlag.Name)
		//if err := common.SetAddressHRP(hrp); err != nil {
		//	return err
		//}
		addressStr := ctx.Args().First()
		signatureHex := ctx.Args().Get(1)
		message := getMessage(ctx, 2)

		if !common.IsHexAddress(addressStr) {
			utils.Fatalf("Invalid address: %s", addressStr)
		}
		address, err := common.StringToAddress(addressStr)
		if err != nil {
			utils.Fatalf("decode address fail: %s", addressStr)
		}
		signature, err := hex.DecodeString(signatureHex)
		if err != nil {
			utils.Fatalf("Signature encoding is not hexadecimal: %v", err)
		}

		recoveredPubkey, err := crypto.SigToPub(signHash(message), signature)
		if err != nil || recoveredPubkey == nil {
			utils.Fatalf("Signature verification failed: %v", err)
		}
		recoveredPubkeyBytes := crypto.FromECDSAPub(recoveredPubkey)
		recoveredAddress := crypto.PubkeyToAddress(*recoveredPubkey)
		success := address == recoveredAddress

		out := outputVerify{
			Success:            success,
			RecoveredPublicKey: hex.EncodeToString(recoveredPubkeyBytes),
			RecoveredAddress:   recoveredAddress.String(),
		}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			if out.Success {
				fmt.Println("Signature verification successful!")
			} else {
				fmt.Println("Signature verification failed!")
			}
			fmt.Println("Recovered public key:", out.RecoveredPublicKey)
			fmt.Println("Recovered address:", out.RecoveredAddress)
		}
		return nil
	},
}

func getMessage(ctx *cli.Context, msgarg int) []byte {
	if file := ctx.String("msgfile"); file != "" {
		if len(ctx.Args()) > msgarg {
			utils.Fatalf("Can't use --msgfile and message argument at the same time.")
		}
		msg, err := ioutil.ReadFile(file)
		if err != nil {
			utils.Fatalf("Can't read message file: %v", err)
		}
		return msg
	} else if len(ctx.Args()) == msgarg+1 {
		return []byte(ctx.Args().Get(msgarg))
	}
	utils.Fatalf("Invalid number of arguments: want %d, got %d", msgarg+1, len(ctx.Args()))
	return nil
}
