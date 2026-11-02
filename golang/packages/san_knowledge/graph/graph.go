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
	SectionBrief    = kb.SectionBrief
	DomainBrief     = kb.DomainBrief
	AnnotateResult  = kb.AnnotateResult
	SectionResult   = kb.SectionResult
	// What Store.Search and Store.Explain give.
	Filter         = kb.Filter
	SearchHit      = kb.SearchHit
	Explanation    = kb.Explanation
	ContextMatch   = kb.ContextMatch
	RelatedContext = kb.RelatedContext
)

// Brains (docs/brain.md) of a folder's graph, in <folder>/knowledge_data/brains.
type (
	Brains         = kb.Brains
	Brain          = kb.Brain
	LearnOptions   = kb.LearnOptions
	LearnReport    = kb.LearnReport
	LearnDocReport = kb.LearnDocReport
	SleepOptions   = kb.SleepOptions
	SleepReport    = kb.SleepReport
	// What learning asks the AI and gets back.
	Proposer      = kb.Proposer
	Verifier      = kb.Verifier
	LearnRequest  = kb.LearnRequest
	LearnProposal = kb.LearnProposal
	ProposedClaim = kb.ProposedClaim
	VerifyRequest = kb.VerifyRequest
	Verdict       = kb.Verdict
	// Using a brain for a task: recall, the run, its outcome, and the
	// citation check that gives an answer its outcome.
	Recall         = kb.Recall
	RecallOptions  = kb.RecallOptions
	FactHit        = kb.FactHit
	Recalled       = kb.Recalled
	Run            = kb.Run
	Outcome        = kb.Outcome
	RunReport      = kb.RunReport
	BrainNode      = kb.BrainNode
	Ref            = kb.Ref
	CitationReport = kb.CitationReport
	CitedFact      = kb.CitedFact
	AnswerReport   = kb.AnswerReport
	AnswerOptions  = kb.AnswerOptions
	// A brain's writes, guarded (kb/brain.go).
	Memory    = kb.Memory
	BrainLink = kb.BrainLink
	// What the brain views read (kb/brainview.go).
	BrainInfo  = kb.BrainInfo
	BrainStats = kb.BrainStats
	ClaimQuery = kb.ClaimQuery
	ClaimPage  = kb.ClaimPage
	ClaimRow   = kb.ClaimRow
	SourceRow  = kb.SourceRow
	RunRow     = kb.RunRow
	// Sleep's AI part.
	Dreamer        = kb.Dreamer
	DreamOptions   = kb.DreamOptions
	DreamReport    = kb.DreamReport
	SettleRequest  = kb.SettleRequest
	Settlement     = kb.Settlement
	SettledContext = kb.SettledContext
	PatternRequest = kb.PatternRequest
	PatternAnswer  = kb.PatternAnswer
)

// ErrNoBrain is a brain that is not there.
var ErrNoBrain = kb.ErrNoBrain

// CitationCheck is the check of a run whose outcome the citation check gives.
const CitationCheck = kb.CitationCheck

// BrainsOf are the brains of root's graph.
func BrainsOf(root string) Brains { return kb.Brains{Data: Dir(root)} }

// CheckCitations checks the citations of answer against root's graph.
func CheckCitations(ctx context.Context, root, answer string, v Verifier) (*CitationReport, error) {
	return kb.CheckCitations(ctx, Dir(root), answer, v)
}

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
