// Package graph lets another module keep a knowledge graph of a folder of
// markdown docs with the knowledge tool's own code (internal/kb, which only
// this module can import): the same store, structural sync and AI step. The
// tower's Knowledge page uses it (golang/apps/san_tower, 2026-10-28).
//
// A folder is a project as the knowledge tool sees it: its graph is in
// <folder>/knowledge_data (DataDir), whose config.json says which folders are
// tracked besides docs/ (Init tracks the whole folder).
package graph

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"san_knowledge/internal/kb"
)

type (
	Store      = kb.Store
	Node       = kb.Node
	Link       = kb.Link
	Stats      = kb.Stats
	SyncReport = kb.SyncReport
	AIOptions  = kb.AIOptions
	AIReport   = kb.AIReport
	Annotator  = kb.Annotator
	ClaudeCLI  = kb.ClaudeCLI
	// What an Annotator gets and gives, for one doc.
	AnnotateRequest = kb.AnnotateRequest
	AnnotateResult  = kb.AnnotateResult
	SectionResult   = kb.SectionResult
	// What Store.Search and Store.Explain give.
	Filter         = kb.Filter
	SearchHit      = kb.SearchHit
	Explanation    = kb.Explanation
	ContextMatch   = kb.ContextMatch
	RelatedContext = kb.RelatedContext
)

// Node types.
const (
	TypeDomain  = kb.TypeDomain
	TypeDoc     = kb.TypeDoc
	TypeSection = kb.TypeSection
)

var (
	ErrNotFound = kb.ErrNotFound
	ErrLocked   = kb.ErrLocked
)

// DataDir is the folder of a project's graph, in the project.
const DataDir = "knowledge_data"

// Dir is where root's graph is.
func Dir(root string) string { return filepath.Join(root, DataDir) }

// Init makes root's data folder, and its config.json tracking every markdown
// file in root, unless it has one: a project the knowledge tool already
// keeps keeps its own config.
func Init(root string) error {
	dir := Dir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, kb.ConfigFile)
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	data, err := json.MarshalIndent(kb.Config{Track: []string{"."}}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// With opens root's graph, runs fn and closes it again, so other processes
// (the knowledge tool) can reach it between calls.
func With(root string, fn func(*Store) error) error { return kb.With(Dir(root), fn) }

// RunAI writes summaries, keywords and domains for root's docs that need
// them (kb.RunAI).
func RunAI(ctx context.Context, root string, ann Annotator, opt AIOptions) (*AIReport, error) {
	return kb.RunAI(ctx, Dir(root), ann, opt)
}

// Leiden numbers the communities of a graph, 0 the largest (kb.Leiden).
func Leiden(keys []string, links []Link, resolution float64) map[string]int {
	return kb.Leiden(keys, links, resolution)
}
