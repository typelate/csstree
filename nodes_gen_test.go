package csstree

import (
	"bytes"
	"os"
	"testing"

	"github.com/typelate/csstree/internal/nodegen"
)

func TestNodesGenIsUpToDate(t *testing.T) {
	want, err := nodegen.Generate(".")
	if err != nil {
		t.Fatalf("nodegen.Generate(.) error = %v", err)
	}
	got, err := os.ReadFile("nodes_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("nodes_gen.go is stale: run go generate")
	}
}
