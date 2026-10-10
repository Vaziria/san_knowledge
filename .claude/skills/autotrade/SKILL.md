---
name: autotrade
description: Run one autotrade tick on the Binance test network in three phases. Analyze each pair in autotrade.yaml, carry out exactly one action per pair (open, protect, close or hold), then update the autotrade knowledge (sync, and the lessons of closed positions) with the autotrade MCP tools. Use when the user types /autotrade, or from /loop.
disable-model-invocation: true
argument-hint: "[note for this tick]"
allowed-tools: mcp__autotrade__autotrade_snapshot, mcp__autotrade__autotrade_open, mcp__autotrade__autotrade_protect, mcp__autotrade__autotrade_close, mcp__autotrade__autotrade_hold, mcp__autotrade__autotrade_note, mcp__autotrade__autotrade_journal, mcp__autotrade__autotrade_review, mcp__autotrade__autotrade_approaches, mcp__autotrade__autotrade_approach, mcp__autotrade__autotrade_approach_define, mcp__autotrade__autotrade_positions, mcp__autotrade__autotrade_position, mcp__autotrade__autotrade_pair, mcp__autotrade__autotrade_portfolio, mcp__autotrade__autotrade_query, mcp__autotrade__autotrade_knowledge_sync
---

Run one tick of the autotrade loop.

1. Read `golang/apps/autotrade/tick.md` in full. It is the procedure and it changes, so don't work from memory.
2. Follow its three phases in order (docs/readme.md):
   - **Doing Analyze:** the snapshot, then for each pair the written view and the knowledge check.
   - **Doing Action:** one action per pair.
   - **Updating Autotrade Knowledge:** sync the graph, write the lesson of each closed position it lists, and update an approach a lesson changed.
3. Reply with one line per pair saying the action and why, and one line on the knowledge.

Use the `autotrade` MCP tools. If they are not connected, tell the user, then use the CLI as tick.md describes.

The user's note for this tick, if any: $ARGUMENTS
