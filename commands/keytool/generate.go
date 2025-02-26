package main

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pborman/uuid"
	"gopkg.in/urfave/cli.v1"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/commands/utils"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/accounts/keystore"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/crypto"
)

type outputGenerate struct {
	Address      string
	AddressEIP55 string
}

var commandGenerate = cli.Command{
	Name:      "generate",
	Usage:     "generate new keyfile",
	ArgsUsage: "[ <keyfile> ]",
	Description: `
Generate a new keyfile.

If you want to encrypt an existing private key, it can be specified by setting
--privatekey with the location of the file containing the private key.
`,
	Flags: []cli.Flag{
		passphraseFlag,
		jsonFlag,
		cli.StringFlag{
			Name:  "privatekey",
			Usage: "file containing a raw private key to encrypt",
		},
		//utils.AddressHRPFlag,
	},
	Action: func(ctx *cli.Context) error {
		//hrp := ctx.String(utils.AddressHRPFlag.Name)
		//if err := common.SetAddressHRP(hrp); err != nil {
		//	return err
		//}

		// Check if keyfile path given and make sure it doesn't already exist.
		keyfilepath := ctx.Args().First()
		if keyfilepath == "" {
			keyfilepath = defaultKeyfileName
		}
		if _, err := os.Stat(keyfilepath); err == nil {
			utils.Fatalf("Keyfile already exists at %s.", keyfilepath)
		} else if !os.IsNotExist(err) {
			utils.Fatalf("Error checking if keyfile exists: %v", err)
		}

		var privateKey *ecdsa.PrivateKey
		var err error
		if file := ctx.String("privatekey"); file != "" {
			// Load private key from file.
			privateKey, err = crypto.LoadECDSA(file)
			if err != nil {
				utils.Fatalf("Can't load private key: %v", err)
			}
		} else {
			// If not loaded, generate random.
			privateKey, err = crypto.GenerateKey()
			if err != nil {
				utils.Fatalf("Failed to generate random private key: %v", err)
			}
		}

		// Create the keyfile object with a random UUID.
		id := uuid.NewRandom()
		key := &keystore.Key{
			Id:         id,
			Address:    crypto.PubkeyToAddress(privateKey.PublicKey),
			PrivateKey: privateKey,
		}

		// Encrypt key with passphrase.
		passphrase := getPassphrase(ctx, true)
		keyjson, err := keystore.EncryptKey(key, passphrase, keystore.StandardScryptN, keystore.StandardScryptP)
		if err != nil {
			utils.Fatalf("Error encrypting key: %v", err)
		}

		// Store the file to disk.
		if err := os.MkdirAll(filepath.Dir(keyfilepath), 0700); err != nil {
			utils.Fatalf("Could not create directory %s", filepath.Dir(keyfilepath))
		}
		if err := ioutil.WriteFile(keyfilepath, keyjson, 0600); err != nil {
			utils.Fatalf("Failed to write keyfile to %s: %v", keyfilepath, err)
		}

		// Output some information.
		out := outputGenerate{
			Address: key.Address.String(),
		}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			fmt.Println("Address:", out.Address)
		}
		return nil
	},
}
