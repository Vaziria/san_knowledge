// Package san_backtest backtests trading approaches written in Go on
// Exness candles (through san_exness, cached on disk) or CSV files, with
// Exness's real costs, risk-% sizing and a shared-account portfolio. Run
// and RunPortfolio return a Result with stats, trades, the equity curve
// and the range that really ran; Save writes the CSV and PNG reports.
// Everything is UTC; there is no look-ahead: a strategy sees only candles
// closed at the current step. See README.md for the design and its limits.
package san_backtest
