package knowledge

import (
	"fmt"
	"math"
	"sort"
)

// stopNow is where the position's stop is: the last protect's, or the
// initial one.
func (p Position) stopNow() float64 {
	if p.StopNow > 0 {
		return p.StopNow
	}
	return p.Stop
}

// atStop is what the position loses if its stop fills now, before fees; 0 once
// the stop is past the entry and locks in a profit.
func (p Position) atStop() float64 {
	d := p.Entry - p.stopNow()
	if p.Side == "short" {
		d = -d
	}
	return math.Max(0, d*p.Qty)
}

// riskProps sums up the risk the portfolio carries against the limits: what
// the open positions have at stake now, and how deep the closed ones went.
func riskProps(in Input, w Account) map[string]any {
	cfg := in.Config
	// over allows the 1% a fill can land past the limit the open was checked
	// against at the mark price.
	over := func(v, limit float64) bool { return limit > 0 && v > limit*1.01 }

	var open int
	var notional, margin, atStake, unrealized float64
	warnings := []string{}
	var closed []Position
	for _, p := range in.Positions {
		if p.Close != nil {
			closed = append(closed, p)
			continue
		}
		open++
		// Each pair is held to its venue's limits: a MetaTrader pair to mt5's.
		lev, maxPos, maxRisk, posKey, riskKey := cfg.limits(p.Symbol)
		n, r := p.Entry*p.Qty, p.atStop()
		notional += n
		margin += n / float64(max(lev, 1))
		atStake += r
		unrealized += p.Unrealized
		if over(n, maxPos) {
			warnings = append(warnings, fmt.Sprintf("position #%d is worth %.2f USDT, over %s %.2f", p.N, n, posKey, maxPos))
		}
		if over(r, maxRisk) {
			warnings = append(warnings, fmt.Sprintf("position #%d loses %.2f USDT at its stop, over %s %.2f", p.N, r, riskKey, maxRisk))
		}
	}

	// The closed positions in the order they closed: the equity curve.
	sort.SliceStable(closed, func(i, j int) bool { return closed[i].Close.Closed.Before(closed[j].Close.Closed) })
	var cum, peak, maxDD, worst, worstR float64
	streak := 0
	for _, p := range closed {
		c := p.Close
		cum += c.Net
		peak = math.Max(peak, cum)
		maxDD = math.Max(maxDD, peak-cum)
		if c.Net < worst {
			worst, worstR = c.Net, c.R
		}
		if c.Net > 0 {
			streak = 0
		} else {
			streak++
		}
	}

	pct := func(v float64) float64 {
		if w.Wallet <= 0 {
			return 0
		}
		return round(v/w.Wallet*100, 2)
	}
	props := map[string]any{
		"leverage": cfg.Leverage, "max_position_usdt": cfg.MaxPositionUSDT, "max_risk_usdt": cfg.MaxRiskUSDT, "max_open_per_pair": cfg.MaxOpenPerPair,
		"open": open, "open_notional": round(notional, 2), "open_margin": round(margin, 2),
		"open_risk": round(atStake, 4), "unrealized": round(unrealized, 4),
		"trades": len(closed), "net": round(cum, 4), "worst_net": round(worst, 4), "worst_r": round(worstR, 3),
		"max_drawdown": round(maxDD, 4), "drawdown": round(peak-cum, 4), "loss_streak": streak,
		"wallet": round(w.Wallet, 2), "wallet_known": in.Account != nil,
		"max_risk_pct": pct(cfg.MaxRiskUSDT), "open_risk_pct": pct(atStake), "margin_pct": pct(margin),
		"warnings": warnings,
	}
	if m := cfg.MT5; len(m.Pairs) > 0 {
		props["mt5_pairs"], props["mt5_leverage"] = m.Pairs, m.Leverage
		props["mt5_max_position"], props["mt5_max_risk"] = m.MaxPosition, m.MaxRisk
	}
	return props
}
