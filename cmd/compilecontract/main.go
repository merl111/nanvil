package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
)

func main() {
	in := flag.String("in", "", "path to contract .go source")
	out := flag.String("out", "contract", "output file prefix (without extension)")
	name := flag.String("name", "ExplorerDeployTest", "contract manifest name")
	flag.Parse()
	if *in == "" {
		fmt.Fprintln(os.Stderr, "usage: compilecontract --in contract.go [--out contract] [--name MyContract]")
		os.Exit(2)
	}
	opts := &compiler.Options{
		Name:         *name,
		Outfile:      *out,
		Ext:          "nef",
		ManifestFile: *out + ".manifest.json",
	}
	if _, err := compiler.CompileAndSave(*in, opts); err != nil {
		fmt.Fprintf(os.Stderr, "compile failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s.nef and %s.manifest.json\n", *out, *out)
}
