package etw

import (
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestSpanIDToActivityID(t *testing.T) {
	spID, err := trace.SpanIDFromHex("abcdef0123456789")
	if err != nil {
		t.Fatal(err)
	}
	g := spanIDtoActivityID(spID)
	got := g.String()
	want := "abcdef01-2345-6789-0000-000000000000"
	if !strings.EqualFold(got, want) {
		t.Fatalf("got %s, wanted %s", got, want)
	}
}
