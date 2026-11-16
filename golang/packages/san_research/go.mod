module github.com/wargasipil/san_research

go 1.26.0

require (
	github.com/wargasipil/san_backtest v0.0.0
	github.com/wargasipil/san_exness v0.0.0
	golang.org/x/image v0.45.0
)

require (
	github.com/coder/websocket v1.8.15 // indirect
	golang.org/x/sys v0.49.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace (
	github.com/wargasipil/san_backtest => ../san_backtest
	github.com/wargasipil/san_exness => ../san_exness
)
