import { Clapperboard, Mountain, RadioTower, Shapes, SlidersHorizontal, Zap } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useBehaviours, type Behaviour } from '../behaviours';
import { CAMERA_DIRECTIONS, isCameraDirection } from '../camera';
import { environments, figureEnvironment } from '../previews';
import {
  environmentOf,
  KUWAHARA_MAX_RADIUS,
  setCamera,
  setEnvironment,
  setFigure,
  setKuwahara,
  setStory,
  setTheme,
  useSettings,
} from '../settings';
import { stories } from '../stories';
import { follow } from '../stream';
import { themes } from '../theme';
import { Action, Choice, Field, RadioList, Select, Slider, Toggle } from './controls';
import { figureTree } from './figureTree';
import './panel.css';
import { SearchTree } from './SearchTree';
import { StreamTab } from './StreamTab';
import { Card } from './ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';

// The control panel, top right over the canvas: vertical tabs on its right
// edge, by the screen's, for the figures (a searchable tree), the shown
// figure's behaviours, the story, the environment, the look (camera, theme
// and Kuwahara) and live streaming to YouTube (StreamTab.tsx). Each control
// reads its setting with useSettings and changes it with a setter from
// settings.ts; main.tsx passes every change on to the stage.
//
// The width is fixed: a menu is as wide as the option it shows, so a panel
// sized by its content would jump with every pick. The card is opaque: over a
// dark scene a see-through one turned grey and hid the controls' light
// borders. The tabs stay mounted while hidden, so typed values and a search
// survive a switch.
const TABS = [
  { value: 'figures', label: 'Figures', icon: Shapes },
  { value: 'behaviours', label: 'Behaviours', icon: Zap },
  { value: 'story', label: 'Story', icon: Clapperboard },
  { value: 'environments', label: 'Environments', icon: Mountain },
  { value: 'settings', label: 'Settings', icon: SlidersHorizontal },
  { value: 'stream', label: 'Stream', icon: RadioTower },
] as const;

type Tab = (typeof TABS)[number]['value'];

// The open tab is kept in this browser, so it survives the reloads a code
// change brings. It says nothing about the scene, so it is not in the URL.
const TAB_KEY = 'animation-panel-tab';

function savedTab(): Tab {
  try {
    const found = TABS.find((t) => t.value === localStorage.getItem(TAB_KEY));
    if (found) return found.value;
  } catch {
    // storage can be blocked; start on Figures
  }
  return 'figures';
}

export function Panel() {
  const [tab, setTab] = useState<Tab>(savedTab);
  const open = (value: unknown) => {
    const found = TABS.find((t) => t.value === value);
    if (!found) return;
    setTab(found.value);
    try {
      localStorage.setItem(TAB_KEY, found.value);
    } catch {
      // not remembered, that's all
    }
  };
  // inert:hidden hides a panel the moment another tab opens; Base UI keeps it
  // shown until its exit animation ends, and side by side they would flicker.
  const panel = 'min-w-0 overflow-y-auto p-3 scrollbar-thin inert:hidden';
  return (
    <Card
      size="sm"
      className="fixed top-4 right-4 max-h-[calc(100vh-2rem)] w-96 max-w-[calc(100vw-2rem)] gap-0 py-0 shadow-md"
    >
      {/* flex-row-reverse puts the rail on the right while it stays first in
          the page, so Tab and screen readers reach the tabs before a panel. */}
      <Tabs orientation="vertical" value={tab} onValueChange={open} className="min-h-0 flex-1 flex-row-reverse gap-0">
        <TabsList className="m-2 ml-0 w-21 shrink-0 self-start">
          {TABS.map(({ value, label, icon: Icon }) => (
            <TabsTrigger
              key={value}
              value={value}
              className="h-auto flex-none flex-col gap-1 px-1 py-2 text-[11px] group-data-vertical/tabs:justify-center"
            >
              <Icon />
              {label}
            </TabsTrigger>
          ))}
        </TabsList>
        {/* A column, so the tree takes the height left under the search box. */}
        <TabsContent value="figures" keepMounted className={`${panel} flex flex-col`}>
          <FiguresTab />
        </TabsContent>
        <TabsContent value="behaviours" keepMounted className={panel}>
          <BehavioursTab />
        </TabsContent>
        <TabsContent value="story" keepMounted className={panel}>
          <StoryTab />
        </TabsContent>
        <TabsContent value="environments" keepMounted className={panel}>
          <EnvironmentsTab />
        </TabsContent>
        <TabsContent value="settings" keepMounted className={panel}>
          <SettingsTab />
        </TabsContent>
        <TabsContent value="stream" keepMounted className={panel}>
          <StreamTab />
        </TabsContent>
      </Tabs>
    </Card>
  );
}

function Note({ children }: { children: ReactNode }) {
  return <p className="text-xs text-muted-foreground">{children}</p>;
}

// The figures in their folders under src/figures, searchable. The tree takes
// the panel's height and scrolls under the search box. A story brings its own
// figures, so the tree rests while it plays and keeps its pick.
function FiguresTab() {
  const figure = useSettings((s) => s.figure);
  const story = useSettings((s) => s.story);
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      {story && <Note>The story {story} brings its own figures. The one picked here comes back when it stops.</Note>}
      <SearchTree label="Figures" nodes={figureTree} value={figure} disabled={story !== null} onChange={setFigure} />
    </div>
  );
}

// The shown figure's behaviours, or a note when its spec has none (the boat,
// the trees) or a story plays.
function BehavioursTab() {
  const count = useBehaviours((s) => s.behaviours.length);
  const shown = useSettings((s) => s.story ?? s.figure);
  if (count === 0) return <Note>{shown} has no behaviours in its spec.</Note>;
  return <BehaviourControls />;
}

// "none" shows the figure picked in Figures; a story brings its own figures
// and environment, so those two tabs rest while it plays.
function StoryTab() {
  const story = useSettings((s) => s.story);
  const options = [
    { value: '', label: 'none', hint: 'the picked figure' },
    ...Object.keys(stories).map((name) => ({ value: name, label: name })),
  ];
  return <RadioList label="Story" value={story ?? ''} options={options} onChange={(name) => setStory(name || null)} />;
}

// Shows the figure's own environment until one is picked; a pick stays when
// the figure changes.
function EnvironmentsTab() {
  const environment = useSettings(environmentOf);
  const own = useSettings((s) => figureEnvironment(s.figure));
  const story = useSettings((s) => s.story);
  const options = Object.keys(environments).map((name) => ({
    value: name,
    label: name,
    hint: name === own ? "figure's own" : undefined,
  }));
  return (
    <div className="flex flex-col gap-3">
      {story && <Note>The story {story} plays in its own environment.</Note>}
      <RadioList
        label="Environment"
        value={environment}
        options={options}
        disabled={story !== null}
        onChange={setEnvironment}
      />
    </div>
  );
}

// How everything looks: the way the camera looks at the figure, the theme's
// colours and the painterly filter. Rows share three columns (label, control,
// value), so labels and controls line up.
function SettingsTab() {
  return (
    <div className="grid grid-cols-[max-content_1fr_max-content] items-center gap-2">
      <CameraSelect />
      <ThemeSelect />
      <KuwaharaControls />
    </div>
  );
}

// The shown figure's behaviours, named as in its spec: a button for each,
// beside an input for each of its arguments. The first one run stops the
// demo, so the figure does only what it is told; Restart demo brings the demo
// back. A value the figure can't read (an unknown chord) shows its error below.
// While the dev server draws the stream ("stream only"), each also goes there.
function BehaviourControls() {
  const list = useBehaviours((s) => s.behaviours);
  const restart = useBehaviours((s) => s.restart);
  if (list.length === 0) return null;
  return (
    <div className="grid grid-cols-[max-content_1fr_max-content] items-center gap-2">
      {/* Keyed by the call's shape, so typed values survive a theme change
          but not a switch to a figure whose behaviour takes other arguments. */}
      {list.map((behaviour) => (
        <BehaviourRow key={signature(behaviour)} behaviour={behaviour} />
      ))}
      <Action
        label="Restart demo"
        wide
        onClick={() => {
          restart();
          follow({ restart: true });
        }}
      />
    </div>
  );
}

// "Speech(text)", "Hold(figure)", "Flap()".
function signature(behaviour: Behaviour): string {
  return `${behaviour.name}(${(behaviour.params ?? []).map((p) => p.name).join(', ')})`;
}

function BehaviourRow({ behaviour }: { behaviour: Behaviour }) {
  const params = behaviour.params ?? [];
  const [values, setValues] = useState(() => params.map((p) => p.value));
  const [error, setError] = useState<string | null>(null);
  const set = (i: number, value: string) => setValues((old) => old.map((v, j) => (j === i ? value : v)));
  const run = () => {
    try {
      behaviour.run(...values);
      setError(null);
      follow({ behaviour: { name: behaviour.name, args: values } }); // and in the stream the dev server draws
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };
  return (
    // The button in the label column, the arguments' inputs beside it.
    <div className="col-span-full grid grid-cols-subgrid items-center gap-y-1">
      <Action label={behaviour.name} onClick={run} />
      <div className="col-span-2 flex min-w-0 gap-1.5">
        {params.map((p, i) =>
          p.options ? (
            <Choice key={p.name} name={p.name} value={values[i]} options={p.options} onChange={(v) => set(i, v)} />
          ) : (
            <Field key={p.name} name={p.name} value={values[i]} onChange={(v) => set(i, v)} onSubmit={run} />
          ),
        )}
      </div>
      {error && <p className="col-span-full text-xs font-medium text-destructive">{error}</p>}
    </div>
  );
}

// The camera's direction (camera.ts): the figure's own view, a side, or top.
// It says "free (dragged)" once the view is dragged away, so the same
// direction can be picked again.
function CameraSelect() {
  const camera = useSettings((s) => s.camera);
  const options = CAMERA_DIRECTIONS.map(({ value, label }) => ({ value, label }));
  return <Select label="Camera" value={camera} options={options} onChange={(v) => isCameraDirection(v) && setCamera(v)} />;
}

function ThemeSelect() {
  const theme = useSettings((s) => s.theme);
  const options = Object.keys(themes).map((name) => ({ value: name, label: name }));
  return <Select label="Theme" value={theme} options={options} onChange={setTheme} />;
}

// The painterly filter: on or off, and its radius, greyed out while it is off.
function KuwaharaControls() {
  const { on, radius } = useSettings((s) => s.kuwahara);
  return (
    <>
      <Toggle label="Kuwahara" on={on} onChange={(on) => setKuwahara({ on })} />
      <Slider
        label="Radius"
        value={radius}
        min={1}
        max={KUWAHARA_MAX_RADIUS}
        unit="px"
        disabled={!on}
        onChange={(radius) => setKuwahara({ radius })}
      />
    </>
  );
}
