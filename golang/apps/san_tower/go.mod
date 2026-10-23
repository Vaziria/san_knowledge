module github.com/wargasipil/san_tower

go 1.26.0

require (
	connectrpc.com/connect v1.21.1
	github.com/urfave/cli/v3 v3.14.0
	github.com/wargasipil/san_message v0.0.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
)

replace github.com/wargasipil/san_message => ../../packages/san_message
