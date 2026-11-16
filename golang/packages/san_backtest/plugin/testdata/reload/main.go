package main

import (
	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/plugin"
)

// version is set by the test's build (-ldflags "-X main.version=2"): a
// rebuilt plugin serves a different approach name.
var version = "1"

type waiter struct{}

func (waiter) Name() string                     { return "reload_v" + version }
func (waiter) Timeframes() []string             { return []string{"M15"} }
func (waiter) Warmup() int                      { return 0 }
func (waiter) OnCandle(c *bt.Context) bt.Signal { return bt.Wait }

func main() {
	plugin.Serve(map[string]func() bt.Strategy{"reload_v" + version: func() bt.Strategy { return waiter{} }})
}
