// Package server runs san_remote's two servers inside another program, as
// the tower does: the hub the San Remote extension attaches to (what
// `san_remote serve` runs) and the site services (what `san_remote api`
// runs). The extension, the CLI, the MCP server and the tower's San Remote
// connection reach them as they reach those, at the same addresses.
package server

import (
	"log/slog"
	"net/http"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/api"
	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/facebook"
	"github.com/wargasipil/san_remote/internal/gemini"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/linkedin"
	"github.com/wargasipil/san_remote/internal/medium"
	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/threads"
	"github.com/wargasipil/san_remote/internal/tiktok"
	"github.com/wargasipil/san_remote/internal/vidara"
	"github.com/wargasipil/san_remote/internal/x"
)

const (
	// HubAddr is where `san_remote serve` listens, and where the extension,
	// the CLI and the MCP server look for it.
	HubAddr = "127.0.0.1:7476"
	// APIAddr is where `san_remote api` listens, and where a San Remote
	// connection looks for it.
	APIAddr = api.DefaultAddr
)

// Hub is the server the San Remote extension attaches to, and the CLI, the
// MCP server and API send their commands to.
type Hub struct{ h *hub.Hub }

func NewHub(log *slog.Logger) *Hub { return &Hub{hub.New(log)} }

// Handler serves the hub: on a loopback address only, with only the San
// Remote extension allowed to attach.
func (h *Hub) Handler() http.Handler { return h.h.Handler() }

// Browsers lists the attached browsers, the longest attached first.
func (h *Hub) Browsers() []*v1.Browser { return h.h.Browsers() }

// Close ends the browsers' Attach streams, which a graceful shutdown would
// otherwise wait on until its timeout.
func (h *Hub) Close() { h.h.Close() }

// API serves the site services (Threads, X, Facebook, Vidara, Medium,
// LinkedIn, TikTok, ChatGPT, Gemini, Claude), reaching the
// browser through the hub at hubURL, as `san_remote api --server hubURL`
// does. The sites' steps go to log as debug lines.
func API(hubURL string, log *slog.Logger) http.Handler {
	c := hub.NewClient(hubURL)
	b := page.Remote{C: c}
	progress := func(site string) func(string) {
		return func(step string) { log.Debug(site, "step", step) }
	}
	a := &api.API{
		Client:   c,
		Server:   hubURL,
		Threads:  &threads.Service{B: b, Progress: progress("threads")},
		X:        &x.Service{B: b, Progress: progress("x")},
		Facebook: &facebook.Service{B: b, Progress: progress("facebook")},
		Vidara:   &vidara.Service{B: b, Progress: progress("vidara")},
		Medium:   &medium.Service{B: b, Progress: progress("medium")},
		LinkedIn: &linkedin.Service{B: b, Progress: progress("linkedin")},
		TikTok:   &tiktok.Service{B: b, Progress: progress("tiktok")},
		ChatGPT:  &chatgpt.Service{B: b, Progress: progress("chatgpt")},
		Gemini:   &gemini.Service{B: b, Progress: progress("gemini")},
		Claude:   &claude.Service{B: b, Progress: progress("claude")},
	}
	return a.Handler()
}
