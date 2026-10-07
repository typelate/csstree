// Command nodegen writes nodes_gen.go, the boilerplate of the AST node
// types of package csstree (see package internal/nodegen). Run it with go
// generate in the package directory.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/typelate/csstree/internal/nodegen"
)

func main() {
	dir := flag.String("C", ".", "the directory of package csstree")
	output := flag.String("o", "nodes_gen.go", "the file to write, relative to -C")
	flag.Parse()

	if err := run(*dir, *output); err != nil {
		fmt.Fprintln(os.Stderr, "nodegen:", err)
		os.Exit(1)
	}
}

func run(dir, output string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	source, err := nodegen.Generate(".")
	if err != nil {
		return err
	}
	return os.WriteFile(output, source, 0o644)
}
