module github.com/wargasipil/san_backtest

go 1.26.0

require (
	github.com/urfave/cli/v3 v3.14.0
	github.com/wargasipil/san_exness v0.0.0
)

require github.com/coder/websocket v1.8.15 // indirect

replace github.com/wargasipil/san_exness => ../san_exness
