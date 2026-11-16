//go:build !windows

package pipe

import (
	"io"
	"os"
)

func spinReader(*os.File) io.Reader { return nil }
