package accounts_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/accounts"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/producer"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGenerateDevAccounts(t *testing.T) {
	accs, err := accounts.Generate("test test test test test test test test test test test junk", 3)
	require.NoError(t, err)
	require.Len(t, accs, 3)
	require.NotEmpty(t, accs[0].Address)
	require.NotEmpty(t, accs[0].WIF)
}

func TestNewValidatorSigner(t *testing.T) {
	s, err := accounts.NewValidatorSigner()
	require.NoError(t, err)
	require.NotEmpty(t, s.Script())
}

func TestNewManager(t *testing.T) {
	m, err := accounts.NewManager("test test test test test test test test test test test junk", 5)
	require.NoError(t, err)
	require.Len(t, m.Accounts, 5)
	require.NotNil(t, m.Validator)
}

func TestRegisterVerificationScripts(t *testing.T) {
	accs, err := accounts.Generate("test test test test test test test test test test test junk", 2)
	require.NoError(t, err)
	accounts.RegisterVerificationScripts(accs)
	script, ok := accounts.LookupVerificationScript(accs[1].Signer.ScriptHash())
	require.True(t, ok)
	require.Equal(t, accs[1].Account.Contract.Script, script)
}

func TestFormatGAS(t *testing.T) {
	require.Equal(t, "10000 GAS", accounts.FormatGAS(10_000_0000_0000))
	require.Equal(t, "100.5 GAS", accounts.FormatGAS(100_5000_0000))
}

func TestPrintStartupInfo(t *testing.T) {
	m, err := accounts.NewManager("test test test test test test test test test test test junk", 2)
	require.NoError(t, err)
	var buf strings.Builder
	m.PrintStartupInfo(&buf, 10_000_0000_0000, "test test test test test test test test test test test junk")
	out := buf.String()
	require.Contains(t, out, "Available Accounts")
	require.Contains(t, out, "Mnemonic:")
	require.Contains(t, out, "10000 GAS")
	require.Contains(t, out, m.Accounts[0].WIF)
	require.Contains(t, out, accounts.ValidatorWIF())
}

func TestFundAll(t *testing.T) {
	opts := nanvilcfg.DefaultStartOptions()
	opts.Accounts = 10
	pubHex, err := accounts.ValidatorPublicKeyHex()
	require.NoError(t, err)

	bcCfg := nanvilcfg.BlockchainConfig(opts)
	bcCfg.StandbyCommittee = []string{pubHex}
	log := zaptest.NewLogger(t)
	bc, err := core.NewBlockchain(storage.NewMemoryStore(), bcCfg, log)
	require.NoError(t, err)
	t.Cleanup(bc.Close)
	go bc.Run()

	mgr, err := accounts.NewManager(opts.Mnemonic, opts.Accounts)
	require.NoError(t, err)
	builder := producer.NewBlockBuilder(bc, mgr.Validator)

	require.NoError(t, mgr.FundAll(bc, opts.Balance, func(txs ...*transaction.Transaction) error {
		_, err := builder.Mine(txs...)
		return err
	}))

	expected := big.NewInt(opts.Balance)
	for _, acc := range mgr.Accounts {
		require.Equal(t, expected, bc.GetUtilityTokenBalance(acc.Signer.ScriptHash()), acc.Address)
	}
	require.Equal(t, uint32(opts.Accounts), bc.BlockHeight())
}
