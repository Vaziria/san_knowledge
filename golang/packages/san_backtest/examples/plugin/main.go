// Command plugin is an approach plugin serving every example approach (and
// its chain), for Registry.Load, the CLI's --plugin and San Tower's
// plugins folder. Build it with
//
//	go build -o plugins/examples.exe ./examples/plugin
//
// It serves the examples under their own names, so load it into a
// registry that does not have them compiled in already (Load refuses a
// taken name). Your own plugin is the same few lines: plugin.Serve with
// your makers.
package main

import (
	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/examples"
	"github.com/wargasipil/san_backtest/plugin"
)

func main() {
	r := bt.NewRegistry()
	examples.Register(r)
	plugin.Serve(plugin.Makers(r))
}
