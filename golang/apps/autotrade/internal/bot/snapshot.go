package bot

import (
	"cmp"
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
	LimitOrder(ctx context.Context, symbol, side, qty, price string) (binance.Order, error)
	GetOrder(ctx context.Context, symbol string, orderID int64) (binance.Order, error)
	CancelOrder(ctx context.Context, symbol string, orderID int64) (binance.Order, error)
	BookTicker(ctx context.Context, symbol string) (binance.Book, error)
	CommissionRate(ctx context.Context, symbol string) (binance.Commission, error)
	StopOrder(ctx context.Context, symbol, side, orderType, trigger string) (binance.AlgoOrder, error)
	ReduceStopOrder(ctx context.Context, symbol, side, orderType, trigger, qty string) (binance.AlgoOrder, error)
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

// PositionStops is a venue whose stop-loss and take-profit are part of the
// position (MetaTrader): placing one replaces the old in a single step, so
// protect does not cancel it first.
type PositionStops interface{ StopsOnPosition() }

// AccountCurrency is a venue whose amounts are not in USDT.
type AccountCurrency interface {
	AccountCurrency(ctx context.Context) string
}

// Venue is a trading venue besides Binance.
type Venue struct {
	Name   string // for the snapshot: "MetaTrader 5 demo"
	Market Market
	Ex     Exchange
}

type Bot struct {
	Cfg Config
	// Market and Ex are the venue of Cfg.Symbol: Binance, or MT5 for the
	// mt5 pairs.
	Market  Market
	Ex      Exchange
	Journal Journal
	// MT5 is the venue of the mt5 pairs; nil when MetaTrader is not used.
	MT5 *Venue
	// home is the bot the pair's bot was made from: Binance's clients and
	// the limits of autotrade.yaml. Nil on that bot itself.
	home *Bot
}

// ForPair returns the bot that acts on pair (BTCUSDT or btc/usdt), which
// must be one of the configured pairs. An empty pair means the only one, and
// is refused when there are several, so no action lands on a pair by
// default.
func (b *Bot) ForPair(pair string) (*Bot, error) {
	var sym string
	if PairSymbol(pair) == "" {
		if len(b.Cfg.Pairs) != 1 {
			return nil, fmt.Errorf("name the pair: one of %s", strings.Join(b.Cfg.Pairs, ", "))
		}
		sym = b.Cfg.Pairs[0]
	} else if s, ok := b.Cfg.Resolve(pair); ok {
		sym = s
	} else {
		return nil, fmt.Errorf("%s is not one of the pairs in autotrade.yaml (%s)", PairSymbol(pair), strings.Join(b.Cfg.Pairs, ", "))
	}
	if b.Cfg.OnMT5(sym) && b.MT5 == nil {
		return nil, fmt.Errorf("%s trades on MetaTrader 5, which is not set up", sym)
	}
	return b.on(sym), nil
}

// on returns the bot of sym on its venue: MetaTrader 5 with mt5's leverage
// and limits for an mt5 pair, else Binance with autotrade.yaml's.
func (b *Bot) on(sym string) *Bot {
	home := b
	if b.home != nil {
		home = b.home
	}
	c := *home
	c.home = home
	c.Cfg.Symbol = sym
	if home.MT5 != nil && home.Cfg.OnMT5(sym) {
		m := home.Cfg.MT5
		c.Market, c.Ex = home.MT5.Market, home.MT5.Ex
		c.Cfg.Leverage, c.Cfg.MaxPositionUSDT, c.Cfg.MaxRiskUSDT = m.Leverage, m.MaxPosition, m.MaxRisk
	}
	return &c
}

// onMT5 reports whether the bot's pair trades on MetaTrader 5.
func (b *Bot) onMT5() bool { return b.MT5 != nil && b.Cfg.OnMT5(b.Cfg.Symbol) }

// terms are the words of the pair's venue in messages: the currency of its
// amounts, and the names of its limits in autotrade.yaml.
func (b *Bot) terms(ctx context.Context) (unit, maxPos, maxRisk string) {
	if !b.onMT5() {
		return "USDT", "max_position_usdt", "max_risk_usdt"
	}
	unit = "USD"
	if ac, ok := b.Ex.(AccountCurrency); ok {
		unit = cmp.Or(ac.AccountCurrency(ctx), unit)
	}
	return unit, "mt5.max_position", "mt5.max_risk"
}

// Snapshot is everything one tick decides on.
type Snapshot struct {
	Symbol string `json:"symbol"`
	// Venue is where the pair trades when not on Binance's test network
	// ("MetaTrader 5 demo"), and Currency its account currency (USD).
	Venue     string    `json:"venue,omitempty"`
	Currency  string    `json:"currency,omitempty"`
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
	Book *binance.Book         `json:"book,omitempty"` // best bid and ask on the trading venue, where a limit entry rests

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
	Fees       *binance.Commission `json:"fees,omitempty"`
	// Pending is the limit entry waiting to fill, and its order now.
	Pending      *Entry         `json:"pending,omitempty"`
	PendingOrder *binance.Order `json:"pending_order,omitempty"`
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
	if b.onMT5() {
		s.Venue = b.MT5.Name
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
	if ac, ok := b.Ex.(AccountCurrency); ok {
		s.Currency = ac.AccountCurrency(ctx)
	}

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
	if bk, err := b.Ex.BookTicker(ctx, cfg.Symbol); err != nil {
		s.Problems = append(s.Problems, fmt.Sprintf("order book: %v", err))
	} else {
		s.Book = &bk
	}
	done, err := b.reconcile(ctx)
	if err != nil {
		s.Problems = append(s.Problems, fmt.Sprintf("checking the limit entry: %v", err))
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
	s.Notes = append(done, notes(s)...)
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
		out = append(out, fmt.Sprintf("open %s -setup %s -usdt %.0f -sl %s -tp %s -reason \"...\"  (risk %.2f %s, %s)",
			sig.Side, sig.Setup, usdt, s.price(sig.Stop), s.price(sig.Target), usdt/price*dist, cmp.Or(s.Currency, "USDT"), sig.Why))
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
	if c, err := b.Ex.CommissionRate(ctx, sym); err == nil {
		p.Fees = &c
	}
	if p.Pending, err = b.pending(); err != nil {
		return nil, err
	}
	if p.Pending != nil {
		if o, err := b.Ex.GetOrder(ctx, sym, p.Pending.OrderIDs[0]); err == nil {
			p.PendingOrder = &o
		}
	}
	return p, nil
}

func notes(s *Snapshot) []string {
	var out []string
	if s.Info != nil && s.Info.Status != "" && s.Info.Status != "TRADING" {
		out = append(out, fmt.Sprintf("%s is not trading now (%s): orders will be refused until it is.", s.Symbol, strings.ToLower(strings.ReplaceAll(s.Info.Status, "_", " "))))
	}
	p := s.Portfolio
	if p == nil {
		return out
	}
	if s.Venue != "" && p.Leverage > 0 && p.Leverage != s.Limits.Leverage {
		out = append(out, fmt.Sprintf("The account's leverage is 1:%d and mt5.leverage says %d: opens are refused until they match. The broker sets it, not `open`.", p.Leverage, s.Limits.Leverage))
	}
	if p.Position == nil && p.Pending != nil {
		e := p.Pending
		with := "wait with it as reduce-only orders"
		if s.Venue != "" {
			with = "are on the order and pass to the position when it fills"
		}
		out = append(out, fmt.Sprintf("A limit %s entry is waiting: %g @ %s since %s UTC, cancelled at %s UTC if unfilled; its stop-loss %s and take-profit %s %s. Keep it while the setup holds; `close` cancels it.",
			strings.TrimPrefix(e.Action, "order_"), e.Qty, s.price(e.Limit), e.Time.Format("15:04"), e.Expires.Format("15:04"), s.price(e.StopLoss), s.price(e.TakeProfit), with))
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
	if s.Venue == "" && p.Leverage > 0 && p.Leverage != s.Limits.Leverage {
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
