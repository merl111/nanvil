package ncast

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

// FormatGAS formats a datoshi amount as GAS.
func FormatGAS(amount int64) string {
	return fixedn.Fixed8(amount).String() + " GAS"
}

// FormatGASBig formats a big.Int datoshi amount as GAS.
func FormatGASBig(amount *big.Int) string {
	if amount == nil {
		return "0 GAS"
	}
	return fixedn.ToString(amount, 8) + " GAS"
}

// ParseGAS parses a human GAS amount to datoshi.
func ParseGAS(s string) (*big.Int, error) {
	return fixedn.FromString(s, 8)
}

// ShortHash shortens a hex hash for display.
func ShortHash(h string, n int) string {
	if n <= 0 {
		n = 8
	}
	h = strings.TrimPrefix(strings.TrimPrefix(h, "0x"), "0X")
	if len(h) <= n*2 {
		return "0x" + h
	}
	return "0x" + h[:n] + "…" + h[len(h)-n:]
}

// FormatTimeMs formats a millisecond timestamp.
func FormatTimeMs(ms uint64) string {
	if ms == 0 {
		return "—"
	}
	return time.UnixMilli(int64(ms)).Format("2006-01-02 15:04:05")
}

// PrintJSON prints v as indented JSON to stdout.
func PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// PrintKV prints key-value lines.
func PrintKV(pairs ...string) {
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Fprintf(os.Stdout, "%-18s %s\n", pairs[i]+":", pairs[i+1])
	}
}
