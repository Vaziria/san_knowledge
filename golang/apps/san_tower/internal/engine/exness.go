package engine

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	exness "github.com/wargasipil/san_exness"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// The Exness steps (the user, 2026-11-14: "Can we have exness ?", then a
// Finance connection like Binance with the basic steps: Order, Candles,
// Portfolio, Orders, Positions, Close Position and Move Stop), on an
// Exness connection through the Exness Trader API
// (golang/packages/san_exness). Exness has no test network, so every order
// is real money: the steps that trade refuse unless the connection's Allow
// Trading is ticked, and Exness Order has limits of its own (the user,
// 2026-11-14: "A", both). Every line a step logs names the account.

// exnessWait is how long a trading step waits for Exness to confirm an
// operation, and exnessPoll how often it asks.
const (
	exnessWait = 30 * time.Second
	exnessPoll = 500 * time.Millisecond
)

// ExnessClients are the Exness connections' clients, kept from run to run:
// each keeps its account's Access Point, which the Gateway gives only a few
// times a minute, and its offset from Exness's clock. A connection's
// settings changing gives a new client. TestConnection shares them with
// the steps.
type ExnessClients struct {
	mu      sync.Mutex
	http    *http.Client
	clients map[[32]byte]*exness.Client
}

// NewExnessClients makes clients that send through h (nil: san_exness's
// default, a 15 s timeout).
func NewExnessClients(h *http.Client) *ExnessClients {
	return &ExnessClients{http: h, clients: map[[32]byte]*exness.Client{}}
}

// UseHTTPClient sends through h from now on (a fake's, in tests).
func (x *ExnessClients) UseHTTPClient(h *http.Client) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.http, x.clients = h, map[[32]byte]*exness.Client{}
}

// Client is the client of a connection's settings. A nil x makes a new one
// each time.
func (x *ExnessClients) Client(s *v1.ExnessSettings) (*exness.Client, error) {
	cfg := exness.Config{APIKey: s.GetApiKey(), PrivateKey: s.GetPrivateKey(), AccountID: s.GetAccountId(), Gateway: s.GetGateway(), AccessPoint: s.GetAccessPoint()}
	if x == nil {
		return exness.New(cfg)
	}
	key := sha256.Sum256([]byte(strings.Join([]string{cfg.APIKey, cfg.PrivateKey, cfg.AccountID, cfg.Gateway, cfg.AccessPoint}, "\x00")))
	x.mu.Lock()
	defer x.mu.Unlock()
	if c := x.clients[key]; c != nil {
		return c, nil
	}
	cfg.HTTPClient = x.http
	c, err := exness.New(cfg)
	if err != nil {
		return nil, err
	}
	x.clients[key] = c
	return c, nil
}

// exnessErr is an error of the API as a step says it: the account first.
func exnessErr(label string, err error) error {
	var oe *exness.OperationError
	if errors.As(err, &oe) {
		return fmt.Errorf("%s: refused: %s (%d)", label, oe.Message, oe.Code)
	}
	return fmt.Errorf("%s: %s", label, strings.TrimPrefix(err.Error(), "exness: "))
}

var exnessKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "exness-order", Title: "Exness Order", Category: "Finance", Group: "Exness",
			Description: "Places an order on an Exness connection, with real money (Exness has no test network): Market opens a position now, Limit and Stop wait at Price; Buy or Sell Volume lots of Instrument (its name as the account shows it, such as XAUUSD or EURUSDm; the volume rounded down to its step), with an optional Stop Loss and Take Profit. It refuses unless the connection's Allow Trading is ticked. It waits until Exness confirms the order (30 s at most), and fails with Exness's reason when refused. Order, when linked, gives all of it as one object (instrument, side buy or sell, type market, limit or stop, volume, price, stop_loss, take_profit), as an AI's order through a Workflow Tool: what it gives wins over the settings. Limits, checked before anything is sent, at the price the order gets (the current ask for a market buy, the bid for a sell): Require Stop, Max Volume (lots) and Max Loss (USD lost at the stop; it needs a stop, and an instrument quoted in USD or based on it, such as XAUUSD or USDJPY); a refusal says the most that fits. Order is what was sent, with its operation's status.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Order", Object), sock("Operation ID", Text)},
			Settings: []*v1.Setting{exnessSetting(),
				{Name: "Type", Options: []string{"Market", "Limit", "Stop"}},
				{Name: "Side", Options: []string{"Buy", "Sell"}},
				{Name: "Require Stop", Options: []string{"No", "Yes"}},
				{Name: "Max Volume", Placeholder: "No limit: lots"},
				{Name: "Max Loss", Placeholder: "No limit: USD lost at the stop"}},
			Inputs: []*v1.Socket{sock("Event", Event), hint("XAUUSD", field("Instrument")), hint("0.01", field("Volume")), only("Type!=Market", field("Price")),
				field("Stop Loss"), field("Take Profit"), sock("Order", Object)},
		},
		run: exnessOrder,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "exness-candles", Title: "Exness Candles", Category: "Finance", Group: "Exness",
			Description: "Reads Instrument's latest candles on an Exness connection: Limit of them (100; up to 1000) at Timeframe, oldest first, the last one usually still forming. Price is Bid (as Exness's charts show), Ask, or Both (each candle with its bid and ask). Last Closed is the newest that has closed, Close its close price (the bid's for Both).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Candles", Object), sock("Last Closed", Object), sock("Close", Text)},
			Settings: []*v1.Setting{exnessSetting(),
				{Name: "Timeframe", Options: exnessTimeframes},
				{Name: "Price", Options: []string{"Bid", "Ask", "Both"}},
				{Name: "Limit", Placeholder: "100"}},
			Inputs: []*v1.Socket{sock("Event", Event), hint("XAUUSD", field("Instrument"))},
		},
		run: exnessCandles,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "exness-portfolio", Title: "Exness Portfolio", Category: "Finance", Group: "Exness",
			Description: "Reads an Exness connection's account: Account its currency, leverage, balance, equity, used and free margin, and whether the connection may trade (allow_trading); Positions its open positions, Orders its orders waiting (limit, stop, and the positions' SL/TP orders).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Account", Object), sock("Positions", Object), sock("Orders", Object)},
			Settings:    []*v1.Setting{exnessSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event)},
		},
		run: exnessPortfolio,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "exness-orders", Title: "Exness Orders", Category: "Finance", Group: "Exness",
			Description: "Lists an Exness connection's orders, newest first, of Instrument or of every instrument: Show Open, the orders still waiting (limit, stop, SL/TP); History, the orders filled, cancelled or rejected in the last Days (7); Deals, the deals of the last Days (opens, closes, balance changes) in Deals. Limit of them (50; up to 1000). Last is the newest.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Orders", Object), sock("Deals", Object), sock("Last", Object)},
			Settings: []*v1.Setting{exnessSetting(),
				{Name: "Show", Options: []string{"Open", "History", "Deals"}},
				{Name: "Days", Placeholder: "7"},
				{Name: "Limit", Placeholder: "50"}},
			Inputs: []*v1.Socket{sock("Event", Event), field("Instrument")},
		},
		run: exnessOrders,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "exness-positions", Title: "Exness Positions", Category: "Finance", Group: "Exness",
			Description: "Lists an Exness connection's open positions, of Instrument or of every instrument, and goes on by Found when there is one, or by None, then by Done either way. Position is the first (position_id, direction buy or sell, volume, open_price; an account can hold several positions of one instrument, both ways).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Found", Event), sock("None", Event), sock("Positions", Object), sock("Position", Object)},
			Settings:    []*v1.Setting{exnessSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Instrument")},
		},
		run: exnessPositions,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "exness-close-position", Title: "Exness Close Position", Category: "Finance", Group: "Exness",
			Description: "Closes Instrument's open positions on an Exness connection at market, whole; their SL/TP orders go with them. The instrument's limit and stop orders still waiting are cancelled first. Position ID closes only that position, and cancels nothing. It refuses unless the connection's Allow Trading is ticked, and waits until Exness confirms each. None when there was nothing to close or cancel, Done after either. Positions are the positions closed, as they were; Cancelled the orders cancelled.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("None", Event), sock("Positions", Object), sock("Cancelled", Object)},
			Settings:    []*v1.Setting{exnessSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), hint("XAUUSD", field("Instrument")), field("Position ID")},
		},
		run: exnessClosePosition,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "exness-move-stop", Title: "Exness Move Stop", Category: "Finance", Group: "Exness",
			Description: "Moves the stop loss, and the take profit, of Instrument's open position on an Exness connection: its SL/TP order is changed, or one placed when it has none. A stop loss only ever moves closer to the price (up for a buy, down for a sell), and must stay on the right side of it (the bid for a buy, the ask for a sell). With several positions of the instrument open, Position ID picks one. It refuses unless the connection's Allow Trading is ticked. Order, when linked, gives instrument, position_id, stop_loss and take_profit as one object. Position is the position, Stop Order its SL/TP order after the move.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Position", Object), sock("Stop Order", Object)},
			Settings:    []*v1.Setting{exnessSetting()},
			Inputs: []*v1.Socket{sock("Event", Event), hint("XAUUSD", field("Instrument")), field("Position ID"), field("Stop Loss"), field("Take Profit"),
				sock("Order", Object)},
		},
		run: exnessMoveStop,
	},
}

func exnessSetting() *v1.Setting {
	return &v1.Setting{Name: "Connection", ConnectionType: store.Exness}
}

// exnessConn is the client of the step's Exness connection and the label
// its lines start with. A step that trades (trade) is refused unless the
// connection allows it, before anything reaches Exness.
func (c *call) exnessConn(trade bool) (*exness.Client, *v1.ExnessSettings, string, error) {
	id, err := c.connectionID("Exness")
	if err != nil {
		return nil, nil, "", err
	}
	if c.env.Connection == nil {
		return nil, nil, "", errors.New("this run has no connections")
	}
	conn, err := c.env.Connection(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, "", fmt.Errorf("connection %d is gone: pick another", id)
	}
	if err != nil {
		return nil, nil, "", err
	}
	if conn.Type != store.Exness {
		return nil, nil, "", fmt.Errorf("connection %s is not an Exness connection", conn.Name)
	}
	s := conn.Settings.GetExness()
	if trade && !s.GetAllowTrading() {
		return nil, nil, "", fmt.Errorf("%s: Allow Trading is off: this step trades real money; tick Allow Trading on the connection (Connections page) to let steps trade", conn.Name)
	}
	cl, err := c.env.Exness.Client(s)
	if err != nil {
		return nil, nil, "", fmt.Errorf("%s: %s", conn.Name, strings.TrimPrefix(err.Error(), "exness: "))
	}
	return cl, s, "Exness " + s.GetAccountId(), nil
}

// exnessInstrument is an instrument's name as Exness writes it.
var exnessInstrument = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// instrument reads the Instrument input, its case kept (XAUUSDm is not
// XAUUSDM). Empty is "" when optional.
func (c *call) instrument(required bool) (string, error) {
	s := strings.TrimSpace(c.text("Instrument"))
	switch {
	case s == "" && required:
		return "", fmt.Errorf("Instrument %w: give one such as XAUUSD (its name as the account shows it)", errEmpty)
	case s != "" && !exnessInstrument.MatchString(s):
		return "", fmt.Errorf("Instrument %q is not an Exness instrument name such as XAUUSD or EURUSDm", s)
	}
	return s, nil
}

// exnessOrderFields are an Order object's fields and the inputs they fill;
// symbol and quantity are taken too, as for Binance Order.
var exnessOrderFields = [][2]string{
	{"symbol", "Instrument"}, {"instrument", "Instrument"}, {"quantity", "Volume"}, {"volume", "Volume"}, {"price", "Price"},
	{"stop_loss", "Stop Loss"}, {"take_profit", "Take Profit"}, {"position_id", "Position ID"},
}

// exnessOrderObject puts a linked Order object's fields in the step's
// inputs, and gives its side (Buy, Sell) and type (Market, Limit, Stop) as
// the settings name them, "" for those it has not.
func (c *call) exnessOrderObject() (side, typ string, err error) {
	f := c.object("Order").GetStructValue().GetFields()
	if f == nil {
		return "", "", nil
	}
	for _, kv := range exnessOrderFields {
		if v, ok := f[kv[0]]; ok {
			c.inputs[kv[1]] = stringValue(valueText(v))
		}
	}
	switch s := strings.ToLower(valueText(f["side"])); s {
	case "":
	case "buy", "long":
		side = "Buy"
	case "sell", "short":
		side = "Sell"
	default:
		return "", "", fmt.Errorf("Order: side %q: buy or sell", s)
	}
	switch t := strings.ToLower(valueText(f["type"])); t {
	case "":
	case "market", "limit", "stop":
		typ = strings.ToUpper(t[:1]) + t[1:]
	default:
		return "", "", fmt.Errorf("Order: type %q: market, limit or stop", t)
	}
	return side, typ, nil
}

// exnessLimits are Exness Order's limits: 0 for none.
type exnessLimits struct {
	requireStop        bool
	maxVolume, maxLoss float64
}

func (c *call) exnessLimits() (exnessLimits, error) {
	l := exnessLimits{requireStop: c.setting("Require Stop") == "Yes"}
	for _, s := range []struct {
		name, unit string
		to         *float64
	}{{"Max Volume", "lots", &l.maxVolume}, {"Max Loss", "USD", &l.maxLoss}} {
		v := strings.ReplaceAll(c.setting(s.name), ",", "")
		if v == "" {
			continue
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || math.IsInf(f, 0) {
			return l, fmt.Errorf("%s %q: give %s, a number above 0", s.name, v, s.unit)
		}
		*s.to = f
	}
	return l, nil
}

// exnessRules are an instrument's volume and price rules.
type exnessRules struct {
	name                    string
	digits, volumeDigits    int
	min, max, step          float64
	contract                float64
	base, quote, minVolText string
	maxVolText, stepText    string
}

func rulesOf(cond exness.InstrumentCondition) exnessRules {
	num := func(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
	r := exnessRules{name: cond.Instrument, digits: cond.PointDigits, min: num(cond.VolumeMin), max: num(cond.VolumeMax), step: num(cond.VolumeStep),
		contract: num(cond.ContractSize), base: cond.BaseCurrency, quote: cond.QuoteCurrency,
		minVolText: cond.VolumeMin, maxVolText: cond.VolumeMax, stepText: cond.VolumeStep}
	if i := strings.IndexByte(cond.VolumeStep, '.'); i >= 0 {
		r.volumeDigits = len(strings.TrimRight(cond.VolumeStep[i+1:], "0"))
	}
	return r
}

func (r exnessRules) price(p float64) string { return strconv.FormatFloat(p, 'f', r.digits, 64) }
func (r exnessRules) volume(v float64) string {
	return strconv.FormatFloat(v, 'f', max(r.volumeDigits, 2), 64)
}
func (r exnessRules) roundPrice(p float64) float64 {
	m := math.Pow(10, float64(r.digits))
	return math.Round(p*m) / m
}

// floorVolume is v rounded down to the step: never more than asked.
func (r exnessRules) floorVolume(v float64) float64 {
	if r.step <= 0 {
		return v
	}
	n := math.Floor(v/r.step + 1e-9)
	m := math.Pow(10, float64(max(r.volumeDigits, 2)))
	return math.Round(n*r.step*m) / m
}

// usdPerLot is what one lot loses in USD over risk (a price distance),
// with the price at the stop for an instrument based on USD.
func (r exnessRules) usdPerLot(risk, stop float64) (float64, error) {
	if r.contract <= 0 {
		return 0, fmt.Errorf("Max Loss: Exness gives no contract size for %s", r.name)
	}
	switch {
	case r.quote == "USD":
		return risk * r.contract, nil
	case r.base == "USD":
		return risk * r.contract / stop, nil
	}
	return 0, fmt.Errorf("Max Loss: %s's profit is in %s; Max Loss is only checked on instruments quoted in USD or based on it (XAUUSD, EURUSD, USDJPY)", r.name, cmp.Or(r.quote, "an unknown currency"))
}

// exnessQuote is an instrument's bid and ask now: the forming minute's close.
func exnessQuote(c *call, cl *exness.Client, label, instrument string) (bid, ask float64, err error) {
	cs, err := cl.Candles(c.ctx, exness.CandleQuery{Instrument: instrument, Timeframe: exness.M1, PriceType: exness.PriceBoth, From: cl.Now(), Count: -1})
	if err != nil {
		return 0, 0, exnessErr(label, err)
	}
	if len(cs) == 0 || cs[len(cs)-1].Bid == nil || cs[len(cs)-1].Ask == nil {
		return 0, 0, fmt.Errorf("%s: no price for %s now: its market may be closed", label, instrument)
	}
	last := cs[len(cs)-1]
	return last.Bid.Close, last.Ask.Close, nil
}

// exnessDone waits until Exness has done operation op of a trading request.
func exnessDone(c *call, cl *exness.Client, ack exness.Ack) (exness.Operation, error) {
	ctx, cancel := context.WithTimeout(c.ctx, exnessWait)
	defer cancel()
	op, err := cl.WaitOperation(ctx, ack.OperationID, exnessPoll)
	if errors.Is(err, context.DeadlineExceeded) && c.ctx.Err() == nil {
		return op, fmt.Errorf("Exness has not finished operation %s in %s: look at the account (Exness Positions, Exness Orders) before sending it again", ack.OperationID, exnessWait)
	}
	return op, err
}

func exnessOrder(c *call) error {
	cl, _, label, err := c.exnessConn(true)
	if err != nil {
		return err
	}
	sideSet, typeSet, err := c.exnessOrderObject()
	if err != nil {
		return err
	}
	instrument, err := c.instrument(true)
	if err != nil {
		return err
	}
	side := exness.Buy
	if cmp.Or(sideSet, c.setting("Side")) == "Sell" {
		side = exness.Sell
	}
	typ := cmp.Or(typeSet, cmp.Or(c.setting("Type"), "Market"))
	if !slices.Contains([]string{"Market", "Limit", "Stop"}, typ) {
		return fmt.Errorf("Type %q: give Market, Limit or Stop", typ)
	}
	vol, err := c.amount("Volume", true)
	if err != nil {
		return err
	}
	price, err := c.amount("Price", typ != "Market")
	if err != nil {
		return err
	}
	sl, err := c.amount("Stop Loss", false)
	if err != nil {
		return err
	}
	tp, err := c.amount("Take Profit", false)
	if err != nil {
		return err
	}
	limits, err := c.exnessLimits()
	if err != nil {
		return err
	}

	cond, err := cl.Instrument(c.ctx, instrument)
	if err != nil {
		return exnessErr(label, err)
	}
	r := rulesOf(cond)
	v := r.floorVolume(vol)
	if v <= 0 || v < r.min-1e-9 {
		return fmt.Errorf("Volume %s is below %s's least, %s lots (step %s)", strings.TrimSpace(c.text("Volume")), instrument, r.minVolText, r.stepText)
	}
	if r.max > 0 && v > r.max+1e-9 {
		return fmt.Errorf("Volume %s is over %s's most, %s lots", r.volume(v), instrument, r.maxVolText)
	}
	if limits.maxVolume > 0 && v > limits.maxVolume+1e-9 {
		return fmt.Errorf("Max Volume: %s lots is over %s", r.volume(v), strconv.FormatFloat(limits.maxVolume, 'f', -1, 64))
	}
	sl, tp = r.roundPrice(sl), r.roundPrice(tp)
	ref := r.roundPrice(price)
	if typ == "Market" {
		bid, ask, err := exnessQuote(c, cl, label, instrument)
		if err != nil {
			return err
		}
		ref = map[exness.Side]float64{exness.Buy: ask, exness.Sell: bid}[side]
	}
	buy := side == exness.Buy
	if sl > 0 && (buy && sl >= ref || !buy && sl <= ref) {
		return fmt.Errorf("Stop Loss %s is on the wrong side of the entry, %s: %s", r.price(sl), r.price(ref), map[bool]string{true: "below it for a buy", false: "above it for a sell"}[buy])
	}
	if tp > 0 && (buy && tp <= ref || !buy && tp >= ref) {
		return fmt.Errorf("Take Profit %s is on the wrong side of the entry, %s: %s", r.price(tp), r.price(ref), map[bool]string{true: "above it for a buy", false: "below it for a sell"}[buy])
	}
	if (limits.requireStop || limits.maxLoss > 0) && sl == 0 {
		return errors.New("Require Stop: the order needs a Stop Loss")
	}
	if limits.maxLoss > 0 {
		risk := math.Abs(ref - sl)
		perLot, err := r.usdPerLot(risk, sl)
		if err != nil {
			return err
		}
		if loss := v * perLot; loss > limits.maxLoss+1e-9 {
			return fmt.Errorf("Max Loss: %s lots lose %.2f USD at the stop %s (%s from %s), over %.2f; at most %s", r.volume(v), loss, r.price(sl),
				r.price(risk), r.price(ref), limits.maxLoss, r.volume(r.floorVolume(limits.maxLoss/perLot)))
		}
	}

	text := func(p float64) string {
		if p == 0 {
			return ""
		}
		return r.price(p)
	}
	var ack exness.Ack
	if typ == "Market" {
		p := exness.OpenPosition{Instrument: instrument, Side: side, Volume: r.volume(v), StopLoss: text(sl), TakeProfit: text(tp)}
		if cond.ExecMode == "instant" {
			p.Price = r.price(ref) // instant execution takes the price it fills at
		}
		ack, err = cl.OpenPosition(c.ctx, p, "")
	} else {
		ack, err = cl.PlaceOrder(c.ctx, exness.PendingOrder{Type: exness.OrderType(strings.ToLower(typ)), Instrument: instrument, Side: side,
			Volume: r.volume(v), Price: r.price(ref), StopLoss: text(sl), TakeProfit: text(tp)}, "")
	}
	what := fmt.Sprintf("%s %s %s %s", side, r.volume(v), instrument, map[bool]string{true: "at market (" + r.price(ref) + ")", false: strings.ToLower(typ) + " at " + r.price(ref)}[typ == "Market"])
	if sl > 0 {
		what += ", stop loss " + r.price(sl)
	}
	if tp > 0 {
		what += ", take profit " + r.price(tp)
	}
	if err != nil {
		return exnessErr(label, err)
	}
	op, err := exnessDone(c, cl, ack)
	var oe *exness.OperationError
	if errors.As(err, &oe) {
		return fmt.Errorf("%s: %s %s %s refused: %s (%d)", label, side, r.volume(v), instrument, oe.Message, oe.Code)
	}
	if err != nil {
		return fmt.Errorf("%s: %s: %w", label, what, err)
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: %s: %s", label, what, op.Status))
	c.outputs["Order"] = jsonValue(map[string]any{"instrument": instrument, "side": side, "type": strings.ToLower(typ), "volume": r.volume(v),
		"price": r.price(ref), "stop_loss": text(sl), "take_profit": text(tp), "operation_id": ack.OperationID, "status": op.Status,
		"idempotency_key": ack.IdempotencyKey})
	c.outputs["Operation ID"] = stringValue(ack.OperationID)
	c.fire("Done")
	return nil
}

// exnessTimeframes are Exness Candles' timeframes, the first the default.
var exnessTimeframes = []string{"M1", "M3", "M5", "M10", "M15", "M30", "H1", "H2", "H4", "D1", "W1", "MN"}

// candleEnd is when a candle that opened at t at timeframe tf closes.
func candleEnd(t time.Time, tf string) time.Time {
	switch tf {
	case "MN":
		return t.AddDate(0, 1, 0)
	case "W1":
		return t.AddDate(0, 0, 7)
	case "D1":
		return t.AddDate(0, 0, 1)
	}
	n, _ := strconv.Atoi(tf[1:])
	unit := map[byte]time.Duration{'M': time.Minute, 'H': time.Hour}[tf[0]]
	return t.Add(time.Duration(n) * unit)
}

func exnessCandles(c *call) error {
	cl, _, label, err := c.exnessConn(false)
	if err != nil {
		return err
	}
	instrument, err := c.instrument(true)
	if err != nil {
		return err
	}
	tf := cmp.Or(c.setting("Timeframe"), exnessTimeframes[0])
	if !slices.Contains(exnessTimeframes, tf) {
		return fmt.Errorf("Timeframe %q: give one of %s", tf, strings.Join(exnessTimeframes, ", "))
	}
	pt := map[string]exness.PriceType{"": exness.PriceBid, "Bid": exness.PriceBid, "Ask": exness.PriceAsk, "Both": exness.PriceBoth}[c.setting("Price")]
	if pt == "" {
		return fmt.Errorf("Price %q: give Bid, Ask or Both", c.setting("Price"))
	}
	limit, err := c.limitSetting(100, 1000)
	if err != nil {
		return err
	}
	now := cl.Now()
	cs, err := cl.Candles(c.ctx, exness.CandleQuery{Instrument: instrument, Timeframe: exness.Timeframe(tf), PriceType: pt, From: now, Count: -limit})
	if err != nil {
		return exnessErr(label, err)
	}
	c.outputs["Candles"] = jsonValue(orEmpty(cs))
	c.outputs["Close"] = stringValue("")
	for i := len(cs) - 1; i >= 0; i-- {
		if candleEnd(cs[i].Time, tf).After(now) {
			continue
		}
		c.outputs["Last Closed"] = jsonValue(cs[i])
		close := cs[i].Close
		if cs[i].Bid != nil {
			close = cs[i].Bid.Close
		}
		c.outputs["Close"] = stringValue(strconv.FormatFloat(close, 'f', -1, 64))
		break
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: %d %s candles of %s", label, len(cs), tf, instrument))
	c.fire("Done")
	return nil
}

func exnessPortfolio(c *call) error {
	cl, s, label, err := c.exnessConn(false)
	if err != nil {
		return err
	}
	acc, err := cl.Account(c.ctx)
	if err != nil {
		return exnessErr(label, err)
	}
	snap, err := cl.Snapshot(c.ctx)
	if err != nil {
		return exnessErr(label, err)
	}
	equity, _ := strconv.ParseFloat(snap.AccountState.Equity, 64)
	used, _ := strconv.ParseFloat(snap.AccountState.UsedMargin, 64)
	c.outputs["Account"] = jsonValue(map[string]any{"account_id": acc.AccountID, "name": acc.AccountName, "currency": acc.Settings.Currency,
		"leverage": acc.Settings.Leverage, "trade_mode": acc.Settings.TradeMode, "account_status": acc.Settings.AccountStatus,
		"balance": snap.AccountState.Balance, "equity": snap.AccountState.Equity, "used_margin": snap.AccountState.UsedMargin,
		"free_margin": fmt.Sprintf("%.2f", equity-used), "allow_trading": s.GetAllowTrading()})
	c.outputs["Positions"] = jsonValue(orEmpty(snap.Positions))
	c.outputs["Orders"] = jsonValue(orEmpty(snap.Orders))
	c.logs = append(c.logs, fmt.Sprintf("%s: balance %s %s, equity %s, %d positions, %d orders waiting", label, snap.AccountState.Balance,
		acc.Settings.Currency, snap.AccountState.Equity, len(snap.Positions), len(snap.Orders)))
	c.fire("Done")
	return nil
}

func exnessOrders(c *call) error {
	cl, _, label, err := c.exnessConn(false)
	if err != nil {
		return err
	}
	instrument, err := c.instrument(false)
	if err != nil {
		return err
	}
	show := cmp.Or(c.setting("Show"), "Open")
	if !slices.Contains([]string{"Open", "History", "Deals"}, show) {
		return fmt.Errorf("Show %q: give Open, History or Deals", show)
	}
	limit, err := c.limitSetting(50, 1000)
	if err != nil {
		return err
	}
	days := 7
	if s := c.setting("Days"); s != "" {
		if days, err = strconv.Atoi(s); err != nil || days < 1 || days > 365 {
			return fmt.Errorf("Days %q: give a number from 1 to 365", s)
		}
	}
	var list []any
	switch show {
	case "Open":
		snap, err := cl.Snapshot(c.ctx)
		if err != nil {
			return exnessErr(label, err)
		}
		os := snap.Orders
		slices.SortStableFunc(os, func(a, b exness.Order) int { return b.CreateTime.Compare(a.CreateTime) })
		for _, o := range os {
			if instrument == "" || o.Instrument == instrument {
				list = append(list, o)
			}
		}
	default:
		q := exness.HistoryQuery{From: cl.Now().AddDate(0, 0, -days), Limit: 200, Instrument: instrument}
		var all []any
		// Pages come oldest first: all of them, then the newest Limit.
		for page := 0; page < 50; page++ {
			var more bool
			if show == "History" {
				p, err := cl.Orders(c.ctx, q)
				if err != nil {
					return exnessErr(label, err)
				}
				for _, it := range p.Items {
					all = append(all, it.Order)
				}
				q, more = exness.HistoryQuery{Cursor: p.NextCursor, Limit: 200}, p.HasMore && p.NextCursor != ""
			} else {
				p, err := cl.Deals(c.ctx, q)
				if err != nil {
					return exnessErr(label, err)
				}
				for _, it := range p.Items {
					all = append(all, it.Deal)
				}
				q, more = exness.HistoryQuery{Cursor: p.NextCursor, Limit: 200}, p.HasMore && p.NextCursor != ""
			}
			if !more {
				break
			}
		}
		slices.Reverse(all)
		list = all
	}
	if len(list) > limit {
		list = list[:limit]
	}
	out := "Orders"
	if show == "Deals" {
		out = "Deals"
	}
	c.outputs[out] = jsonValue(orEmpty(list))
	if len(list) > 0 {
		c.outputs["Last"] = jsonValue(list[0])
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: %d %s", label, len(list), strings.ToLower(map[string]string{"Open": "orders waiting", "History": "orders in history", "Deals": "deals"}[show])))
	c.fire("Done")
	return nil
}

func exnessPositions(c *call) error {
	cl, _, label, err := c.exnessConn(false)
	if err != nil {
		return err
	}
	instrument, err := c.instrument(false)
	if err != nil {
		return err
	}
	snap, err := cl.Snapshot(c.ctx)
	if err != nil {
		return exnessErr(label, err)
	}
	var ps []exness.Position
	for _, p := range snap.Positions {
		if instrument == "" || p.Instrument == instrument {
			ps = append(ps, p)
		}
	}
	c.outputs["Positions"] = jsonValue(orEmpty(ps))
	if len(ps) > 0 {
		c.outputs["Position"] = jsonValue(ps[0])
		c.fire("Found")
	} else {
		c.fire("None")
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: %d open positions", label, len(ps)))
	c.fire("Done")
	return nil
}

func exnessClosePosition(c *call) error {
	cl, _, label, err := c.exnessConn(true)
	if err != nil {
		return err
	}
	instrument, err := c.instrument(true)
	if err != nil {
		return err
	}
	only := strings.TrimSpace(c.text("Position ID"))
	snap, err := cl.Snapshot(c.ctx)
	if err != nil {
		return exnessErr(label, err)
	}
	var ps []exness.Position
	for _, p := range snap.Positions {
		if p.Instrument == instrument && (only == "" || p.PositionID == only) {
			ps = append(ps, p)
		}
	}
	if only != "" && len(ps) == 0 {
		return fmt.Errorf("%s: no open %s position %s", label, instrument, only)
	}
	var cancelled []exness.Order
	if only == "" {
		for _, o := range snap.Orders {
			if o.Instrument != instrument || (o.OrderType != "limit" && o.OrderType != "stop") {
				continue
			}
			ack, err := cl.CancelOrder(c.ctx, o.OrderID, "")
			if err == nil {
				_, err = exnessDone(c, cl, ack)
			}
			if err != nil {
				return fmt.Errorf("cancelling order %s: %w", o.OrderID, exnessErr(label, err))
			}
			cancelled = append(cancelled, o)
			c.logs = append(c.logs, fmt.Sprintf("%s: cancelled the %s %s %s %s at %s", label, o.Direction, o.OrderType, o.Volume, o.Instrument, o.Price))
		}
	}
	var closed []exness.Position
	for _, p := range ps {
		ack, err := cl.ClosePosition(c.ctx, p.PositionID, exness.CloseOptions{}, "")
		if err == nil {
			_, err = exnessDone(c, cl, ack)
		}
		if err != nil {
			return fmt.Errorf("closing position %s: %w", p.PositionID, exnessErr(label, err))
		}
		closed = append(closed, p)
		c.logs = append(c.logs, fmt.Sprintf("%s: closed the %s %s %s from %s (position %s)", label, p.Direction, p.Volume, p.Instrument, p.OpenPrice, p.PositionID))
	}
	c.outputs["Positions"] = jsonValue(orEmpty(closed))
	c.outputs["Cancelled"] = jsonValue(orEmpty(cancelled))
	if len(closed) == 0 && len(cancelled) == 0 {
		c.logs = append(c.logs, fmt.Sprintf("%s: no open %s position or order", label, instrument))
		c.fire("None")
	}
	c.fire("Done")
	return nil
}

func exnessMoveStop(c *call) error {
	cl, _, label, err := c.exnessConn(true)
	if err != nil {
		return err
	}
	if _, _, err := c.exnessOrderObject(); err != nil {
		return err
	}
	instrument, err := c.instrument(true)
	if err != nil {
		return err
	}
	sl, err := c.amount("Stop Loss", false)
	if err != nil {
		return err
	}
	tp, err := c.amount("Take Profit", false)
	if err != nil {
		return err
	}
	if sl == 0 && tp == 0 {
		return errors.New("give a Stop Loss, a Take Profit, or both")
	}
	only := strings.TrimSpace(c.text("Position ID"))
	snap, err := cl.Snapshot(c.ctx)
	if err != nil {
		return exnessErr(label, err)
	}
	var ps []exness.Position
	for _, p := range snap.Positions {
		if p.Instrument == instrument && (only == "" || p.PositionID == only) {
			ps = append(ps, p)
		}
	}
	switch {
	case len(ps) == 0 && only != "":
		return fmt.Errorf("%s: no open %s position %s", label, instrument, only)
	case len(ps) == 0:
		return fmt.Errorf("%s: no open %s position", label, instrument)
	case len(ps) > 1:
		return fmt.Errorf("%s: %d open %s positions: give Position ID", label, len(ps), instrument)
	}
	pos := ps[0]
	var old *exness.Order
	for i, o := range snap.Orders {
		if o.OrderType == "sltp" && o.PositionID == pos.PositionID {
			old = &snap.Orders[i]
		}
	}
	cond, err := cl.Instrument(c.ctx, instrument)
	if err != nil {
		return exnessErr(label, err)
	}
	r := rulesOf(cond)
	sl, tp = r.roundPrice(sl), r.roundPrice(tp)
	bid, ask, err := exnessQuote(c, cl, label, instrument)
	if err != nil {
		return err
	}
	buy := pos.Direction == exness.Buy
	price := map[bool]float64{true: bid, false: ask}[buy]
	if sl > 0 {
		if buy && sl >= price || !buy && sl <= price {
			return fmt.Errorf("Stop Loss %s is on the wrong side of the price %s", r.price(sl), r.price(price))
		}
		if old != nil && old.StopLossPrice != "" {
			was, _ := strconv.ParseFloat(old.StopLossPrice, 64)
			if buy && sl <= was || !buy && sl >= was {
				return fmt.Errorf("Stop Loss %s would loosen the stop at %s: a stop only moves closer to the price", r.price(sl), r.price(was))
			}
		}
	}
	if tp > 0 && (buy && tp <= price || !buy && tp >= price) {
		return fmt.Errorf("Take Profit %s is on the wrong side of the price %s", r.price(tp), r.price(price))
	}
	text := func(p float64) string {
		if p == 0 {
			return ""
		}
		return r.price(p)
	}
	var ack exness.Ack
	if old != nil {
		ack, err = cl.ModifyOrder(c.ctx, old.OrderID, exness.ModifyOrder{StopLoss: text(sl), TakeProfit: text(tp)}, "")
	} else {
		ack, err = cl.PlaceOrder(c.ctx, exness.PendingOrder{Type: exness.OrderSLTP, PositionID: pos.PositionID, StopLoss: text(sl), TakeProfit: text(tp)}, "")
	}
	if err == nil {
		_, err = exnessDone(c, cl, ack)
	}
	if err != nil {
		return fmt.Errorf("the new stop on %s position %s: %w", instrument, pos.PositionID, exnessErr(label, err))
	}
	for _, m := range []struct {
		name  string
		price float64
		was   string
	}{{"stop loss", sl, oldPrice(old, true)}, {"take profit", tp, oldPrice(old, false)}} {
		if m.price == 0 {
			continue
		}
		line := fmt.Sprintf("%s: %s %s at %s", label, m.name, instrument, r.price(m.price))
		if m.was != "" {
			line += ", was " + m.was
		}
		c.logs = append(c.logs, line)
	}
	c.outputs["Position"] = jsonValue(pos)
	if after, err := cl.Snapshot(c.ctx); err == nil {
		for _, o := range after.Orders {
			if o.OrderType == "sltp" && o.PositionID == pos.PositionID {
				c.outputs["Stop Order"] = jsonValue(o)
			}
		}
	}
	c.fire("Done")
	return nil
}

// oldPrice is an SL/TP order's stop loss (sl) or take profit, "" for none.
func oldPrice(o *exness.Order, sl bool) string {
	switch {
	case o == nil:
		return ""
	case sl:
		return o.StopLossPrice
	}
	return o.TakeProfitPrice
}
