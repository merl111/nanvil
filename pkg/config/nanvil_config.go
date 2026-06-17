package config

// NanvilConfiguration holds nanvil dev-node settings.
type NanvilConfiguration struct {
	// Enabled turns on nanvil dev RPC methods and dev-mode behavior.
	Enabled bool `yaml:"Enabled"`
	// ImpersonationEnabled allows witness bypass for impersonated accounts.
	ImpersonationEnabled bool `yaml:"ImpersonationEnabled"`
	// DisablePoolBalanceChecks skips balance checks in the mempool.
	DisablePoolBalanceChecks bool `yaml:"DisablePoolBalanceChecks"`
	// PrintTraces logs VM execution traces for invocations and mined transactions.
	PrintTraces bool `yaml:"PrintTraces"`
	// AutoMine mines a block when a transaction enters the mempool.
	AutoMine bool `yaml:"AutoMine"`
	// BlockTimeSeconds is interval between blocks when AutoMine uses timer (0 = instant on tx).
	BlockTimeSeconds uint32 `yaml:"BlockTimeSeconds"`
	// MineEmptyBlocks allows interval mining to produce blocks with no transactions.
	MineEmptyBlocks bool `yaml:"MineEmptyBlocks"`
	// EmptyBlockIntervalSeconds mines empty blocks on this interval when BlockTime is 0.
	EmptyBlockIntervalSeconds uint32 `yaml:"EmptyBlockIntervalSeconds"`
	// Accounts is the number of dev accounts to generate.
	Accounts int `yaml:"Accounts"`
	// Balance is default GAS balance for each dev account in GAS fractions (8 decimals).
	Balance int64 `yaml:"Balance"`
	// Mnemonic is the BIP39 phrase for dev accounts.
	Mnemonic string `yaml:"Mnemonic"`
}

// DefaultNanvil returns sensible nanvil defaults.
func DefaultNanvil() NanvilConfiguration {
	return NanvilConfiguration{
		Enabled:              true,
		ImpersonationEnabled: true,
		AutoMine:             true,
		Accounts:             10,
		Balance:              10_000_0000_0000, // 10,000 GAS
		Mnemonic:             "test test test test test test test test test test test junk",
	}
}
