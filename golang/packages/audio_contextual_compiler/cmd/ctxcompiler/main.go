// Command ctxcompiler is the LLM contextual compiler for music: a prompt spec
// context (spec.md) goes through Claude into a jsongraph context
// (spec.graph.json), and through Claude again into Sonic Pi code (spec.rb).
// Run "ctxcompiler --help" for usage.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	acc "github.com/wargasipil/audio_contextual_compiler"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, nil); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run executes a command line. claude nil means the claude CLI; tests pass a
// fake.
func run(ctx context.Context, argv []string, out, errOut io.Writer, claude acc.Claude) error {
	sonicPiFlags := func() []cli.Flag {
		return []cli.Flag{
			&cli.StringFlag{Name: "sonic-pi", Sources: cli.EnvVars("SONIC_PI_HOME"),
				Usage: "Sonic Pi folder, for checking the code's names and syntax (default: the usual install folder)"},
			&cli.BoolFlag{Name: "no-sonic-pi", Usage: "check the code without an installed Sonic Pi"},
		}
	}
	claudeFlags := func() []cli.Flag {
		return append([]cli.Flag{
			&cli.StringFlag{Name: "model", Value: "opus", Usage: "Claude model for both passes"},
			&cli.DurationFlag{Name: "timeout", Value: 10 * time.Minute, Usage: "time limit of one Claude call"},
			&cli.IntFlag{Name: "retries", Value: 2, Usage: "how many more times to ask when an answer fails the checks"},
			&cli.BoolFlag{Name: "fresh", Usage: "build the graph again even if the spec is unchanged, without the old graph's ids"},
		}, sonicPiFlags()...)
	}

	app := &cli.Command{
		Name:  "ctxcompiler",
		Usage: "LLM contextual compiler: prompt spec context -> Claude -> jsongraph context -> Claude -> Sonic Pi code",
		Description: "A spec.md describes a piece of music as OOP-like pseudo code. Pass 1 (Claude) turns it into\n" +
			"spec.graph.json: nodes song, section, instrument, effect; edges contains, follows, signals, applies.\n" +
			"Pass 2 (Claude) writes spec.rb, Sonic Pi code, from the graph's nested contexts. Both answers are\n" +
			"checked, and a failed one goes back to Claude with its problems. An unchanged spec reuses its graph.\n" +
			"render records spec.rb to spec.wav with Sonic Pi 5, in real time.",
		Writer:    out,
		ErrWriter: errOut,
		// Errors are printed once by main, not also by the framework.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Commands: []*cli.Command{
			{
				Name: "compile", ArgsUsage: "<spec.md | spec.graph.json>",
				Usage: "both passes: write the graph (from a spec) and the Sonic Pi code",
				Flags: append([]cli.Flag{
					&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "Sonic Pi code file (default: <spec>.rb)"},
					&cli.StringFlag{Name: "graph", Usage: "graph file (default: <spec>.graph.json)"},
				}, claudeFlags()...),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return compile(ctx, cmd, claude, out, errOut)
				},
			},
			{
				Name: "graph", ArgsUsage: "<spec.md>",
				Usage: "pass 1 only: write the jsongraph context of a spec",
				Flags: append([]cli.Flag{
					&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "graph file (default: <spec>.graph.json)"},
				}, claudeFlags()...),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					src, err := oneArg(cmd)
					if err != nil {
						return err
					}
					c, err := newCompiler(cmd, claude, errOut)
					if err != nil {
						return err
					}
					_, err = buildGraph(ctx, c, src, orDefault(cmd.String("output"), base(src)+".graph.json"), cmd.Bool("fresh"), out)
					return err
				},
			},
			{
				Name: "check", ArgsUsage: "<spec.graph.json> [code.rb]",
				Usage: "check a graph, and the Sonic Pi code compiled from it, without Claude",
				Flags: sonicPiFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return check(ctx, cmd, out, errOut)
				},
			},
			{
				Name: "render", ArgsUsage: "<code.rb>",
				Usage: "record Sonic Pi code to a WAV with Sonic Pi 5's headless recorder (real time; silent unless --audible)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "WAV file (default: <code>.wav)"},
					&cli.DurationFlag{Name: "duration", Aliases: []string{"d"}, Usage: "how much to record, e.g. 5m or 90s (required)"},
					&cli.BoolFlag{Name: "audible", Usage: "let the music play out loud as it records; by default Sonic Pi's engine is muted in the volume mixer (Windows), which does not change the WAV"},
					sonicPiFlags()[0],
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					src, err := oneArg(cmd)
					if err != nil {
						return err
					}
					sp, err := acc.FindSonicPi(cmd.String("sonic-pi"))
					if err != nil {
						return err
					}
					if sp == nil {
						return fmt.Errorf("Sonic Pi not found (--sonic-pi <folder>)")
					}
					wav := orDefault(cmd.String("output"), base(src)+".wav")
					if err := sp.Record(ctx, src, wav, cmd.Duration("duration"), cmd.Bool("audible"), errOut); err != nil {
						return err
					}
					fmt.Fprintln(out, "wrote", wav)
					return nil
				},
			},
		},
	}
	return app.Run(ctx, append([]string{"ctxcompiler"}, argv...))
}

func compile(ctx context.Context, cmd *cli.Command, claude acc.Claude, out, errOut io.Writer) error {
	src, err := oneArg(cmd)
	if err != nil {
		return err
	}
	c, err := newCompiler(cmd, claude, errOut)
	if err != nil {
		return err
	}
	var g *acc.Graph
	from := filepath.Base(src)
	if isGraph(src) {
		if g, err = acc.LoadGraph(src); err != nil {
			return err
		}
	} else {
		graphPath := orDefault(cmd.String("graph"), base(src)+".graph.json")
		if g, err = buildGraph(ctx, c, src, graphPath, cmd.Bool("fresh"), out); err != nil {
			return err
		}
		from += " (graph: " + filepath.Base(graphPath) + ")"
	}
	fmt.Fprintln(errOut, "pass 2: Claude is writing the Sonic Pi code")
	code, err := c.Code(ctx, g)
	if err != nil {
		return err
	}
	codePath := orDefault(cmd.String("output"), base(src)+".rb")
	text := "# Compiled by ctxcompiler from " + from + ".\n\n" + strings.TrimRight(code, "\n") + "\n"
	if err := os.WriteFile(codePath, []byte(text), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(out, "wrote", codePath)
	return nil
}

// buildGraph runs pass 1 for the spec at specPath and saves the graph at
// graphPath. A graph already there that was built from the same spec is used
// as it is; one built from an earlier spec gives Claude the ids to keep.
func buildGraph(ctx context.Context, c *acc.Compiler, specPath, graphPath string, fresh bool, out io.Writer) (*acc.Graph, error) {
	b, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}
	spec := string(b)
	var prev *acc.Graph
	if !fresh {
		prev, err = acc.LoadGraph(graphPath)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			c.Log(fmt.Sprintf("not using the old graph: %v", err))
		case prev.SpecSHA256 == acc.SpecHash(spec) && len(prev.Check()) == 0:
			c.Log(fmt.Sprintf("%s is unchanged: using %s (--fresh to build it again)", specPath, graphPath))
			return prev, nil
		}
	}
	c.Log(fmt.Sprintf("pass 1: Claude is building the graph of %s", specPath))
	g, err := c.Graph(ctx, spec, prev)
	if err != nil {
		return nil, err
	}
	if err := g.Save(graphPath); err != nil {
		return nil, err
	}
	fmt.Fprintln(out, "wrote", graphPath)
	return g, nil
}

func check(ctx context.Context, cmd *cli.Command, out, errOut io.Writer) error {
	args := cmd.Args().Slice()
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: ctxcompiler check <spec.graph.json> [code.rb]")
	}
	g, err := acc.LoadGraph(args[0])
	if err != nil {
		return err
	}
	problems := g.Check()
	if len(problems) == 0 && len(args) == 2 {
		code, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		sp, err := sonicPi(cmd, errOut)
		if err != nil {
			return err
		}
		problems = acc.CheckCode(ctx, string(code), g, sp)
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(out, "- "+p)
		}
		return fmt.Errorf("%d problem(s)", len(problems))
	}
	fmt.Fprintf(out, "ok: %d nodes, %d edges", len(g.Nodes), len(g.Edges))
	if len(args) == 2 {
		fmt.Fprint(out, "; the code passes the checks")
	}
	fmt.Fprintln(out)
	return nil
}

func newCompiler(cmd *cli.Command, claude acc.Claude, errOut io.Writer) (*acc.Compiler, error) {
	if claude == nil {
		claude = acc.ClaudeCLI{Model: cmd.String("model"), Timeout: cmd.Duration("timeout")}
	}
	sp, err := sonicPi(cmd, errOut)
	if err != nil {
		return nil, err
	}
	return &acc.Compiler{
		Claude:  claude,
		SonicPi: sp,
		Retries: cmd.Int("retries"),
		Log:     func(msg string) { fmt.Fprintln(errOut, msg) },
	}, nil
}

// sonicPi finds the installed Sonic Pi the code is checked against.
func sonicPi(cmd *cli.Command, errOut io.Writer) (*acc.SonicPi, error) {
	if cmd.Bool("no-sonic-pi") {
		return nil, nil
	}
	sp, err := acc.FindSonicPi(cmd.String("sonic-pi"))
	switch {
	case err != nil:
		return nil, err
	case sp == nil:
		fmt.Fprintln(errOut, "Sonic Pi not found: the code's synth, FX and sample names and its syntax are not checked (--sonic-pi <folder>)")
	case sp.Ruby == "":
		fmt.Fprintf(errOut, "checking against Sonic Pi %s; no Ruby found, so the syntax is not checked\n", sp.Version)
	}
	return sp, nil
}

func oneArg(cmd *cli.Command) (string, error) {
	if cmd.Args().Len() != 1 {
		return "", fmt.Errorf("usage: ctxcompiler %s %s", cmd.Name, cmd.ArgsUsage)
	}
	return cmd.Args().First(), nil
}

func isGraph(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".json")
}

// base is a path without its extension, and without ".graph" for a graph.
func base(path string) string {
	b := strings.TrimSuffix(path, filepath.Ext(path))
	if isGraph(path) {
		b = strings.TrimSuffix(b, ".graph")
	}
	return b
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
