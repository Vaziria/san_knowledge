package ownview

// The Brain view's API (docs/brain.md; the user, 2026-10-31: brains in the
// UI): the brains with their stats, a brain's claims a page at a time, its
// latest runs, and what it recalls for a task. Read-only: a brain changes
// through learning, runs and sleep, not the UI. A brain is opened per
// request, as the store is.

import (
	"errors"
	"net/http"
	"strconv"

	"san_knowledge/internal/kb"
)

func (h *api) brains() kb.Brains { return kb.Brains{Data: h.o.Dir} }

// brainName is the request's brain, root when it names none.
func brainName(r *http.Request) string {
	if b := r.URL.Query().Get("brain"); b != "" {
		return b
	}
	return kb.RootBrain
}

// withBrain runs fn on the request's brain and writes its result as JSON.
func (h *api) withBrain(w http.ResponseWriter, r *http.Request, fn func(*kb.Brain) (any, error)) {
	var out any
	err := h.brains().With(brainName(r), func(br *kb.Brain) error {
		var err error
		out, err = fn(br)
		return err
	})
	if errors.Is(err, kb.ErrNoBrain) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// brainList is every brain with its stats. With none yet, it opens none
// (opening root would make it).
func (h *api) brainList(w http.ResponseWriter, r *http.Request) {
	b := h.brains()
	list, err := b.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	type row struct {
		kb.BrainInfo
		Stats *kb.BrainStats `json:"stats"`
	}
	out := []row{}
	for _, info := range list {
		var st *kb.BrainStats
		err := b.With(info.Name, func(br *kb.Brain) error {
			var err error
			st, err = br.Stats()
			return err
		})
		if err != nil {
			writeErr(w, err)
			return
		}
		out = append(out, row{info, st})
	}
	writeJSON(w, http.StatusOK, map[string]any{"brains": out, "default": kb.RootBrain})
}

func (h *api) brainClaims(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	query := kb.ClaimQuery{Search: q.Get("q"), Status: q.Get("status"), Sort: q.Get("sort"), Offset: offset, Limit: limit}
	h.withBrain(w, r, func(br *kb.Brain) (any, error) { return br.ClaimPage(query) })
}

func (h *api) brainRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	h.withBrain(w, r, func(br *kb.Brain) (any, error) {
		runs, err := br.Runs(limit)
		return map[string]any{"runs": runs}, err
	})
}

func (h *api) brainRecall(w http.ResponseWriter, r *http.Request) {
	task := r.URL.Query().Get("q")
	if task == "" {
		writeErr(w, errors.New("give a task: ?q="))
		return
	}
	h.withBrain(w, r, func(br *kb.Brain) (any, error) {
		rec, err := br.Recall(task, kb.RecallOptions{Keep: 20})
		if err != nil {
			return nil, err
		}
		return map[string]any{"recall": rec, "markdown": rec.Markdown()}, nil
	})
}
