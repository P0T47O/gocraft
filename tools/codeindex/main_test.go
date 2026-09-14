package main

import (
	"go/parser"
	"testing"
)

func TestReceiverNames(t *testing.T) {
	for _, name := range []string{"World", "*World", "Cache[int]"} {
		expr, err := parser.ParseExpr(name)
		if err != nil {
			t.Fatal(err)
		}
		want := name
		if name == "Cache[int]" {
			want = "Cache"
		}
		if got := receiverName(expr); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestConflictingFlags(t *testing.T) {
	if err := run(true, true); err == nil {
		t.Fatal("accepted conflicting modes")
	}
}
