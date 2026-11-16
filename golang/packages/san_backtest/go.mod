module github.com/wargasipil/san_backtest

go 1.26.0

require (
	github.com/urfave/cli/v3 v3.14.0
	github.com/wargasipil/san_exness v0.0.0
	golang.org/x/sys v0.49.0
	google.golang.org/protobuf v1.36.12
)

require github.com/coder/websocket v1.8.15 // indirect

replace github.com/wargasipil/san_exness => ../san_exness
