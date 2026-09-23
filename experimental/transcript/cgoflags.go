package main

// Link against the libvosk.dll sitting next to these sources, so a plain
// "go run ." works without any CGO_* environment variables.

// #cgo LDFLAGS: -L${SRCDIR} -lvosk
import "C"
