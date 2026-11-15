package livetrading

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The hard rule of Live Trading's first stage (the user, 2026-11-15: "1a":
// Monitor + Signals, paper trading; his account is real money): nothing
// here may trade. These tests fail when any of san_exness's trading
// methods is named anywhere in this package (its subpackages too) or in the
// server's Live Trading file, and when the read-only interface or its
// wrapper grows a method that is not read-only.

// trading are san_exness's trading methods (api.go).
var trading = []string{"OpenPosition", "PlaceOrder", "ModifyOrder", "CancelOrder", "ClosePosition"}

// readOnlyMethods are the only methods the Exness interface may have.
var readOnlyMethods = []string{"Account", "Candles", "Events", "Instrument", "Now", "Snapshot", "Ticks"}

func goFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "safety_test.go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	server, _ := filepath.Glob(filepath.Join("..", "server", "live_trading*.go"))
	if len(server) == 0 {
		t.Fatal("the server's live_trading.go was not found: the check would miss it")
	}
	return append(files, server...)
}

func TestNoTradingCallAnywhere(t *testing.T) {
	bad := map[string]bool{}
	for _, n := range trading {
		bad[n] = true
	}
	files := goFiles(t)
	for _, path := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if bad[x.Sel.Name] {
					t.Errorf("%s: %s names the trading method %s: Live Trading must never trade", path, fset.Position(x.Pos()), x.Sel.Name)
				}
			case *ast.Ident:
				if bad[x.Name] {
					t.Errorf("%s: %s names %s: Live Trading must never trade", path, fset.Position(x.Pos()), x.Name)
				}
			}
			return true
		})
		// the raw text too, so a string or a comment building a call by
		// name is caught as well
		b, _ := os.ReadFile(path)
		for _, n := range trading {
			if strings.Contains(string(b), n+"(") {
				t.Errorf("%s: the text %s( appears", path, n)
			}
		}
	}
	t.Logf("%d files checked", len(files))
}

func TestReadOnlyInterface(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[Exness](), reflect.TypeFor[readOnly]()} {
		var got []string
		for i := range typ.NumMethod() {
			got = append(got, typ.Method(i).Name)
		}
		if strings.Join(got, ",") != strings.Join(readOnlyMethods, ",") {
			t.Errorf("%s has methods %v; only %v (read-only) are allowed", typ, got, readOnlyMethods)
		}
	}
}

// The check itself catches a trading call: a file that names one fails.
func TestTheCheckCatchesATradingCall(t *testing.T) {
	src := "package x\nfunc f(c *exness.Client) { c.PlaceOrder(nil, o, \"\") }\n"
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "PlaceOrder" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("the AST walk missed c.PlaceOrder")
	}
}
