# dip_fade_m1 through san_backtest

Buy after a 3-minute drop larger than 2 x M1 ATR(14) x sqrt(3), SL 1 / TP 2 M1 ATR, entries 20:00-23:58 WIB, closed at 00:00 WIB, one position at a time. $10,000, 1% risk, leverage 2000. Harsh = spread $0.40 + $0.10 slippage. Walk-forward 3m:1m tunes sl and tp.

| run | trades | win % | PF | net % | max DD % | avg R |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| real | 715 | 37.1% | 1.01 | +3.0 | 35.2 | +0.015 |
| walk-forward OOS | 602 | 37.5% | 1.01 | +2.0 | 47.7 | +0.015 |
| harsh | 713 | 36.7% | 0.76 | -60.2 | 68.2 | -0.123 |
| 2024 | 171 | 38.0% | 0.99 | -1.0 | 11.4 | +0.002 |
| 2025 | 303 | 33.7% | 0.87 | -22.1 | 29.7 | -0.075 |
| 2026 | 241 | 40.7% | 1.20 | +33.7 | 10.0 | +0.137 |
