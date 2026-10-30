package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/wargasipil/san_tower/internal/engine"
)

// toolCall is a POST to /api/tools/<token>: a call to one of the tools.
type toolCall struct {
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
}

// tools answers the MCP server of a Claude Ask (CLI) step's claude: GET
// the step's tools, POST a call to one, whose result is {"result": "..."}.
func (s *Server) tools(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/api/tools/")
	if s.engine == nil || token == "" || strings.Contains(token, "/") {
		http.NotFound(w, r)
		return
	}
	var out any
	var err error
	switch r.Method {
	case http.MethodGet:
		out, err = s.engine.Tools(token)
	case http.MethodPost:
		var call toolCall
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&call); err != nil {
			http.Error(w, "the body is not a tool call: "+err.Error(), http.StatusBadRequest)
			return
		}
		var result string
		result, err = s.engine.CallTool(token, call.Name, call.Command, call.Arguments)
		out = map[string]string{"result": result}
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
		return
	}
	if errors.Is(err, engine.ErrNoToolSession) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(out)
}
