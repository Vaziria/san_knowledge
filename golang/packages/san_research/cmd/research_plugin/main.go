// Command research_plugin is an approach plugin serving every san_research
// approach (RegisterAll: the trend approaches, their chains, the demo
// chains and the dip fade scalpers), for san_backtest's Registry.Load, its
// CLI's --plugin and San Tower's plugins folder:
//
//	go build -o plugins/research.exe ./cmd/research_plugin
//
// San Tower has these approaches compiled in already, so there its names
// are taken; the plugin is here for the proof that a plugin decides
// exactly as compiled in (plugin_proof_test.go) and as a starting point.
package main

import (
	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/plugin"
	research "github.com/wargasipil/san_research"
)

func main() {
	r := bt.NewRegistry()
	research.RegisterAll(r)
	plugin.Serve(plugin.Makers(r))
}
