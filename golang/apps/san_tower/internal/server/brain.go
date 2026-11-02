package server

// The brains of a knowledge, for the Knowledge page's Brain view (the user,
// 2026-11-01: "make tower can show it"): read-only, a brain opened per call
// as the graph is. A brain changes by learning, runs and sleep (the engine's
// Brain steps), never from here.

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// brainsOf are knowledge id's brains.
func (s *Server) brainsOf(id uint32) (graph.Brains, error) {
	root, err := s.knowledgeRoot(id)
	if err != nil {
		return graph.Brains{}, err
	}
	return graph.BrainsOf(root), nil
}

// withBrain runs fn on brain name (root when empty) of knowledge id.
func (s *Server) withBrain(id uint32, name string, fn func(*graph.Brain) error) error {
	b, err := s.brainsOf(id)
	if err != nil {
		return err
	}
	if name == "" {
		name = "root"
	}
	// Opening root would make it: a knowledge without that brain answers
	// not found instead.
	if _, err := b.Info(name); err != nil {
		return brainError(err)
	}
	return brainError(b.With(name, fn))
}

func brainError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, graph.ErrNoBrain):
		return connect.NewError(connect.CodeNotFound, err)
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	return graphError(err)
}

func (s *Server) ListBrains(_ context.Context, req *connect.Request[v1.ListBrainsRequest]) (*connect.Response[v1.ListBrainsResponse], error) {
	b, err := s.brainsOf(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	list, err := b.List()
	if err != nil {
		return nil, brainError(err)
	}
	res := &v1.ListBrainsResponse{Brains: []*v1.Brain{}}
	for _, info := range list {
		var st *graph.BrainStats
		err := b.With(info.Name, func(br *graph.Brain) error {
			var err error
			st, err = br.Stats()
			return err
		})
		if err != nil {
			return nil, brainError(fmt.Errorf("brain %s: %w", info.Name, err))
		}
		res.Brains = append(res.Brains, &v1.Brain{
			Name: info.Name, Grammar: uint32(info.Grammar), ForkedFrom: info.ForkedFrom, Created: info.Created, LastSleep: info.LastSleep,
			Claims: uint32(st.NodeTypes["claim"]), Concepts: uint32(st.NodeTypes["concept"]), Sources: uint32(st.NodeTypes["source"]),
			Runs: uint32(st.Runs), Passed: uint32(st.Passed), Waiting: uint32(st.Waiting), Retired: uint32(st.Retired),
		})
	}
	return connect.NewResponse(res), nil
}

func (s *Server) ListBrainClaims(_ context.Context, req *connect.Request[v1.ListBrainClaimsRequest]) (*connect.Response[v1.ListBrainClaimsResponse], error) {
	m := req.Msg
	q := graph.ClaimQuery{Search: m.GetQuery(), Status: m.GetStatus(), Sort: m.GetSort(), Offset: int(m.GetOffset()), Limit: int(m.GetLimit())}
	res := &v1.ListBrainClaimsResponse{Claims: []*v1.BrainClaim{}}
	err := s.withBrain(m.GetId(), m.GetBrain(), func(br *graph.Brain) error {
		page, err := br.ClaimPage(q)
		if err != nil {
			return err
		}
		res.Total = uint32(page.Total)
		for _, c := range page.Claims {
			res.Claims = append(res.Claims, &v1.BrainClaim{
				Key: c.Key, Text: c.Text, Status: c.Status, Confidence: c.Confidence, Uses: uint32(c.Uses), Wins: uint32(c.Wins),
				Weight: c.Weight, Retired: c.Retired, Stale: c.Stale, About: c.About, AppliesWhen: c.When,
				Sources: brainSources(c.Sources), Against: brainSources(c.Against),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func brainSources(rows []graph.SourceRow) []*v1.BrainSource {
	out := make([]*v1.BrainSource, len(rows))
	for i, r := range rows {
		out[i] = &v1.BrainSource{Fact: r.Fact, Title: r.Title, Loc: r.Loc, Quote: r.Quote}
	}
	return out
}

func (s *Server) ListBrainRuns(_ context.Context, req *connect.Request[v1.ListBrainRunsRequest]) (*connect.Response[v1.ListBrainRunsResponse], error) {
	res := &v1.ListBrainRunsResponse{Runs: []*v1.BrainRun{}}
	err := s.withBrain(req.Msg.GetId(), req.Msg.GetBrain(), func(br *graph.Brain) error {
		runs, err := br.Runs(int(req.Msg.GetLimit()))
		if err != nil {
			return err
		}
		for _, r := range runs {
			p := &v1.BrainRun{Key: r.Key, Task: r.Task, Check: r.Check, Author: r.Author, Created: r.Created, Used: uint32(r.Used),
				HasOutcome: r.Passed != nil, Detail: r.Detail, Score: r.Score}
			if r.Passed != nil {
				p.Passed = *r.Passed
			}
			res.Runs = append(res.Runs, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// recallShown is how many claims and facts RecallBrain gives.
const recallShown = 10

func (s *Server) RecallBrain(_ context.Context, req *connect.Request[v1.RecallBrainRequest]) (*connect.Response[v1.RecallBrainResponse], error) {
	if req.Msg.GetQuery() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("give a task or question to recall for"))
	}
	res := &v1.RecallBrainResponse{Claims: []*v1.RecalledClaim{}, Facts: []*v1.RecalledFact{}}
	err := s.withBrain(req.Msg.GetId(), req.Msg.GetBrain(), func(br *graph.Brain) error {
		rec, err := br.Recall(req.Msg.GetQuery(), graph.RecallOptions{})
		if err != nil {
			return err
		}
		for _, n := range rec.Nodes {
			if n.Node.NodeType == "claim" && len(res.Claims) < recallShown {
				res.Claims = append(res.Claims, &v1.RecalledClaim{Key: n.Node.Key, Text: n.Node.Title, Status: n.Node.Status, Score: n.Score})
			}
		}
		for i, f := range rec.Facts {
			if i == recallShown {
				break
			}
			res.Facts = append(res.Facts, &v1.RecalledFact{Key: f.Key, Title: f.Title, Loc: f.Loc, Score: f.Score})
		}
		res.Markdown = rec.Markdown()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
