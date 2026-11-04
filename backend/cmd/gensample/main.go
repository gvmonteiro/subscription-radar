// Command gensample writes a synthetic card statement to stdout.
//
//	go run ./cmd/gensample -seed 7 > statement.csv
package main

import (
	"flag"
	"os"

	"subscription-radar/internal/sample"
)

func main() {
	seed := flag.Uint64("seed", sample.DefaultSeed, "random seed; the demo uses the default")
	flag.Parse()
	_, _ = os.Stdout.Write(sample.Generate(*seed).CSV)
}
