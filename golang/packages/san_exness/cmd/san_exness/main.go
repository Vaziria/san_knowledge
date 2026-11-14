// Command san_exness reads an Exness trading account through the Exness
// Trader API: its details, instruments, candles, open positions, history,
// and live streams. It places no orders: trading is in the Go package.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	exness "github.com/wargasipil/san_exness"
)

var version = "dev"

// httpClient is nil (the package default) except in tests.
var httpClient *http.Client

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := app(os.Stdout).Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "san_exness:", err)
		os.Exit(1)
	}
}

func app(out io.Writer) *cli.Command {
	exness.Version = version
	print := func(v any) error {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	return &cli.Command{
		Name:    "san_exness",
		Usage:   "read an Exness trading account through the Exness Trader API (no trading)",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "api-key", Usage: "API key identifier (exnsk-...)", Sources: cli.EnvVars("EXN_API_KEY")},
			&cli.StringFlag{Name: "private-key-file", Usage: "file with the private key (128 hex characters or PKCS#8 PEM); or set EXN_PRIVATE_KEY", Sources: cli.EnvVars("EXN_PRIVATE_KEY_FILE")},
			&cli.StringFlag{Name: "account", Usage: "trading account id", Sources: cli.EnvVars("EXN_ACCOUNT_ID")},
			&cli.StringFlag{Name: "gateway", Usage: "Gateway host from Key Management's DNS widget (https://...)", Sources: cli.EnvVars("EXN_DISCOVERY_BASE_URL")},
			&cli.StringFlag{Name: "access-point", Usage: "the account's Access Point, to skip discovery", Sources: cli.EnvVars("EXN_API_BASE_URL")},
		},
		Commands: []*cli.Command{
			{
				Name:  "discover",
				Usage: "print the account's Access Point",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := client(cmd)
					if err != nil {
						return err
					}
					ap, err := c.AccessPoint(ctx)
					if err != nil {
						return err
					}
					return print(map[string]string{"access_point": ap, "account_id": c.AccountID()})
				},
			},
			{
				Name:  "account",
				Usage: "the account's details and settings",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return run(cmd, print, func(c *exness.Client) (any, error) { return c.Account(ctx) })
				},
			},
			{
				Name:      "instruments",
				Usage:     "list the instruments, or show one's conditions",
				ArgsUsage: "[INSTRUMENT]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if name := cmd.Args().First(); name != "" {
						return run(cmd, print, func(c *exness.Client) (any, error) { return c.Instrument(ctx, name) })
					}
					return run(cmd, print, func(c *exness.Client) (any, error) { return c.Instruments(ctx) })
				},
			},
			{
				Name:  "limits",
				Usage: "the configured REST and WebSocket limits",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return run(cmd, print, func(c *exness.Client) (any, error) { return c.Limits(ctx) })
				},
			},
			{
				Name:  "candles",
				Usage: "candle history, oldest first",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "instrument", Required: true},
					&cli.StringFlag{Name: "timeframe", Value: "M15", Usage: "S1 S5 S15 M1 M3 M5 M10 M15 M30 H1 H2 H4 D1 W1 MN"},
					&cli.StringFlag{Name: "price-type", Usage: "bid (default), ask or both"},
					&cli.StringFlag{Name: "from", Usage: "start, RFC 3339 (2026-11-14T08:00:00Z) or a duration ago (6h)", Value: "24h"},
					&cli.StringFlag{Name: "to", Usage: "end (exclusive), instead of --count"},
					&cli.IntFlag{Name: "count", Usage: "number of candles, -1000 to 1000 (negative: back from --from)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					q := exness.CandleQuery{Instrument: cmd.String("instrument"), Timeframe: exness.Timeframe(strings.ToUpper(cmd.String("timeframe"))),
						PriceType: exness.PriceType(cmd.String("price-type")), Count: int(cmd.Int("count"))}
					var err error
					if q.From, err = when(cmd.String("from")); err != nil {
						return err
					}
					if cmd.String("to") != "" {
						if q.To, err = when(cmd.String("to")); err != nil {
							return err
						}
					} else if q.Count == 0 {
						q.To = time.Now()
					}
					return run(cmd, print, func(c *exness.Client) (any, error) { return c.Candles(ctx, q) })
				},
			},
			{
				Name:  "snapshot",
				Usage: "open orders and positions, and the account's money",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return run(cmd, print, func(c *exness.Client) (any, error) { return c.Snapshot(ctx) })
				},
			},
			history("orders", "closed orders", "state", "filled, cancelled or rejected", print),
			history("deals", "deals", "type", "open, close, balance, swap_fee, ...", print),
			{
				Name:      "operation",
				Usage:     "a trading operation's status",
				ArgsUsage: "OPERATION_ID",
				Flags:     []cli.Flag{&cli.BoolFlag{Name: "wait", Usage: "poll until it is final"}},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if cmd.Bool("wait") {
						return run(cmd, print, func(c *exness.Client) (any, error) { return c.WaitOperation(ctx, id, time.Second) })
					}
					return run(cmd, print, func(c *exness.Client) (any, error) { return c.Operation(ctx, id) })
				},
			},
			{
				Name:  "stream",
				Usage: "print a live stream as JSON lines until Ctrl+C",
				Commands: []*cli.Command{
					{
						Name:      "ticks",
						Usage:     "live bid and ask",
						ArgsUsage: "INSTRUMENT...",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return stream(ctx, cmd, out, false, []exness.Subscription{exness.TicksOf(cmd.Args().Slice()...)})
						},
					},
					{
						Name:  "events",
						Usage: "transactions (default), account state, instrument updates, HMR periods",
						Flags: []cli.Flag{
							&cli.BoolFlag{Name: "transactions", Usage: "trading results and balance operations (default when nothing is chosen)"},
							&cli.BoolFlag{Name: "account-state", Usage: "balance, equity and margin every 2 s"},
							&cli.StringSliceFlag{Name: "instruments", Usage: "instrument updates of these (\"all\" for every one)"},
							&cli.StringSliceFlag{Name: "hmr", Usage: "high margin requirement periods of these instruments"},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							var subs []exness.Subscription
							if cmd.Bool("transactions") {
								subs = append(subs, exness.Transactions())
							}
							if cmd.Bool("account-state") {
								subs = append(subs, exness.AccountStateUpdates())
							}
							if names := cmd.StringSlice("instruments"); len(names) > 0 {
								subs = append(subs, exness.InstrumentUpdates(names...))
							}
							if names := cmd.StringSlice("hmr"); len(names) > 0 {
								subs = append(subs, exness.HMR(names...))
							}
							if len(subs) == 0 {
								subs = append(subs, exness.Transactions())
							}
							return stream(ctx, cmd, out, true, subs)
						},
					},
				},
			},
		},
	}
}

func history(name, usage, filter, filterUsage string, print func(any) error) *cli.Command {
	return &cli.Command{
		Name:  name,
		Usage: "a page of " + usage + ", oldest first",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "from", Value: "168h", Usage: "start, RFC 3339 or a duration ago"},
			&cli.StringFlag{Name: "to", Usage: "end (exclusive)"},
			&cli.StringFlag{Name: "cursor", Usage: "next_cursor of the previous page (goes alone)"},
			&cli.IntFlag{Name: "limit", Usage: "1 to 200 (default 100)"},
			&cli.StringFlag{Name: "instrument"},
			&cli.StringFlag{Name: filter, Usage: filterUsage},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			q := exness.HistoryQuery{Cursor: cmd.String("cursor"), Limit: int(cmd.Int("limit")), Instrument: cmd.String("instrument")}
			if q.Cursor == "" {
				var err error
				if q.From, err = when(cmd.String("from")); err != nil {
					return err
				}
				if cmd.String("to") != "" {
					if q.To, err = when(cmd.String("to")); err != nil {
						return err
					}
				}
			}
			if name == "orders" {
				q.OrderState = cmd.String(filter)
				return run(cmd, print, func(c *exness.Client) (any, error) { return c.Orders(ctx, q) })
			}
			q.DealType = cmd.String(filter)
			return run(cmd, print, func(c *exness.Client) (any, error) { return c.Deals(ctx, q) })
		},
	}
}

// when reads a time: RFC 3339, or a duration ago ("6h", "90m").
func when(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return time.Now().Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("time %q: use RFC 3339 (2026-11-14T08:00:00Z) or a duration ago (6h)", s)
}

// client builds the API client from the flags and the environment. The
// private key comes only from EXN_PRIVATE_KEY or a file: a flag's value
// would show in the process list.
func client(cmd *cli.Command) (*exness.Client, error) {
	key := os.Getenv("EXN_PRIVATE_KEY")
	if path := cmd.String("private-key-file"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("private key file: %w", err)
		}
		key = string(b)
	}
	if key == "" {
		return nil, errors.New("no private key: set EXN_PRIVATE_KEY or --private-key-file")
	}
	return exness.New(exness.Config{
		APIKey:      cmd.String("api-key"),
		PrivateKey:  key,
		AccountID:   cmd.String("account"),
		Gateway:     cmd.String("gateway"),
		AccessPoint: cmd.String("access-point"),
		HTTPClient:  httpClient,
	})
}

func run(cmd *cli.Command, print func(any) error, call func(*exness.Client) (any, error)) error {
	c, err := client(cmd)
	if err != nil {
		return err
	}
	v, err := call(c)
	if err != nil {
		return err
	}
	return print(v)
}

func stream(ctx context.Context, cmd *cli.Command, out io.Writer, events bool, subs []exness.Subscription) error {
	for _, sub := range subs {
		if sub.Event != "transactions" && sub.Event != "account_state" && len(sub.Instruments) == 0 {
			return fmt.Errorf("%s: name at least one instrument", sub.Event)
		}
	}
	c, err := client(cmd)
	if err != nil {
		return err
	}
	var s *exness.Stream
	if events {
		s, err = c.Events(ctx)
	} else {
		s, err = c.Ticks(ctx)
	}
	if err != nil {
		return err
	}
	defer s.Close()
	for i, sub := range subs {
		if err := s.Subscribe(ctx, fmt.Sprintf("san-%d", i+1), sub); err != nil {
			return err
		}
	}
	for {
		m, err := s.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Ctrl+C
			}
			return err
		}
		if _, err := fmt.Fprintf(out, "%s\n", m.Raw); err != nil {
			return err
		}
	}
}
