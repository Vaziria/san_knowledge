import { toJson } from '@bufbuild/protobuf';
import { anyUnpack, durationMs, StringValueSchema, timestampDate, ValueSchema, type Any } from '@bufbuild/protobuf/wkt';
import { Box, Code, type CodeProps } from '@chakra-ui/react';
import { createContext, useContext } from 'react';
import { LogLevel, RunStatus, type Run, type StepRun } from '../gen/san/tower/v1/workflow_pb';

// What the editor shows of runs: the run picked in the Runs panel marks, on
// each step of the canvas, what that step did (StepRunContext).

export const StepRunContext = createContext<Map<string, StepRun>>(new Map());
export const useStepRun = (stepId: string) => useContext(StepRunContext).get(stepId);

export const runStatus: Record<RunStatus, { label: string; palette: string }> = {
  [RunStatus.UNSPECIFIED]: { label: 'Unknown', palette: 'gray' },
  [RunStatus.RUNNING]: { label: 'Running', palette: 'blue' },
  [RunStatus.SUCCEEDED]: { label: 'Succeeded', palette: 'green' },
  [RunStatus.FAILED]: { label: 'Failed', palette: 'red' },
  [RunStatus.INTERRUPTED]: { label: 'Interrupted', palette: 'orange' },
};

// logLevel is how a log line of each type (the Log step's Type) is shown. A
// line from before lines had a type is info.
export const logLevel: Record<LogLevel, { label: string; color: string }> = {
  [LogLevel.UNSPECIFIED]: { label: 'info', color: 'fg' },
  [LogLevel.INFO]: { label: 'info', color: 'fg' },
  [LogLevel.SUCCESS]: { label: 'success', color: 'fg.success' },
  [LogLevel.WARNING]: { label: 'warning', color: 'fg.warning' },
  [LogLevel.FAILED]: { label: 'failed', color: 'fg.error' },
  [LogLevel.DEBUG]: { label: 'debug', color: 'fg.muted' },
};

// LogLine is one line a Log step wrote, in its type's colour with the type
// in front. It wraps, even inside a long word such as a URL.
export function LogLine({ text, level, ...props }: { text: string; level?: LogLevel } & CodeProps) {
  const l = logLevel[level ?? LogLevel.UNSPECIFIED];
  return (
    <Code whiteSpace="pre-wrap" overflowWrap="anywhere" color={l.color} {...props}>
      <Box as="span" display="inline-block" minW="8ch" fontWeight="semibold">
        {l.label}
      </Box>
      {text}
    </Code>
  );
}

// runDuration is how long a run took, or has been running.
export function runDuration(r: Run): string {
  if (!r.startedAt) return '';
  const end = r.finishedAt ? timestampDate(r.finishedAt) : new Date();
  return msText(end.getTime() - timestampDate(r.startedAt).getTime());
}

export function stepDuration(s: StepRun): string {
  return s.duration ? msText(durationMs(s.duration)) : '';
}

function msText(ms: number): string {
  if (ms < 1) return '<1 ms';
  if (ms < 1000) return `${Math.round(ms)} ms`;
  return `${(ms / 1000).toFixed(ms < 10000 ? 2 : 1)} s`;
}

// outputText is an output's value as text: a string as it is, an object as
// indented JSON.
export function outputText(a: Any): string {
  const s = anyUnpack(a, StringValueSchema);
  if (s) return s.value;
  const v = anyUnpack(a, ValueSchema);
  if (v) return JSON.stringify(toJson(ValueSchema, v), null, 2);
  return `(${a.typeUrl})`;
}
