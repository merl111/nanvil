package ncast_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/ncast"
)

func TestParseGAS(t *testing.T) {
	amt, err := ncast.ParseGAS("1.5")
	if err != nil {
		t.Fatal(err)
	}
	if got := ncast.FormatGASBig(amt); got != "1.5 GAS" {
		t.Fatalf("got %q", got)
	}
}

func TestShortHash(t *testing.T) {
	s := ncast.ShortHash("0xabcdef0123456789abcdef0123456789abcdef01", 4)
	if s != "0xabcd…ef01" {
		t.Fatalf("got %q", s)
	}
}
