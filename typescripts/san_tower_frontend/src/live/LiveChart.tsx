import { Box } from '@chakra-ui/react';
import {
  CandlestickSeries,
  createChart,
  createSeriesMarkers,
  LineStyle,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type SeriesMarker,
  type Time,
  type UTCTimestamp,
} from 'lightweight-charts';
import { useEffect, useRef } from 'react';
import { baseOptions } from '../backtest/EquityChart';
import { chartColors, sec } from '../backtest/format';
import type { BacktestCandle, BacktestTrade } from '../gen/san/tower/v1/backtest_analysis_pb';

// tfSeconds are the chart timeframes' lengths.
export const tfSeconds: Record<string, number> = { M1: 60, M5: 300, M15: 900, M30: 1800, H1: 3600, H4: 14400, D1: 86400 };

export type Level = { price: number; color: 'entry' | 'stop' | 'target'; title: string };
export type Tick = { bid: number; time: number };

// LiveChart is a live candle chart: the closed candles given, and the
// candle still forming built from the ticks (bid) as they come, for the
// chart only (a paper session's approach never sees it). levels are price
// lines (open positions' entry, stop and target); trades get an arrow at the
// entry and a dot at the exit.
export function LiveChart({
  candles,
  timeframe,
  tick,
  levels = [],
  trades = [],
  height = 420,
}: {
  candles: BacktestCandle[];
  timeframe: string;
  tick?: Tick;
  levels?: Level[];
  trades?: BacktestTrade[];
  height?: number;
}) {
  const box = useRef<HTMLDivElement>(null);
  const chart = useRef<{
    ch: IChartApi;
    s: ISeriesApi<'Candlestick'>;
    lines: IPriceLine[];
    markers: ISeriesMarkersPluginApi<Time>;
    last?: { time: number; open: number; high: number; low: number; close: number };
  }>(undefined);
  const fitted = useRef('');

  useEffect(() => {
    if (!box.current) return;
    const c = chartColors();
    const ch = createChart(box.current, baseOptions());
    const s = ch.addSeries(CandlestickSeries, {
      upColor: c.up,
      downColor: c.down,
      borderVisible: false,
      wickUpColor: c.up,
      wickDownColor: c.down,
      priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
    });
    chart.current = { ch, s, lines: [], markers: createSeriesMarkers(s, []) };
    fitted.current = '';
    return () => {
      chart.current = undefined;
      ch.remove();
    };
  }, []);

  // the closed candles
  useEffect(() => {
    const cur = chart.current;
    if (!cur) return;
    const data = candles.map((k) => ({ time: Number(k.time) as UTCTimestamp, open: k.open, high: k.high, low: k.low, close: k.close }));
    cur.s.setData(data);
    const l = data[data.length - 1];
    cur.last = l ? { time: l.time as number, open: l.open, high: l.high, low: l.low, close: l.close } : undefined;
    const key = `${timeframe}:${data.length > 0}`;
    if (fitted.current !== key && data.length > 0) {
      fitted.current = key;
      const n = data.length;
      cur.ch.timeScale().setVisibleLogicalRange({ from: Math.max(0, n - 150), to: n + 5 });
    }
  }, [candles, timeframe]);

  // the forming candle, from the ticks
  useEffect(() => {
    const cur = chart.current;
    const step = tfSeconds[timeframe];
    if (!cur || !tick || !step || !tick.bid) return;
    const at = Math.floor(tick.time / step) * step;
    const last = cur.last;
    if (last && at < last.time) return;
    const bar =
      last && last.time === at
        ? { ...last, high: Math.max(last.high, tick.bid), low: Math.min(last.low, tick.bid), close: tick.bid }
        : { time: at, open: last?.close ?? tick.bid, high: tick.bid, low: tick.bid, close: tick.bid };
    cur.last = bar;
    cur.s.update({ ...bar, time: bar.time as UTCTimestamp });
  }, [tick, timeframe]);

  // the levels
  useEffect(() => {
    const cur = chart.current;
    if (!cur) return;
    for (const l of cur.lines) cur.s.removePriceLine(l);
    const c = chartColors();
    cur.lines = levels
      .filter((l) => l.price > 0)
      .map((l) =>
        cur.s.createPriceLine({
          price: l.price,
          color: c[l.color],
          title: l.title,
          lineStyle: l.color === 'entry' ? LineStyle.Solid : LineStyle.Dashed,
          lineWidth: 1,
          axisLabelVisible: true,
        }),
      );
  }, [levels]);

  // the trades' markers, on the candles there are
  useEffect(() => {
    const cur = chart.current;
    if (!cur) return;
    const c = chartColors();
    const times = candles.map((k) => Number(k.time));
    const at = (t: number) => {
      let lo = 0;
      let hi = times.length - 1;
      if (hi < 0 || t < times[0]) return undefined;
      while (lo < hi) {
        const mid = (lo + hi + 1) >> 1;
        if (times[mid] <= t) lo = mid;
        else hi = mid - 1;
      }
      return times[lo];
    };
    const m: SeriesMarker<Time>[] = [];
    for (const t of trades) {
      const win = t.net > 0;
      const buy = t.side === 'buy';
      const a = at(sec(t.openTime));
      const b = at(sec(t.closeTime));
      if (a !== undefined)
        m.push({ time: a as UTCTimestamp, position: buy ? 'belowBar' : 'aboveBar', shape: buy ? 'arrowUp' : 'arrowDown', color: win ? c.win : c.loss, text: buy ? 'B' : 'S' });
      if (b !== undefined)
        m.push({ time: b as UTCTimestamp, position: 'atPriceMiddle', price: t.exit, shape: 'circle', color: win ? c.win : c.loss, size: 0.6 });
    }
    m.sort((x, y) => (x.time as number) - (y.time as number));
    cur.markers.setMarkers(m);
  }, [trades, candles]);

  return <Box ref={box} h={`${height}px`} w="100%" data-testid="live-chart" />;
}
