package db

import (
	"fmt"
	"testing"
)

func TestTextArrayGoesThroughPostgres(t *testing.T) {
	gdb, ctx := scratchDatabase(t)
	for _, want := range []TextArray{{}, {"pipa", "listrik"}, {`a "b"`, `c\d`, "e,f", "{g}", " h ", "NULL"}} {
		var got TextArray
		if err := gdb.WithContext(ctx).Raw(`SELECT (?::text::text[])::text`, want).Row().Scan(&got); err != nil {
			t.Fatalf("%q: %v", want, err)
		}
		if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", want) {
			t.Fatalf("through Postgres %q, want %q", got, want)
		}
	}
}

func TestParseTextArrayRefusesBadLiterals(t *testing.T) {
	for _, s := range []string{"", "pipa", "{pipa", `{"pipa}`, "{NULL}", "{{a}}", `{"a"b}`} {
		if _, err := parseTextArray(s); err == nil {
			t.Errorf("parseTextArray(%q) = nil error", s)
		}
	}
}
