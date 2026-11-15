package san_backtest

// Chain strategies: a strategy written as an ordered list of small steps,
// a chain of responsibility (the user, 2026-11-15: "can we make strategy
// code like chainable pipeline" and "In function like chain of
// responsibility"). Build turns the chain into an ordinary Strategy, so the
// engine, the reports, run.json, decisions.jsonl and the tower's Backtest
// Analysis work with it unchanged.
//
//	st, err := bt.Chain("pullback_trend", "H1", "H4").Use(
//		steps.ManageOpen(steps.TimeStop(69)),
//		steps.Hours(14, 23),
//		steps.EMAAgree("H4", 20, 50),
//		steps.PullbackTo("H1", steps.EMA("H4", 20)),
//		steps.SwingStop("H1", 12, steps.ATR("H1", 14).Times(0.5), steps.ATR("H1", 14)),
//		steps.RR(2),
//	).Build()
//
// Each step gets the Context and next, the rest of the chain. It may stop
// the chain (return Wait, best with a reason: bt.Wait.Because("...")), call
// next and pass its answer on, change the answer next gave on the way
// back, or answer itself (a Close, a MoveStop). Order matters: a step only
// runs when every step before it called next.
//
// The steps share four fields on the Context, reset on every candle: Dir
// (the side the chain is looking at: Long, Short or "" for not set yet),
// Entry (the planned entry price, the step candle's close), Stop and Target
// (NaN until a step sets them). A direction step sets Dir, so the steps
// after it are written once for both sides; triggers check both sides while
// Dir is not set. When the end of the chain is reached with Dir set, the
// chain answers Buy or Sell with Stop and Target.
//
// San's rule #1 (every trade has a stop-loss and a take-profit) is always
// enforced: Build adds RequireSLTP as the innermost step, and the final
// answer is checked once more, so an entry without a stop and a target on
// the right sides of Entry becomes Wait with a note.

// Next runs the rest of the chain.
type Next func(c *Context) Signal

// Step is one step of a chain as a plain function.
type Step func(c *Context, next Next) Signal

// Run makes Step a Link.
func (s Step) Run(c *Context, next Next) Signal { return s(c, next) }

// Link is anything Chain.Use takes: a Step, or a built-in step (package
// steps) that also says what it needs by implementing Setup.
type Link interface {
	Run(c *Context, next Next) Signal
}

// Setup is optional on a Link: Build calls it once, before the first run,
// so the step can name itself and declare its params, the bars it needs,
// its state reset and nested steps.
type Setup interface {
	Setup(d *Decl)
}

// Role says what kind of step a link is. It drives the decision log (a
// trigger that fires makes a candle worth logging) and ExitOnTrendFlip
// (which reads the chain's direction steps).
type Role int

const (
	RoleStep      Role = iota // anything else
	RoleFilter                // may stop the chain (hours, spread, ATR range)
	RoleDirection             // sets or checks Dir (a trend)
	RoleTrigger               // fires on the side it sees an entry
	RoleStop                  // sets Stop
	RoleTarget                // sets Target
	RoleCheck                 // checks the answer on the way back
	RoleManage                // ManageOpen: the open position's steps
)

// Define makes a Link from a name, a role, the step itself and an optional
// setup (params, bars, reset). The built-in steps are made with it.
func Define(name string, role Role, run Step, setup func(d *Decl)) Link {
	return &defined{name: name, role: role, run: run, setup: setup}
}

type defined struct {
	name  string
	role  Role
	run   Step
	setup func(d *Decl)
}

func (l *defined) Run(c *Context, next Next) Signal { return l.run(c, next) }

func (l *defined) Setup(d *Decl) {
	d.Name(l.name)
	d.Role(l.role)
	if l.setup != nil {
		l.setup(d)
	}
}

// ParamDef is a tunable number of a chain (steps.P makes one). The chain's
// Params() lists every ParamDef its steps declare, so Config.Params, the
// CLI's --param and walk-forward tune them.
type ParamDef struct {
	Name    string
	Default float64
	// Grid is what walk-forward tries around centre (the default, or the
	// Config.Params value); nil = DefaultGrid.
	Grid func(centre float64) []float64
}

// GridTuner is optional for a Tunable strategy: Grid gives the values
// walk-forward tries for one param around centre; nil keeps DefaultGrid.
// Chains implement it, so their fractional numbers (a 2.5 RR) are tuned
// as fractions.
type GridTuner interface {
	Grid(name string, centre float64) []float64
}
