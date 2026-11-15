module github.com/wargasipil/san_research

go 1.26.0

require (
	github.com/wargasipil/san_backtest v0.0.0
	golang.org/x/image v0.45.0
)

require (
	github.com/coder/websocket v1.8.15 // indirect
	github.com/wargasipil/san_exness v0.0.0 // indirect
)

replace (
	github.com/wargasipil/san_backtest => ../san_backtest
	github.com/wargasipil/san_exness => ../san_exness
)
