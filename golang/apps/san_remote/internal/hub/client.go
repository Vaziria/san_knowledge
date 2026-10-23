package hub

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
)

// DefaultServer is where `san_remote serve` listens unless told otherwise,
// and where the extension looks for it.
const DefaultServer = "http://127.0.0.1:7476"

// NewClient is the RemoteService client of the CLI and the MCP server.
func NewClient(server string) remotev1connect.RemoteServiceClient {
	return remotev1connect.NewRemoteServiceClient(http.DefaultClient, server)
}

// ClientError says plainly when the server is not running, rather than
// passing on a dial error.
func ClientError(err error, server string) error {
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return fmt.Errorf("san_remote is not running at %s: start it with `san_remote serve`", server)
	}
	return err
}
