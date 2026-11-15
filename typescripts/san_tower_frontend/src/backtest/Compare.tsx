import { Button, Heading, Stack, Table, Text } from '@chakra-ui/react';
import { useEffect, useMemo, useState } from 'react';
import { backtestAnalysis, errorText } from '../api';
import type { BacktestRun, BacktestRunSummary, BacktestStats } from '../gen/san/tower/v1/backtest_analysis_pb';
import { EquityChart, type Curve } from './EquityChart';
import { compareColors, money, pct, pf, rangeText, sec } from './format';

type Row = { label: string; get: (s: BacktestStats, r: BacktestRun) => string };

const rows: Row[] = [
  { label: 'Range (WIB)', get: (_, r) => rangeText(r.summary?.from, r.summary?.to) },
  { label: 'Trades', get: (s) => String(s.trades) },
  { label: 'Win rate', get: (s) => pct(s.winRate) },
  { label: 'Profit factor', get: (s) => pf(s.profitFactor) },
  { label: 'Average R', get: (s) => s.avgR.toFixed(2) },
  { label: 'Net profit', get: (s) => money(s.netProfit) },
  { label: 'Return', get: (s) => pct(s.returnPct, 2) },
  { label: 'Max drawdown', get: (s) => `${money(s.maxDrawdown)} (${pct(s.maxDrawdownPct, 2)})` },
  { label: 'Losing streak', get: (s) => String(s.longestLosingStreak) },
  { label: 'First half net', get: (s) => (s.firstHalf ? `${money(s.firstHalf.netProfit)} (${s.firstHalf.trades})` : '-') },
  { label: 'Second half net', get: (s) => (s.secondHalf ? `${money(s.secondHalf.netProfit)} (${s.secondHalf.trades})` : '-') },
  { label: 'Swap', get: (s) => money(s.swap) },
  { label: 'Out of sample net', get: (_, r) => (r.outOfSample ? `${money(r.outOfSample.netProfit)} (${r.outOfSample.trades})` : '-') },
];

// Compare shows ticked runs side by side: their stats in columns and their
// equity curves on one chart, as percent return so different balances line
// up.
export function Compare({ runs, onOpen }: { runs: BacktestRunSummary[]; onOpen: (path: string) => void }) {
  const [loaded, setLoaded] = useState<Record<string, BacktestRun>>({});
  const [error, setError] = useState('');
  const key = runs.map((r) => r.path).join('|');

  useEffect(() => {
    let live = true;
    for (const r of runs) {
      if (loaded[r.path]) continue;
      backtestAnalysis
        .getRun({ path: r.path, equityPoints: 1500 })
        .then((res) => live && res.run && setLoaded((m) => ({ ...m, [r.path]: res.run! })))
        .catch((err) => live && setError(errorText(err)));
    }
    return () => {
      live = false;
    };
  }, [key]);

  const list = runs.map((r) => loaded[r.path]).filter(Boolean) as BacktestRun[];
  const curves: Curve[] = useMemo(
    () =>
      list.map((r, i) => {
        const start = r.stats?.startBalance || r.equity[0]?.equity || 1;
        return {
          name: r.summary?.approach ?? '',
          color: compareColors[i % compareColors.length],
          points: r.equity.map((p) => ({ time: sec(p.time), value: (100 * (p.equity - start)) / start })),
        };
      }),
    [list.map((r) => r.summary?.path).join('|')],
  );

  return (
    <Stack gap="3" data-testid="compare">
      <Heading size="md">Compare {runs.length} runs</Heading>
      {error && <Text color="fg.error">{error}</Text>}
      {list.length < runs.length && <Text color="fg.muted">Reading the runs...</Text>}
      {list.length > 0 && (
        <>
          <Table.ScrollArea borderWidth="1px" rounded="md">
            <Table.Root size="sm">
              <Table.Header>
                <Table.Row>
                  <Table.ColumnHeader />
                  {list.map((r, i) => (
                    <Table.ColumnHeader key={r.summary?.path} textAlign="end">
                      <Button size="xs" variant="plain" color={compareColors[i % compareColors.length]} onClick={() => onOpen(r.summary!.path)}>
                        {r.summary?.approach} {r.summary?.relPath && `(${r.summary.relPath})`}
                      </Button>
                    </Table.ColumnHeader>
                  ))}
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {rows.map((row) => (
                  <Table.Row key={row.label}>
                    <Table.Cell color="fg.muted">{row.label}</Table.Cell>
                    {list.map((r) => (
                      <Table.Cell key={r.summary?.path} textAlign="end">
                        {r.stats ? row.get(r.stats, r) : '-'}
                      </Table.Cell>
                    ))}
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Root>
          </Table.ScrollArea>
          <Text textStyle="sm" color="fg.muted">
            Equity as return % from each run's starting balance.
          </Text>
          <EquityChart curves={curves} percent height={320} />
        </>
      )}
    </Stack>
  );
}
