package bot

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/feature"
	"github.com/wargasipil/autotrade/internal/strategy"
)

// Market is where candles come from.
type Market interface {
	SyncTime(ctx context.Context) error
	Now() time.Time
	Klines(ctx context.Context, symbol, interval string, limit int) ([]binance.Candle, error)
}

// Exchange is the trading venue. *binance.Client implements it.
type Exchange interface {
	HasKey() bool
	SyncTime(ctx context.Context) error
	Now() time.Time
	SymbolInfo(ctx context.Context, symbol string) (binance.SymbolInfo, error)
	PremiumIndex(ctx context.Context, symbol string) (binance.PremiumIndex, error)
	Account(ctx context.Context) (binance.Account, error)
	Positions(ctx context.Context, symbol string) ([]binance.Position, error)
	SymbolConfig(ctx context.Context, symbol string) (binance.SymbolConfig, error)
	DualSide(ctx context.Context) (bool, error)
	SetLeverage(ctx context.Context, symbol string, leverage int) error
	Income(ctx context.Context, symbol string, start, end time.Time) ([]binance.Income, error)
	MarketOrder(ctx context.Context, symbol, side, qty string, reduceOnly bool) (binance.Order, error)
	StopOrder(ctx context.Context, symbol, side, orderType, trigger string) (binance.AlgoOrder, error)
	OpenAlgoOrders(ctx context.Context, symbol string) ([]binance.AlgoOrder, error)
	CancelAlgoOrder(ctx context.Context, symbol string, algoID int64) error
	CancelAlgoOrders(ctx context.Context, symbol string) error
}

// Positioner serves the futures positioning data (mainnet only). The
// snapshot shows it when the market client has it.
type Positioner interface {
	OpenInterestHist(ctx context.Context, symbol, period string, limit int) ([]binance.OIPoint, error)
	TakerRatio(ctx context.Context, symbol, period string, limit int) ([]binance.Ratio, error)
	LongShortRatio(ctx context.Context, kind, symbol, period string, limit int) ([]binance.Ratio, error)
	FundingHistory(ctx context.Context, symbol string, limit int) ([]binance.Funding, error)
}

type Bot struct {
	Cfg     Config
	Market  Market
	Ex      Exchange
	Journal Journal
}

// ForPair returns the bot that acts on pair (BTCUSDT or btc/usdt), which
// must be one of the configured pairs. An empty pair means the only one, and
// is refused when there are several, so no action lands on a pair by
// default.
func (b *Bot) ForPair(pair string) (*Bot, error) {
	sym := PairSymbol(pair)
	if sym == "" {
		if len(b.Cfg.Pairs) != 1 {
			return nil, fmt.Errorf("name the pair: one of %s", strings.Join(b.Cfg.Pairs, ", "))
		}
		sym = b.Cfg.Pairs[0]
	}
	if !b.Cfg.Trades(sym) {
		return nil, fmt.Errorf("%s is not one of the pairs in autotrade.yaml (%s)", sym, strings.Join(b.Cfg.Pairs, ", "))
	}
	c := *b
	c.Cfg.Symbol = sym
	return &c, nil
}

// Snapshot is everything one tick decides on.
type Snapshot struct {
	Symbol    string    `json:"symbol"`
	Now       time.Time `json:"now"`
	NextClose time.Time `json:"next_close"` // when the forming candle closes

	Frames  []feature.Frame  `json:"frames"`  // the trading interval first, then the context intervals
	Recent  []binance.Candle `json:"recent"`  // last closed candles of the trading interval
	Forming *binance.Candle  `json:"forming"` // the candle still open

	// Price is the latest price (the forming candle's close), which the
	// levels are measured from.
	Price       float64         `json:"price"`
	LevelsAbove []feature.Level `json:"levels_above"` // nearest first
	LevelsBelow []feature.Level `json:"levels_below"`
	Positioning *Positioning    `json:"positioning,omitempty"`

	Info *binance.SymbolInfo   `json:"symbol_info,omitempty"`
	Mark *binance.PremiumIndex `json:"mark,omitempty"` // on the trading venue

	Portfolio    *Portfolio `json:"portfolio,omitempty"`
	PortfolioErr string     `json:"portfolio_error,omitempty"`

	// Signals are the coded setups (internal/strategy) at the last close;
	// Suggestions are ready-to-run open commands for the triggered ones.
	Signals     *strategy.Result `json:"signals,omitempty"`
	Suggestions []string         `json:"suggestions,omitempty"`
	// Plan applies the management rules to the open position.
	Plan *Plan `json:"plan,omitempty"`

	Limits    Limits   `json:"limits"`
	Journal   []Entry  `json:"journal"`              // last decisions, oldest first
	LastTrade *Entry   `json:"last_trade,omitempty"` // newest open or close in the whole journal
	Notes     []string `json:"notes"`                // things the decision should not miss
	Problems  []string `json:"problems,omitempty"`
}

type Portfolio struct {
	Account    binance.Account     `json:"account"`
	Position   *binance.Position   `json:"position"` // nil when flat
	Leverage   int                 `json:"leverage"`
	Orders     []binance.AlgoOrder `json:"orders"`     // stop-loss / take-profit
	Income24h  map[string]float64  `json:"income_24h"` // by type: REALIZED_PNL, COMMISSION, FUNDING_FEE
	TradeCount int                 `json:"trades_24h"` // REALIZED_PNL entries
}

// Positioning is how futures traders are positioned (all ratios hourly).
type Positioning struct {
	OpenInterest   float64           `json:"open_interest"`     // contracts
	OIChange4h     float64           `json:"oi_change_4h"`      // percent
	OIChange24h    float64           `json:"oi_change_24h"`     // percent
	TakerBuySell1h float64           `json:"taker_buy_sell_1h"` // taker buy over sell volume, last hour
	TakerBuySell24 float64           `json:"taker_buy_sell_24h"`
	AccountsLS     float64           `json:"accounts_long_short"` // all accounts, long over short
	AccountsLS24h  float64           `json:"accounts_long_short_24h_ago"`
	TopLS          float64           `json:"top_long_short"` // top traders' positions, long over short
	TopLS24h       float64           `json:"top_long_short_24h_ago"`
	Funding        []binance.Funding `json:"funding"` // last settled rates, oldest first
}

// Plan is the open position measured against its journal entry.
type Plan struct {
	Trade   strategy.Trade  `json:"trade"`
	Target  float64         `json:"target,omitempty"`
	Advice  strategy.Advice `json:"advice"`
	Command string          `json:"command,omitempty"` // for protect and close
}

type Limits struct {
	Leverage        int     `json:"leverage"`
	MaxPositionUSDT float64 `json:"max_position_usdt"`
	MaxRiskUSDT     float64 `json:"max_risk_usdt"`
}

// Snapshot gathers market, features, portfolio and journal. Problems with
// one part are reported in the snapshot rather than failing the whole of it,
// except for the trading interval's candles.
func (b *Bot) Snapshot(ctx context.Context, recent int) (*Snapshot, error) {
	cfg := b.Cfg
	if err := b.Market.SyncTime(ctx); err != nil {
		return nil, err
	}
	now := b.Market.Now().UTC()
	s := &Snapshot{
		Symbol: cfg.Symbol,
		Now:    now,
		Limits: Limits{Leverage: cfg.Leverage, MaxPositionUSDT: cfg.MaxPositionUSDT, MaxRiskUSDT: cfg.MaxRiskUSDT},
	}

	var base []binance.Candle // closed candles of the trading interval
	ivs := append([]string{cfg.Interval}, cfg.ContextIntervals...)
	closedBy := make([][]binance.Candle, len(ivs))
	for i, iv := range ivs {
		all, err := b.Market.Klines(ctx, cfg.Symbol, iv, cfg.Candles+1)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			s.Problems = append(s.Problems, fmt.Sprintf("%s candles: %v", iv, err))
			continue
		}
		closed := feature.Closed(all, now)
		f, err := feature.Extract(iv, closed)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			s.Problems = append(s.Problems, err.Error())
			continue
		}
		s.Frames = append(s.Frames, f)
		closedBy[i] = closed
		if i == 0 {
			if len(closed) < len(all) {
				forming := all[len(closed)]
				s.Forming = &forming
				s.NextClose = forming.CloseTime.Add(time.Millisecond)
			}
			s.Recent = closed[max(0, len(closed)-recent):]
			base = closed
		}
	}
	s.Price = base[len(base)-1].Close
	if s.Forming != nil {
		s.Price = s.Forming.Close
	}
	s.LevelsAbove, s.LevelsBelow = levels(ivs, closedBy, s.Price)

	if pm, ok := b.Market.(Positioner); ok {
		if p, err := positioning(ctx, pm, cfg.Symbol); err != nil {
			s.Problems = append(s.Problems, fmt.Sprintf("positioning: %v", err))
		} else {
			s.Positioning = p
		}
	}

	if info, err := b.Ex.SymbolInfo(ctx, cfg.Symbol); err != nil {
		s.Problems = append(s.Problems, fmt.Sprintf("symbol info: %v", err))
	} else {
		s.Info = &info
	}
	if pi, err := b.Ex.PremiumIndex(ctx, cfg.Symbol); err != nil {
		s.Problems = append(s.Problems, fmt.Sprintf("mark price: %v", err))
	} else {
		s.Mark = &pi
	}

	if !b.Ex.HasKey() {
		s.PortfolioErr = "no API key: set BINANCE_TESTNET_API_KEY and BINANCE_TESTNET_API_SECRET"
	} else if p, err := b.portfolio(ctx, now); err != nil {
		s.PortfolioErr = err.Error()
	} else {
		s.Portfolio = p
	}

	all, err := b.Journal.Last(math.MaxInt)
	if err != nil {
		s.Problems = append(s.Problems, fmt.Sprintf("journal: %v", err))
	}
	all = slices.DeleteFunc(all, func(e Entry) bool { return e.Symbol != "" && e.Symbol != cfg.Symbol }) // this pair's decisions
	s.Journal = all[max(0, len(all)-5):]
	s.LastTrade = lastTrade(all)

	if len(cfg.Setups) > 0 && len(s.Frames) == 3 {
		res := strategy.Evaluate(base, s.Frames[1], s.Frames[2], cfg.Setups)
		s.Signals = &res
		if s.Portfolio == nil || s.Portfolio.Position == nil {
			s.Suggestions = b.suggest(s, res.Signals)
		}
	}
	if s.Portfolio != nil && s.Portfolio.Position != nil && len(s.Frames) >= 2 {
		s.Plan = b.plan(s, base)
	}
	s.Notes = notes(s)
	if err := b.saveLast(s); err != nil {
		s.Problems = append(s.Problems, fmt.Sprintf("saving the snapshot: %v", err))
	}
	return s, nil
}

// suggest turns triggered signals into open commands sized by the limits.
func (b *Bot) suggest(s *Snapshot, sigs []strategy.Signal) []string {
	var out []string
	for _, sig := range sigs {
		price := sig.Entry
		if s.Mark != nil {
			price = s.Mark.MarkPrice
		}
		dir := 1.0
		if sig.Side == "short" {
			dir = -1
		}
		dist := dir * (price - sig.Stop)
		if dist <= 0 || dir*(sig.Target-price) <= 0 {
			out = append(out, fmt.Sprintf("%s %s: the price %s has already moved past a level; skip it", sig.Setup, sig.Side, s.price(price)))
			continue
		}
		usdt := b.Cfg.MaxPositionUSDT
		if b.Cfg.MaxRiskUSDT > 0 {
			usdt = math.Min(usdt, b.Cfg.MaxRiskUSDT/dist*price)
		}
		usdt = math.Floor(usdt)
		out = append(out, fmt.Sprintf("open %s -setup %s -usdt %.0f -sl %s -tp %s -reason \"...\"  (risk %.2f USDT, %s)",
			sig.Side, sig.Setup, usdt, s.price(sig.Stop), s.price(sig.Target), usdt/price*dist, sig.Why))
	}
	return out
}

// plan reads the open position's journal entry and applies Manage.
func (b *Bot) plan(s *Snapshot, base []binance.Candle) *Plan {
	pos := s.Portfolio.Position
	e := s.LastTrade
	if e == nil || e.Action == "close" || (e.Action == "open_long") != (pos.Amount > 0) || e.StopLoss == 0 {
		return &Plan{Advice: strategy.Advice{Action: "hold", Why: "the position has no matching open in the journal, so it has no plan; manage it by hand"}}
	}
	side := "long"
	if pos.Amount < 0 {
		side = "short"
	}
	step, _ := feature.IntervalDuration(b.Cfg.Interval)
	t := strategy.Trade{Setup: e.Setup, Side: side, Entry: pos.EntryPrice, InitialStop: e.StopLoss, Stop: e.StopLoss,
		Opened: e.Time.Truncate(step)}
	for _, o := range s.Portfolio.Orders {
		if o.Type == binance.StopMarket {
			t.Stop = o.TriggerPrice
		}
	}
	p := &Plan{Trade: t, Target: e.TakeProfit, Advice: strategy.Manage(t, base, s.Frames[1], b.Cfg.FeeRate)}
	switch p.Advice.Action {
	case "protect":
		p.Command = fmt.Sprintf("protect -sl %s -reason \"...\"", s.price(p.Advice.Stop))
	case "close":
		p.Command = "close -reason \"...\""
	}
	return p
}

func (s *Snapshot) price(v float64) string {
	if s.Info != nil {
		return s.Info.FormatPrice(s.Info.RoundPrice(v))
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func (b *Bot) portfolio(ctx context.Context, now time.Time) (*Portfolio, error) {
	sym := b.Cfg.Symbol
	acc, err := b.Ex.Account(ctx)
	if err != nil {
		return nil, err
	}
	p := &Portfolio{Account: acc, Income24h: map[string]float64{}}
	pos, err := b.Ex.Positions(ctx, sym)
	if err != nil {
		return nil, err
	}
	if len(pos) > 0 {
		p.Position = &pos[0]
	}
	sc, err := b.Ex.SymbolConfig(ctx, sym)
	if err != nil {
		return nil, err
	}
	p.Leverage = sc.Leverage
	if p.Orders, err = b.Ex.OpenAlgoOrders(ctx, sym); err != nil {
		return nil, err
	}
	inc, err := b.Ex.Income(ctx, sym, now.Add(-24*time.Hour), time.Time{})
	if err != nil {
		return nil, err
	}
	for _, in := range inc {
		p.Income24h[in.Type] += in.Amount
		if in.Type == "REALIZED_PNL" {
			p.TradeCount++
		}
	}
	return p, nil
}

func notes(s *Snapshot) []string {
	var out []string
	p := s.Portfolio
	if p == nil {
		return out
	}
	if p.Position == nil {
		if len(p.Orders) > 0 {
			out = append(out, fmt.Sprintf("%d stop/take-profit orders are left over from a closed position; `open` and `close` cancel them.", len(p.Orders)))
		}
		if last := s.LastTrade; last != nil && last.Action != "close" {
			out = append(out, fmt.Sprintf("The position opened at %s is gone without a `close`: its stop-loss or take-profit was hit (see realized PnL).", last.Time.Format("01-02 15:04")))
		}
		return out
	}
	if !hasStop(p.Orders) {
		out = append(out, "The open position has NO stop-loss. Set one with `protect --sl`.")
	}
	if p.Leverage > 0 && p.Leverage != s.Limits.Leverage {
		out = append(out, fmt.Sprintf("Leverage on the exchange is %dx, the config says %dx; `open` sets it.", p.Leverage, s.Limits.Leverage))
	}
	return out
}

func hasStop(orders []binance.AlgoOrder) bool {
	for _, o := range orders {
		if o.Type == binance.StopMarket {
			return true
		}
	}
	return false
}

// lastTrade is the newest successful entry that opened or closed a position.
func lastTrade(entries []Entry) *Entry {
	for i := len(entries) - 1; i >= 0; i-- {
		switch entries[i].Action {
		case "open_long", "open_short", "close":
			if entries[i].Error == "" {
				return &entries[i]
			}
		}
	}
	return nil
}

// levels finds the swing highs and lows of the trading interval (last week
// of candles) and the first context interval (last 180 candles), plus the
// high and low of the last closed candle of the second context interval
// (the prior day, for 1d).
func levels(ivs []string, closedBy [][]binance.Candle, price float64) (above, below []feature.Level) {
	var all []feature.Level
	for i := len(ivs) - 1; i >= 0; i-- {
		cs := closedBy[i]
		if len(cs) == 0 {
			continue
		}
		if i == 2 {
			last := cs[len(cs)-1]
			all = append(all,
				feature.Level{Price: last.High, Kind: "prior " + ivs[i] + " high", Interval: ivs[i], Time: last.OpenTime},
				feature.Level{Price: last.Low, Kind: "prior " + ivs[i] + " low", Interval: ivs[i], Time: last.OpenTime})
			continue
		}
		n := 180
		if i == 0 {
			if step, err := feature.IntervalDuration(ivs[0]); err == nil && step < 24*time.Hour {
				n = int(7 * 24 * time.Hour / step)
			}
		}
		all = append(all, feature.Pivots(ivs[i], cs[max(0, len(cs)-n):], 3)...)
	}
	return feature.Nearest(all, price, 4, 0.0015)
}

func positioning(ctx context.Context, m Positioner, symbol string) (*Positioning, error) {
	p := &Positioning{}
	oi, err := m.OpenInterestHist(ctx, symbol, "1h", 25)
	if err != nil {
		return nil, err
	}
	if n := len(oi); n > 0 {
		last := oi[n-1].OI
		p.OpenInterest = last
		if n >= 5 && oi[n-5].OI > 0 {
			p.OIChange4h = (last/oi[n-5].OI - 1) * 100
		}
		if oi[0].OI > 0 {
			p.OIChange24h = (last/oi[0].OI - 1) * 100
		}
	}
	taker, err := m.TakerRatio(ctx, symbol, "1h", 24)
	if err != nil {
		return nil, err
	}
	if n := len(taker); n > 0 {
		p.TakerBuySell1h = taker[n-1].Ratio
		var buy, sell float64
		for _, t := range taker {
			buy, sell = buy+t.Buy, sell+t.Sell
		}
		if sell > 0 {
			p.TakerBuySell24 = buy / sell
		}
	}
	for _, r := range []struct {
		kind      string
		now, then *float64
	}{{binance.AllAccounts, &p.AccountsLS, &p.AccountsLS24h}, {binance.TopPositions, &p.TopLS, &p.TopLS24h}} {
		rs, err := m.LongShortRatio(ctx, r.kind, symbol, "1h", 25)
		if err != nil {
			return nil, err
		}
		if n := len(rs); n > 0 {
			*r.now, *r.then = rs[n-1].Ratio, rs[0].Ratio
		}
	}
	if p.Funding, err = m.FundingHistory(ctx, symbol, 3); err != nil {
		return nil, err
	}
	return p, nil
}
