// Command hatchbook prints the specimen sheet's manifest: for every page,
// every square by row and column, what it shows and the exact spec that
// drew it.
//
// The sheets carry no labels — there are no fonts in this repo and no
// third-party dependency to bring one in — so this is how a square is read.
// It is generated from the same tables the sketch renders from.
//
//	go run ./tools/hatchbook > out/agent-hatch/manifest.txt
package main

import (
	"fmt"

	"github.com/jaminalder/go-graphics/internal/sketch/hatchbook"
)

func main() {
	fmt.Print(hatchbook.Manifest())
}
