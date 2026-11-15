import { Box } from '@chakra-ui/react';
import {
  AreaSeries,
  ColorType,
  createChart,
  LineSeries,
  TickMarkType,
  type DeepPartial,
  type ChartOptions,
  type IChartApi,
  type Time,
  type UTCTimestamp,
} from 'lightweight-charts';
import { useEffect, useRef } from 'react';
import { chartColors, WIB_OFFSET, wib } from './format';

// baseOptions are the charts' shared looks: the page's light or dark
// colours, and times in WIB on the axis and the crosshair.
export function baseOptions(): DeepPartial<ChartOptions> {
  const c = chartColors();
  const tick = (t: Time, type: TickMarkType) => {
    const d = new Date(((t as number) + WIB_OFFSET) * 1000);
    const p = (n: number) => String(n).padStart(2, '0');
    switch (type) {
      case TickMarkType.Year:
        return String(d.getUTCFullYear());
      case TickMarkType.Month:
        return d.toLocaleString('en-US', { month: 'short', timeZone: 'UTC' });
      case TickMarkType.DayOfMonth:
        return `${d.getUTCDate()} ${d.toLocaleString('en-US', { month: 'short', timeZone: 'UTC' })}`;
      default:
        return `${p(d.getUTCHours())}:${p(d.getUTCMinutes())}`;
    }
  };
  return {
    autoSize: true,
    layout: { background: { type: ColorType.Solid, color: c.bg }, textColor: c.text, attributionLogo: false },
    grid: { vertLines: { color: c.grid }, horzLines: { color: c.grid } },
    localization: { timeFormatter: (t: Time) => `${wib(t as number)} WIB` },
    // a small minBarSpacing lets a long curve (or months of M15 candles) fit
    // the width; the default stops at about two thousand points
    timeScale: { timeVisible: true, secondsVisible: false, tickMarkFormatter: tick, rightOffset: 4, minBarSpacing: 0.01 },
    rightPriceScale: { borderVisible: false },
  };
}

export type Curve = { name: string; color: string; points: { time: number; value: number }[] };

export type TimeRange = { from: number; to: number };

// EquityChart draws equity curves (and below them the drawdown, for one
// run) on a time axis that can follow the price chart's.
export function EquityChart({
  curves,
  drawdown,
  height = 260,
  range,
  percent,
}: {
  curves: Curve[];
  drawdown?: { time: number; value: number }[];
  height?: number;
  range?: TimeRange;
  percent?: boolean;
}) {
  const box = useRef<HTMLDivElement>(null);
  const chart = useRef<IChartApi>(undefined);

  useEffect(() => {
    if (!box.current) return;
    const c = chartColors();
    const ch = createChart(box.current, {
      ...baseOptions(),
      localization: {
        timeFormatter: (t: Time) => `${wib(t as number)} WIB`,
        priceFormatter: (v: number) => (percent ? `${v.toFixed(2)}%` : v.toFixed(2)),
      },
    });
    chart.current = ch;
    for (const cv of curves) {
      const s = ch.addSeries(LineSeries, { color: cv.color, lineWidth: 2, title: curves.length > 1 ? cv.name : '', priceLineVisible: false });
      s.setData(cv.points.map((p) => ({ time: p.time as UTCTimestamp, value: p.value })));
    }
    if (drawdown && drawdown.length) {
      const dd = ch.addSeries(
        AreaSeries,
        { lineColor: c.dd, topColor: 'rgba(220,38,38,0.05)', bottomColor: 'rgba(220,38,38,0.35)', lineWidth: 1, priceLineVisible: false, title: 'drawdown %' },
        1,
      );
      dd.setData(drawdown.map((p) => ({ time: p.time as UTCTimestamp, value: -p.value })));
      const panes = ch.panes();
      if (panes[1]) panes[1].setHeight(Math.round(height * 0.3));
    }
    ch.timeScale().fitContent();
    return () => {
      chart.current = undefined;
      ch.remove();
    };
  }, [curves, drawdown, height, percent]);

  useEffect(() => {
    if (range && chart.current) {
      try {
        chart.current.timeScale().setVisibleRange({ from: range.from as UTCTimestamp, to: range.to as UTCTimestamp });
      } catch {
        // a range outside the data: keep the view
      }
    }
  }, [range]);

  return <Box ref={box} h={`${height}px`} w="100%" data-testid="equity-chart" />;
}
