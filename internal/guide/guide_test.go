// This file contains the code snippets from the developer's guide embedded into
// Go tests. This ensures that any code published in out guides will not break
// accidentally via some code update. If some API changes nonetheless that needs
// modifying this file, please port any modification over into the developer's
// guide wiki pages too!

package guide

import (
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/accounts/keystore"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/ethereum/core/types"
)

// Tests that the account management snippets work correctly.
func TestAccountManagement(t *testing.T) {
	// Create a temporary folder to work with
	workdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to create temporary work dir: %v", err)
	}
	defer os.RemoveAll(workdir)

	// Create an encrypted keystore with standard crypto parameters
	ks := keystore.NewKeyStore(filepath.Join(workdir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)

	// Create a new account with the specified encryption passphrase
	newAcc, err := ks.NewAccount("Creation password")
	if err != nil {
		t.Fatalf("Failed to create new account: %v", err)
	}
	// Export the newly created account with a different passphrase. The returned
	// data from this method invocation is a JSON encoded, encrypted key-file
	jsonAcc, err := ks.Export(newAcc, "Creation password", "Export password")
	if err != nil {
		t.Fatalf("Failed to export account: %v", err)
	}
	// Update the passphrase on the account created above inside the local keystore
	if err := ks.Update(newAcc, "Creation password", "Update password"); err != nil {
		t.Fatalf("Failed to update account: %v", err)
	}
	// Delete the account updated above from the local keystore
	if err := ks.Delete(newAcc, "Update password"); err != nil {
		t.Fatalf("Failed to delete account: %v", err)
	}
	// Import back the account we've exported (and then deleted) above with yet
	// again a fresh passphrase
	if _, err := ks.Import(jsonAcc, "Export password", "Import password"); err != nil {
		t.Fatalf("Failed to import account: %v", err)
	}
	// Create a new account to sign transactions with
	signer, err := ks.NewAccount("Signer password")
	if err != nil {
		t.Fatalf("Failed to create signer account: %v", err)
	}
	tx, chain := new(types.Transaction), big.NewInt(1)

	// Sign a transaction with a single authorization
	if _, err := ks.SignTxWithPassphrase(signer, "Signer password", tx, chain); err != nil {
		t.Fatalf("Failed to sign with passphrase: %v", err)
	}
	// Sign a transaction with multiple manually cancelled authorizations
	if err := ks.Unlock(signer, "Signer password"); err != nil {
		t.Fatalf("Failed to unlock account: %v", err)
	}
	if _, err := ks.SignTx(signer, tx, chain); err != nil {
		t.Fatalf("Failed to sign with unlocked account: %v", err)
	}
	if err := ks.Lock(signer.Address); err != nil {
		t.Fatalf("Failed to lock account: %v", err)
	}
	// Sign a transaction with multiple automatically cancelled authorizations
	if err := ks.TimedUnlock(signer, "Signer password", time.Second); err != nil {
		t.Fatalf("Failed to time unlock account: %v", err)
	}
	if _, err := ks.SignTx(signer, tx, chain); err != nil {
		t.Fatalf("Failed to sign with time unlocked account: %v", err)
	}
}
