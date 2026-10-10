package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/wargasipil/autotrade/internal/bot"
	"github.com/wargasipil/autotrade/internal/mt5"
)

// checkMT5 shows the MetaTrader account the EA is logged in to and each mt5
// pair, and fails on what would refuse an open.
func checkMT5(ctx context.Context, w io.Writer, cfg bot.Config, c *mt5.Client) error {
	fmt.Fprintf(w, "\nmt5      %s, bridge %s\n", strings.Join(cfg.MT5.Pairs, ", "), c.Dir())
	t, err := c.Ping(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "account  %d on %s (%s): %s, %s\n", t.Login, t.Server, t.Company, t.TradeMode, t.MarginMode)
	fmt.Fprintf(w, "balance  %.2f %s (equity %.2f, free margin %.2f)\n", t.Balance, t.Currency, t.Equity, t.MarginFree)
	fmt.Fprintf(w, "leverage 1:%d on the account, %d in mt5.leverage\n", t.Leverage, cfg.MT5.Leverage)
	fmt.Fprintf(w, "server   UTC%+g, terminal build %d, bridge %s\n", float64(t.GMTOffset)/3600, t.Build, t.Bridge)
	var problems []error
	switch {
	case t.TradeMode != "demo":
		problems = append(problems, fmt.Errorf("account %d is a %s account: autotrade trades on demo accounts only", t.Login, t.TradeMode))
	case !t.Connected:
		problems = append(problems, errors.New("the terminal is not connected to the trade server"))
	case t.TradeRefusal != "":
		problems = append(problems, errors.New(t.TradeRefusal))
	}
	if t.Leverage != cfg.MT5.Leverage {
		problems = append(problems, fmt.Errorf("set mt5.leverage in autotrade.yaml to the account's %d", t.Leverage))
	}
	for _, sym := range cfg.MT5.Pairs {
		info, err := c.SymbolInfo(ctx, sym)
		if err != nil {
			fmt.Fprintf(w, "pair     %s: %v\n", sym, err)
			problems = append(problems, err)
			continue
		}
		smallest := ""
		if pi, err := c.PremiumIndex(ctx, sym); err == nil {
			v := info.MinQty * pi.MarkPrice
			smallest = fmt.Sprintf(" (%.0f %s now)", v, t.Currency)
			if v > cfg.MT5.MaxPosition {
				problems = append(problems, fmt.Errorf("the smallest %s position is worth %.0f %s, over mt5.max_position %.0f", sym, v, t.Currency, cfg.MT5.MaxPosition))
			}
		}
		fmt.Fprintf(w, "pair     %s %s, smallest size %g%s in steps of %g, price tick %g\n",
			sym, strings.ToLower(strings.ReplaceAll(info.Status, "_", " ")), info.MinQty, smallest, info.StepSize, info.TickSize)
	}
	return errors.Join(problems...)
}

// installMT5 copies the AutotradeBridge EA into the MQL5/Experts folder of
// each MetaTrader 5 terminal of this Windows user, and compiles it with the
// terminal's MetaEditor when it finds one.
func installMT5(w io.Writer, bridge string) error {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return errors.New("APPDATA is not set: copy internal/mt5/AutotradeBridge.mq5 into the terminal's MQL5/Experts folder by hand")
	}
	root := filepath.Join(appdata, "MetaQuotes", "Terminal")
	dirs, _ := filepath.Glob(filepath.Join(root, "*", "MQL5", "Experts"))
	if len(dirs) == 0 {
		return fmt.Errorf("no MetaTrader 5 terminal found under %s: install MT5 (MIFX's download), start it once, then run this again", root)
	}
	for _, experts := range dirs {
		src := filepath.Join(experts, "AutotradeBridge.mq5")
		if err := os.WriteFile(src, mt5.Source, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(w, "copied   %s\n", src)
		install := origin(filepath.Dir(filepath.Dir(experts)))
		editor := filepath.Join(install, "metaeditor64.exe")
		if _, err := os.Stat(editor); install == "" || err != nil {
			fmt.Fprintf(w, "compile  open it in MetaEditor and press F7 (no metaeditor64.exe found for this terminal)\n")
			continue
		}
		fmt.Fprintf(w, "compile  %s\n", compileEA(editor, src))
	}
	fmt.Fprintf(w, `
Then, in the MT5 terminal logged in to the MIFX demo account:
  1. Navigator > Expert Advisors > AutotradeBridge: drag it onto any chart.
  2. In its settings, Common tab: tick "Allow Algo Trading".
  3. Turn on "Algo Trading" in the toolbar. The Experts tab shows "autotrade bridge ... answering requests".
  4. Run: autotrade check
The EA answers in %s.
`, bridge)
	return nil
}

// origin reads the terminal's install folder from origin.txt in its data
// folder (UTF-16).
func origin(dataDir string) string {
	b, err := os.ReadFile(filepath.Join(dataDir, "origin.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(utf16Text(b))
}

func utf16Text(b []byte) string {
	if len(b) < 2 || !(b[0] == 0xFF && b[1] == 0xFE) {
		return string(b)
	}
	b = b[2:]
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(u))
}

// compileEA runs MetaEditor on src and says how it went: the .ex5 is new,
// or the end of MetaEditor's log.
func compileEA(editor, src string) string {
	start := time.Now()
	ex5 := strings.TrimSuffix(src, ".mq5") + ".ex5"
	cmd := compileCmd(editor, src)
	if cmd == nil {
		return "open it in MetaEditor and press F7"
	}
	_ = cmd.Run() // MetaEditor's exit code is not its result; the .ex5 is
	if st, err := os.Stat(ex5); err == nil && !st.ModTime().Before(start.Add(-time.Second)) {
		return "ok, " + ex5
	}
	log, _ := os.ReadFile(strings.TrimSuffix(src, ".mq5") + ".log")
	lines := strings.Split(strings.TrimSpace(utf16Text(bytes.TrimSpace(log))), "\n")
	return fmt.Sprintf("failed (%s): open it in MetaEditor and press F7 to see why", strings.TrimSpace(lines[len(lines)-1]))
}
