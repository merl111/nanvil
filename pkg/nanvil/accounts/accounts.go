package accounts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	neoio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/tyler-smith/go-bip39"
)

const validatorWIF = "KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY"

// DevAccount is a prefunded development account.
type DevAccount struct {
	Index   int
	Address string
	WIF     string
	Account *wallet.Account
	Signer  neotest.SingleSigner
}

// Manager holds dev accounts and the validator signer.
type Manager struct {
	Validator neotest.Signer
	Accounts  []DevAccount
}

var (
	verificationScripts   map[util.Uint160][]byte
	verificationScriptsMu sync.RWMutex
)

// RegisterVerificationScripts records verification scripts for dev accounts so
// RPC fee estimation can resolve wallet placeholder witnesses.
func RegisterVerificationScripts(accs []DevAccount) {
	verificationScriptsMu.Lock()
	defer verificationScriptsMu.Unlock()
	if verificationScripts == nil {
		verificationScripts = make(map[util.Uint160][]byte)
	}
	for _, acc := range accs {
		if acc.Account == nil || acc.Account.Contract == nil {
			continue
		}
		hash := acc.Signer.ScriptHash()
		verificationScripts[hash] = slices.Clone(acc.Account.Contract.Script)
	}
}

// LookupVerificationScript returns a known verification script for the account.
func LookupVerificationScript(hash util.Uint160) ([]byte, bool) {
	verificationScriptsMu.RLock()
	defer verificationScriptsMu.RUnlock()
	script, ok := verificationScripts[hash]
	return script, ok
}

// NewValidatorSigner returns the fixed single-validator signer used by nanvil.
func NewValidatorSigner() (neotest.Signer, error) {
	acc, err := wallet.NewAccountFromWIF(validatorWIF)
	if err != nil {
		return nil, err
	}
	pubs := keys.PublicKeys{acc.PublicKey()}
	if err := acc.ConvertMultisig(1, pubs); err != nil {
		return nil, err
	}
	return neotest.NewMultiSigner(acc), nil
}

// ValidatorPublicKeyHex returns standby committee entry for the validator.
func ValidatorPublicKeyHex() (string, error) {
	acc, err := wallet.NewAccountFromWIF(validatorWIF)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(acc.PublicKey().Bytes()), nil
}

// Generate derives dev accounts from a BIP39 mnemonic.
func Generate(mnemonic string, count int) ([]DevAccount, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}
	seed := bip39.NewSeed(mnemonic, "")
	out := make([]DevAccount, 0, count)
	for i := range count {
		priv, err := keyFromSeed(seed, i)
		if err != nil {
			return nil, fmt.Errorf("account %d: %w", i, err)
		}
		acc := wallet.NewAccountFromPrivateKey(priv)
		wif := priv.WIF()
		addr := address.Uint160ToString(acc.Contract.ScriptHash())
		out = append(out, DevAccount{
			Index:   i,
			Address: addr,
			WIF:     wif,
			Account: acc,
			Signer:  neotest.NewSingleSigner(acc),
		})
	}
	return out, nil
}

// NewManager creates dev accounts and validator signer.
func NewManager(mnemonic string, count int) (*Manager, error) {
	val, err := NewValidatorSigner()
	if err != nil {
		return nil, err
	}
	accs, err := Generate(mnemonic, count)
	if err != nil {
		return nil, err
	}
	return &Manager{Validator: val, Accounts: accs}, nil
}

// FundAll transfers GAS to each dev account, one block per account.
func (m *Manager) FundAll(bc *core.Blockchain, amount int64, mine func(txs ...*transaction.Transaction) error) error {
	for i := range m.Accounts {
		if err := m.FundAddress(bc, m.Accounts[i].Signer.ScriptHash(), amount, mine); err != nil {
			return fmt.Errorf("account %d: %w", i, err)
		}
	}
	return nil
}

// FundAddress transfers GAS to a specific address.
func (m *Manager) FundAddress(bc *core.Blockchain, to util.Uint160, amount int64, mine func(...*transaction.Transaction) error) error {
	gasHash, err := bc.GetNativeContractScriptHash(nativenames.Gas)
	if err != nil {
		return err
	}
	script, err := smartcontract.CreateCallScript(gasHash, "transfer",
		m.Validator.ScriptHash(), to, amount, nil)
	if err != nil {
		return err
	}
	tx := transaction.New(script, 0)
	tx.ValidUntilBlock = bc.BlockHeight() + bc.GetMaxValidUntilBlockIncrement()
	tx.Signers = []transaction.Signer{{
		Account: m.Validator.ScriptHash(),
		Scopes:  transaction.Global,
	}}
	if err := m.prepareTx(bc, tx); err != nil {
		return err
	}
	return mine(tx)
}

func (m *Manager) prepareTx(bc *core.Blockchain, tx *transaction.Transaction) error {
	addNetworkFee(bc, tx, m.Validator)
	if err := addSystemFee(bc, tx); err != nil {
		return err
	}
	return m.Validator.SignTx(bc.GetConfig().Magic, tx)
}

func addNetworkFee(bc *core.Blockchain, tx *transaction.Transaction, signers ...neotest.Signer) {
	baseFee := bc.GetBaseExecFee()
	size := neoio.GetVarSize(tx)
	for _, sgr := range signers {
		netFee, sizeDelta := fee.Calculate(baseFee, sgr.Script())
		tx.NetworkFee += netFee
		size += sizeDelta
	}
	tx.NetworkFee += int64(size)*bc.FeePerByte() + bc.CalculateAttributesFee(tx)
}

func addSystemFee(bc *core.Blockchain, tx *transaction.Transaction) error {
	lastBlock, err := bc.GetBlock(bc.GetHeaderHash(bc.BlockHeight()))
	if err != nil {
		return err
	}
	b := &block.Block{
		Header: block.Header{
			Index:     bc.BlockHeight() + 1,
			Timestamp: lastBlock.Timestamp + 1,
		},
	}
	ttx := *tx
	ic, err := bc.GetTestVM(trigger.Application, &ttx, b)
	if err != nil {
		return err
	}
	defer ic.Finalize()
	ic.VM.LoadWithFlags(tx.Script, callflag.All)
	if err := ic.VM.Run(); err != nil {
		return fmt.Errorf("test invoke: %w", err)
	}
	tx.SystemFee = ic.VM.GasConsumed()
	return nil
}

const gasFractionalUnit = 100_000_000

// PrintStartupInfo writes Anvil-style dev account details to w.
func (m *Manager) PrintStartupInfo(w io.Writer, balance int64, mnemonic string) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Available Accounts")
	fmt.Fprintln(w, "==================")
	fmt.Fprintf(w, "Mnemonic:          %s\n", mnemonic)
	fmt.Fprintf(w, "Default balance:   %s per account\n", formatGAS(balance))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Validator (funds dev accounts)")
	fmt.Fprintf(w, "  Address: %s\n", address.Uint160ToString(m.Validator.ScriptHash()))
	fmt.Fprintf(w, "  WIF:     %s\n", validatorWIF)
	fmt.Fprintln(w)
	for _, acc := range m.Accounts {
		fmt.Fprintf(w, "(%d) %s (%s)\n", acc.Index, acc.Address, formatGAS(balance))
		fmt.Fprintf(w, "    WIF: %s\n", acc.WIF)
	}
	fmt.Fprintln(w)
}

// ValidatorWIF returns the fixed validator private key (WIF format).
func ValidatorWIF() string {
	return validatorWIF
}

// FormatGAS formats a GAS amount in datoshi units for display.
func FormatGAS(amount int64) string {
	return formatGAS(amount)
}

func formatGAS(amount int64) string {
	if amount < 0 {
		return fmt.Sprintf("%d GAS", amount)
	}
	whole := amount / gasFractionalUnit
	frac := amount % gasFractionalUnit
	if frac == 0 {
		return fmt.Sprintf("%d GAS", whole)
	}
	fracStr := fmt.Sprintf("%08d", frac)
	fracStr = strings.TrimRight(fracStr, "0")
	return fmt.Sprintf("%d.%s GAS", whole, fracStr)
}

func keyFromSeed(seed []byte, index int) (*keys.PrivateKey, error) {
	buf := append(append([]byte{}, seed...), byte(index), byte(index>>8), byte(index>>16), byte(index>>24))
	sum := sha256.Sum256(buf)
	return keys.NewPrivateKeyFromBytes(sum[:])
}
