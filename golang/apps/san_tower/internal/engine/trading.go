package engine

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
)

// Trading safely from a workflow (the user, 2026-10-29: autotrade as a
// workflow, an AI deciding through Workflow Tools): Binance Order takes an
// AI's whole order as one object and checks it against limits of its own,
// and Binance Move Stop moves a position's stop, only ever closer.

// orderFields are an Order object's fields and the inputs they fill.
var orderFields = map[string]string{
	"symbol": "Symbol", "quantity": "Quantity", "price": "Price",
	"stop_loss": "Stop Loss", "take_profit": "Take Profit", "leverage": "Leverage",
}

// orderObject puts a linked Order object's fields in the step's inputs, and
// gives its side (Buy, Sell), type (Market, Limit) and time in force as
// the settings name them, "" for those it has not.
func (c *call) orderObject() (side, typ, tif string, err error) {
	f := c.object("Order").GetStructValue().GetFields()
	if f == nil {
		return "", "", "", nil
	}
	for key, input := range orderFields {
		if v, ok := f[key]; ok {
			c.inputs[input] = stringValue(valueText(v))
		}
	}
	switch s := strings.ToLower(valueText(f["side"])); s {
	case "":
	case "buy", "long":
		side = "Buy"
	case "sell", "short":
		side = "Sell"
	default:
		return "", "", "", fmt.Errorf("Order: side %q: buy or sell", s)
	}
	switch t := strings.ToLower(valueText(f["type"])); t {
	case "":
	case "market":
		typ = "Market"
	case "limit":
		typ = "Limit"
	default:
		return "", "", "", fmt.Errorf("Order: type %q: market or limit", t)
	}
	switch t := strings.ToLower(strings.NewReplacer("_", " ", "-", " ").Replace(valueText(f["time_in_force"]))); t {
	case "":
	case "post only", "gtx":
		tif = "Post Only"
	case "gtc", "ioc", "fok":
		tif = strings.ToUpper(t)
	default:
		return "", "", "", fmt.Errorf("Order: time_in_force %q: gtc, post_only, ioc or fok", t)
	}
	return side, typ, tif, nil
}

// valueText is a value as an input's text: a number without exponent.
func valueText(v *structpb.Value) string {
	switch x := v.GetKind().(type) {
	case *structpb.Value_NumberValue:
		return strconv.FormatFloat(x.NumberValue, 'f', -1, 64)
	case *structpb.Value_StringValue:
		return strings.TrimSpace(x.StringValue)
	case *structpb.Value_BoolValue:
		return strconv.FormatBool(x.BoolValue)
	}
	return ""
}

// orderLimits are Binance Order's limits: 0 for none.
type orderLimits struct {
	requireStop          bool
	maxNotional, maxLoss float64
}

func (c *call) orderLimits() (orderLimits, error) {
	l := orderLimits{requireStop: c.setting("Require Stop") == "Yes"}
	for _, s := range []struct {
		name string
		to   *float64
	}{{"Max Notional", &l.maxNotional}, {"Max Loss", &l.maxLoss}} {
		v := strings.ReplaceAll(c.setting(s.name), ",", "")
		if v == "" {
			continue
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 {
			return l, fmt.Errorf("%s %q: give USDT, a number above 0", s.name, v)
		}
		*s.to = f
	}
	return l, nil
}

func (l orderLimits) any() bool { return l.requireStop || l.maxNotional > 0 || l.maxLoss > 0 }

// check refuses an order of q at price, with a stop at sl (0: none), over
// the limits, saying the most that fits. A reduce-only order, which only
// closes, is not held to them.
func (l orderLimits) check(info binance.SymbolInfo, q, price, sl float64, reduceOnly bool) error {
	if reduceOnly || !l.any() {
		return nil
	}
	if (l.requireStop || l.maxLoss > 0) && sl == 0 {
		return errors.New("Require Stop: the order needs a Stop Loss")
	}
	if l.maxNotional > 0 && q*price > l.maxNotional {
		return fmt.Errorf("Max Notional: %s × %s = %.2f USDT, over %.2f; at most %s", info.FormatQty(q), info.FormatPrice(price),
			q*price, l.maxNotional, info.FormatQty(info.FloorQty(l.maxNotional/price)))
	}
	if risk := math.Abs(price - sl); l.maxLoss > 0 && q*risk > l.maxLoss {
		return fmt.Errorf("Max Loss: %s lost %.2f USDT at the stop %s (%s from %s), over %.2f; at most %s", info.FormatQty(q), q*risk,
			info.FormatPrice(sl), info.FormatPrice(risk), info.FormatPrice(price), l.maxLoss, info.FormatQty(info.FloorQty(l.maxLoss/risk)))
	}
	return nil
}

var tradingKinds = []*kind{{
	StepKind: &v1.StepKind{
		Kind: "binance-move-stop", Title: "Binance Move Stop", Category: "Finance", Group: "Binance",
		Description: "Moves the stop loss, and the take profit, of Symbol's open position on a Binance connection: the new one is placed (reduce-only, for the whole position, on the mark price), then the old one cancelled. A stop loss only ever moves closer to the price (up for a long, down for a short), and must stay on the right side of the mark price. Order, when linked, gives symbol, stop_loss and take_profit as one object. Position is the position, Stop Loss Order and Take Profit Order the new ones.",
		Outputs:     []*v1.Socket{sock("Done", Event), sock("Position", Object), sock("Stop Loss Order", Object), sock("Take Profit Order", Object)},
		Settings:    []*v1.Setting{binanceSetting()},
		Inputs:      []*v1.Socket{sock("Event", Event), field("Symbol"), field("Stop Loss"), field("Take Profit"), sock("Order", Object)},
	},
	run: binanceMoveStop,
}}

func binanceMoveStop(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	if _, _, _, err := c.orderObject(); err != nil {
		return err
	}
	symbol, err := c.symbol(true)
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
	ps, err := cl.Positions(c.ctx, symbol)
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	var pos *binance.Position
	for i := range ps {
		if ps[i].Symbol == symbol && ps[i].Amount != 0 {
			pos = &ps[i]
		}
	}
	if pos == nil {
		return fmt.Errorf("%s: no open position in %s", network, symbol)
	}
	info, err := cl.SymbolInfo(c.ctx, symbol)
	if err != nil {
		return err
	}
	long := pos.Amount > 0
	closeSide := map[bool]string{true: "SELL", false: "BUY"}[long]
	qty := info.FormatQty(math.Abs(pos.Amount))
	sl, tp = info.RoundPrice(sl), info.RoundPrice(tp)
	algos, err := cl.OpenAlgoOrders(c.ctx, symbol)
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	old := map[string]*binance.AlgoOrder{}
	for i := range algos {
		if t := algos[i].Type; t == binance.StopMarket || t == binance.TakeProfitMarket {
			old[t] = &algos[i]
		}
	}
	if sl > 0 {
		if long && sl >= pos.MarkPrice || !long && sl <= pos.MarkPrice {
			return fmt.Errorf("Stop Loss %s is on the wrong side of the mark price %s", info.FormatPrice(sl), info.FormatPrice(pos.MarkPrice))
		}
		if o := old[binance.StopMarket]; o != nil && (long && sl <= o.TriggerPrice || !long && sl >= o.TriggerPrice) {
			return fmt.Errorf("Stop Loss %s would loosen the stop at %s: a stop only moves closer to the price", info.FormatPrice(sl), info.FormatPrice(o.TriggerPrice))
		}
	}
	if tp > 0 && (long && tp <= pos.MarkPrice || !long && tp >= pos.MarkPrice) {
		return fmt.Errorf("Take Profit %s is on the wrong side of the mark price %s", info.FormatPrice(tp), info.FormatPrice(pos.MarkPrice))
	}
	for _, m := range []struct {
		output, kind string
		price        float64
	}{{"Stop Loss Order", binance.StopMarket, sl}, {"Take Profit Order", binance.TakeProfitMarket, tp}} {
		if m.price == 0 {
			continue
		}
		// The new one first: the position is never without its stop.
		a, err := cl.ReduceStopOrder(c.ctx, symbol, closeSide, m.kind, info.FormatPrice(m.price), qty)
		if err != nil {
			return fmt.Errorf("%s: the new %s at %s was refused: %w", network, strings.ToLower(strings.TrimSuffix(m.output, " Order")), info.FormatPrice(m.price), err)
		}
		c.outputs[m.output] = jsonValue(a)
		line := fmt.Sprintf("%s: %s %s at %s", network, strings.ToLower(strings.TrimSuffix(m.output, " Order")), symbol, info.FormatPrice(m.price))
		if o := old[m.kind]; o != nil {
			if err := cl.CancelAlgoOrder(c.ctx, symbol, o.AlgoID); err != nil {
				c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%s: the old one at %s was not cancelled: %v", line, info.FormatPrice(o.TriggerPrice), err)
			}
			line += ", was " + info.FormatPrice(o.TriggerPrice)
		}
		c.logs = append(c.logs, line)
	}
	c.outputs["Position"] = jsonValue(pos)
	c.fire("Done")
	return nil
}
