import { createSystem, defaultConfig, defineConfig } from '@chakra-ui/react';
import { useCallback, useEffect, useLayoutEffect, useState } from 'react';

export type Mode = 'light' | 'dark';

// [light, dark]. The graph reads these values directly (cytoscape cannot use
// CSS variables); the rest of the UI gets them through the Chakra tokens below.
// The slots s1–s8 are validated all-pairs, so neighbouring colours stay apart.
const PALETTE = {
  page: ['#f9f9f7', '#0d0d0d'],
  surface: ['#fcfcfb', '#1a1a19'],
  surface2: ['#f3f2ee', '#232321'],
  ink: ['#0b0b0b', '#ffffff'],
  ink2: ['#52514e', '#c3c2b7'],
  muted: ['#898781', '#898781'],
  grid: ['#e1e0d9', '#2c2c2a'],
  axis: ['#c3c2b7', '#383835'],
  border: ['rgba(11, 11, 11, 0.10)', 'rgba(255, 255, 255, 0.10)'],
  accent: ['#2a78d6', '#3987e5'],
  s1: ['#2a78d6', '#3987e5'],
  s2: ['#eb6834', '#d95926'],
  s3: ['#1baf7a', '#199e70'],
  s4: ['#eda100', '#c98500'],
  s5: ['#e87ba4', '#d55181'],
  s6: ['#008300', '#008300'],
  s7: ['#4a3aa7', '#9085e9'],
  s8: ['#e34948', '#e66767'],
  other: ['#898781', '#898781'],
} as const;

export type Palette = Record<keyof typeof PALETTE, string>;

export const palette = (mode: Mode): Palette =>
  Object.fromEntries(Object.entries(PALETTE).map(([k, v]) => [k, v[mode === 'dark' ? 1 : 0]])) as Palette;

const tok = (name: keyof typeof PALETTE) => ({ value: { base: PALETTE[name][0], _dark: PALETTE[name][1] } });
const fixed = (value: string) => ({ value });
const font = 'system-ui, -apple-system, "Segoe UI", sans-serif';

// Chakra's own tokens are remapped to the palette, so its components (buttons,
// inputs, dialogs, tables) use the same warm neutrals as the graph. Primary
// actions use colorPalette="accent".
export const system = createSystem(defaultConfig, defineConfig({
  globalCss: {
    'html, body, #root': { height: '100%' },
    body: { fontSize: '14px', lineHeight: '1.45' },
  },
  theme: {
    tokens: {
      fonts: {
        body: fixed(font),
        heading: fixed(font),
        mono: fixed('ui-monospace, "Cascadia Code", Consolas, monospace'),
      },
    },
    semanticTokens: {
      colors: {
        bg: { DEFAULT: tok('page'), panel: tok('surface'), subtle: tok('surface2'), muted: tok('surface2'), emphasized: tok('grid') },
        fg: { DEFAULT: tok('ink'), muted: tok('ink2'), subtle: tok('muted') },
        border: { DEFAULT: tok('border'), muted: tok('grid'), emphasized: tok('axis') },
        gray: {
          solid: tok('ink'), contrast: tok('page'), fg: tok('ink'),
          subtle: tok('surface2'), muted: tok('grid'), emphasized: tok('axis'), focusRing: tok('accent'),
        },
        accent: {
          solid: tok('accent'), contrast: fixed('#ffffff'), fg: tok('accent'),
          subtle: fixed('color-mix(in srgb, #2a78d6 12%, transparent)'),
          muted: fixed('color-mix(in srgb, #2a78d6 22%, transparent)'),
          emphasized: fixed('color-mix(in srgb, #2a78d6 32%, transparent)'),
          focusRing: tok('accent'),
        },
        surface: tok('surface'),
      },
    },
  },
}));

const THEME_KEY = 'knowledge-graph-theme';

// ?theme= wins for this page load; otherwise the stored choice; otherwise the system.
function chosenTheme(): Mode | null {
  const param = new URLSearchParams(location.search).get('theme');
  if (param === 'light' || param === 'dark') return param;
  try {
    const stored = localStorage.getItem(THEME_KEY);
    return stored === 'light' || stored === 'dark' ? stored : null;
  } catch {
    return null;
  }
}

const darkQuery = () => matchMedia('(prefers-color-scheme: dark)');

export function useColorMode(): [Mode, () => void] {
  const [choice, setChoice] = useState(chosenTheme);
  const [systemDark, setSystemDark] = useState(() => darkQuery().matches);
  useEffect(() => {
    const mq = darkQuery();
    const onChange = () => setSystemDark(mq.matches);
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, []);
  const mode: Mode = choice ?? (systemDark ? 'dark' : 'light');
  useLayoutEffect(() => {
    const root = document.documentElement;
    root.classList.toggle('dark', mode === 'dark');
    root.classList.toggle('light', mode === 'light');
    root.style.colorScheme = mode;
  }, [mode]);
  const toggle = useCallback(() => {
    const next: Mode = mode === 'dark' ? 'light' : 'dark';
    try { localStorage.setItem(THEME_KEY, next); } catch { /* storage unavailable */ }
    setChoice(next);
  }, [mode]);
  return [mode, toggle];
}
