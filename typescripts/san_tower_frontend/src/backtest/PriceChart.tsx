import { Box, Text } from '@chakra-ui/react';
import {
  CandlestickSeries,
  createChart,
  createSeriesMarkers,
  LineSeries,
  LineStyle,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  type LineData,
  type SeriesMarker,
  type Time,
  type UTCTimestamp,
} from 'lightweight-charts';
import { useEffect, useMemo, useRef, useState } from 'react';
import { backtestAnalysis, errorText } from '../api';
import type { BacktestCandle, BacktestTrade } from '../gen/san/tower/v1/backtest_analysis_pb';
import { baseOptions, type TimeRange } from './EquityChart';
import { chartColors, price, sec } from './format';

// barAt is the open time of the candle holding t: the newest candle that
// opened at or before it.
function barAt(times: number[], t: number) {
  let lo = 0;
  let hi = times.length - 1;
  if (hi < 0 || t < times[0]) return times[0];
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (times[mid] <= t) lo = mid;
    else hi = mid - 1;
  }
  return times[lo];
}

// segments is one line per trade, from its entry candle to its exit
// candle, at the level level() gives. A line series joins every point (it
// draws over whitespace), so each trade's last point has a transparent
// colour: a point's colour is the one of the stretch after it.
function segments(times: number[], trades: BacktestTrade[], level: (t: BacktestTrade) => number): LineData[] {
  const out = new Map<number, LineData>();
  const sorted = [...trades].sort((x, y) => sec(x.openTime) - sec(y.openTime));
  for (const t of sorted) {
    const v = level(t);
    if (!v) continue;
    const a = barAt(times, sec(t.openTime));
    const b = barAt(times, sec(t.closeTime));
    let last: LineData | undefined;
    for (let i = times.indexOf(a); i >= 0 && i < times.length && times[i] <= b; i++) {
      if (out.has(times[i])) continue; // the lane's previous trade ends on this candle
      last = { time: times[i] as UTCTimestamp, value: v };
      out.set(times[i], last);
    }
    if (last) last.color = 'transparent';
  }
  return [...out.values()].sort((x, y) => (x.time as number) - (y.time as number));
}

// lanes splits trades so no two in one lane are open on the same candle:
// several positions can be open at once (San Backtest's MaxOpen, hedging,
// or several legs), and one line series can draw only one level a candle,
// so each lane gets its own stop, target and entry lines.
function lanes(times: number[], trades: BacktestTrade[]): BacktestTrade[][] {
  const sorted = [...trades].sort((x, y) => sec(x.openTime) - sec(y.openTime) || x.index - y.index);
  const ends: number[] = [];
  const out: BacktestTrade[][] = [];
  for (const t of sorted) {
    const a = barAt(times, sec(t.openTime));
    const b = barAt(times, sec(t.closeTime));
    let k = ends.findIndex((e) => e < a);
    if (k < 0) {
      k = ends.length;
      ends.push(b);
      out.push([]);
    } else ends[k] = b;
    out[k].push(t);
  }
  return out;
}

// PriceChart is a report's candles (candles.csv, in the timeframe picked)
// with every trade drawn: an arrow at the entry, a dot at the exit, green
// for a win and red for a loss, the stop and target as dashed lines while
// it is open. The selected trade gets its levels as price lines and the
// chart jumps to it. A click on a trade selects it; a click elsewhere picks
// that candle (for the decision log).
export function PriceChart({
  path,
  timeframe,
  trades,
  selected,
  onSelect,
  onCandle,
  onRange,
  height = 460,
}: {
  path: string;
  timeframe: string;
  trades: BacktestTrade[];
  selected?: BacktestTrade;
  onSelect: (t: BacktestTrade) => void;
  onCandle: (time: number) => void;
  onRange?: (r: TimeRange) => void;
  height?: number;
}) {
  const box = useRef<HTMLDivElement>(null);
  const [candles, setCandles] = useState<BacktestCandle[]>();
  const [error, setError] = useState('');
  const [truncated, setTruncated] = useState(false);
  const chart = useRef<{ ch: IChartApi; s: ISeriesApi<'Candlestick'>; lines: IPriceLine[] }>(undefined);
  const latest = useRef({ onSelect, onCandle, onRange, trades });
  latest.current = { onSelect, onCandle, onRange, trades };

  useEffect(() => {
    let live = true;
    setCandles(undefined);
    setError('');
    backtestAnalysis
      .getCandles({ path, timeframe })
      .then((r) => {
        if (!live) return;
        setCandles(r.candles);
        setTruncated(r.truncated);
      })
      .catch((err) => live && setError(errorText(err)));
    return () => {
      live = false;
    };
  }, [path, timeframe]);

  const times = useMemo(() => (candles ?? []).map((c) => Number(c.time)), [candles]);

  useEffect(() => {
    if (!box.current || !candles || candles.length === 0) return;
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
    s.setData(candles.map((k) => ({ time: Number(k.time) as UTCTimestamp, open: k.open, high: k.high, low: k.low, close: k.close })));
    const line = (color: string, style: LineStyle) =>
      ch.addSeries(LineSeries, { color, lineWidth: 1, lineStyle: style, priceLineVisible: false, lastValueVisible: false, crosshairMarkerVisible: false });
    for (const lane of lanes(times, trades)) {
      line(c.stop, LineStyle.Dashed).setData(segments(times, lane, (t) => t.stop));
      line(c.target, LineStyle.Dashed).setData(segments(times, lane, (t) => t.target));
      line(c.entry, LineStyle.Dotted).setData(segments(times, lane, (t) => t.entry));
    }
    const markers: SeriesMarker<Time>[] = [];
    for (const t of trades) {
      const win = t.net > 0;
      const buy = t.side === 'buy';
      markers.push({
        time: barAt(times, sec(t.openTime)) as UTCTimestamp,
        position: buy ? 'belowBar' : 'aboveBar',
        shape: buy ? 'arrowUp' : 'arrowDown',
        color: win ? c.win : c.loss,
        id: `e${t.index}`,
        text: (buy ? 'B' : 'S') + (t.orderType === 'limit' ? ' lmt' : t.orderType === 'stop' ? ' stp' : ''),
      });
      markers.push({
        time: barAt(times, sec(t.closeTime)) as UTCTimestamp,
        position: 'atPriceMiddle',
        price: t.exit,
        shape: 'circle',
        color: win ? c.win : c.loss,
        id: `x${t.index}`,
        size: 0.6,
      });
    }
    markers.sort((a, b) => (a.time as number) - (b.time as number));
    createSeriesMarkers(s, markers);
    ch.subscribeClick((p) => {
      if (p.time === undefined) return;
      const at = p.time as number;
      const id = typeof p.hoveredObjectId === 'string' ? p.hoveredObjectId : '';
      const ts = latest.current.trades;
      const hit =
        (id && ts.find((t) => `e${t.index}` === id || `x${t.index}` === id)) ||
        ts.find((t) => barAt(times, sec(t.openTime)) <= at && at <= barAt(times, sec(t.closeTime)));
      if (hit) latest.current.onSelect(hit);
      latest.current.onCandle(at);
    });
    ch.timeScale().subscribeVisibleTimeRangeChange((r) => {
      if (r) latest.current.onRange?.({ from: r.from as number, to: r.to as number });
    });
    chart.current = { ch, s, lines: [] };
    return () => {
      chart.current = undefined;
      ch.remove();
    };
  }, [candles, times, trades]);

  // The selected trade: its levels as price lines, and the view on it.
  useEffect(() => {
    const cur = chart.current;
    if (!cur) return;
    for (const l of cur.lines) cur.s.removePriceLine(l);
    cur.lines = [];
    if (!selected || times.length < 2) return;
    const c = chartColors();
    const add = (p: number, color: string, title: string, style = LineStyle.Solid) =>
      p && cur.lines.push(cur.s.createPriceLine({ price: p, color, title, lineStyle: style, lineWidth: 1, axisLabelVisible: true }));
    add(selected.entry, c.entry, `entry ${price(selected.entry)}`);
    add(selected.stop, c.stop, 'stop', LineStyle.Dashed);
    if (selected.finalStop && selected.finalStop !== selected.stop) add(selected.finalStop, c.stop, 'final stop', LineStyle.Dotted);
    add(selected.target, c.target, 'target', LineStyle.Dashed);
    add(selected.exit, selected.net > 0 ? c.win : c.loss, `exit ${selected.reason}`);
    const step = times[1] - times[0];
    const a = sec(selected.openTime);
    const b = sec(selected.closeTime);
    const pad = Math.max(40 * step, (b - a) * 0.6);
    cur.ch.timeScale().setVisibleRange({ from: (a - pad) as UTCTimestamp, to: (b + pad) as UTCTimestamp });
  }, [selected, candles, times]);

  if (error) return <Text color="fg.error">{error}</Text>;
  return (
    <Box position="relative">
      {!candles && (
        <Text color="fg.muted" position="absolute" p="3">
          Loading candles...
        </Text>
      )}
      {candles && candles.length === 0 && <Text color="fg.muted">No candles in this report.</Text>}
      {truncated && (
        <Text color="fg.warning" textStyle="xs">
          Only the newest candles are shown: pick a higher timeframe for the whole range.
        </Text>
      )}
      <Box ref={box} h={`${height}px`} w="100%" data-testid="price-chart" />
    </Box>
  );
}
