import {
  Camera,
  Clapperboard,
  Layers,
  Mountain,
  Music,
  PanelRightClose,
  PanelRightOpen,
  RadioTower,
  Shapes,
  SlidersHorizontal,
  Zap,
} from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useBehaviours, type Behaviour } from '../behaviours';
import { environments, figureEnvironment, TIMES_OF_DAY } from '../previews';
import {
  environmentOf,
  KUWAHARA_MAX_RADIUS,
  setEnvironment,
  setFigure,
  setKuwahara,
  setStory,
  setTerrain,
  setTime,
  useSettings,
} from '../settings';
import { stories } from '../stories';
import { follow } from '../stream';
import { AudioTab } from './AudioTab';
import { CameraTab } from './CameraTab';
import { ChatControls } from './ChatControls';
import { Action, Choice, Field, RadioList, Slider, Toggle } from './controls';
import { figureTree, fromTreeValue, treeValue } from './figureTree';
import './panel.css';
import { SearchTree } from './SearchTree';
import { StreamTab } from './StreamTab';
import { terrainTree } from './terrainTree';
import { Button } from './ui/button';
import { Card } from './ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';

// The control panel, top right over the canvas: vertical tabs on its right
// edge, by the screen's, for the figures (a searchable tree), the terrains
// (another, like it), the shown figure's behaviours, the story, the
// environment, the camera (its view, and
// walking it; CameraTab.tsx), the sound (AudioTab.tsx), the look (the
// Kuwahara filter) and live streaming to YouTube (StreamTab.tsx). Each control
// reads its setting with useSettings and changes it with a setter from
// settings.ts; main.tsx passes every change on to the stage.
//
// The panel folds down to its rail of tabs, to see the whole scene: click
// the open tab again, or Collapse under the rail; any tab opens it again.
//
// The width is fixed: a menu is as wide as the option it shows, so a panel
// sized by its content would jump with every pick. The card is opaque: over a
// dark scene a see-through one turned grey and hid the controls' light
// borders. The tabs stay mounted while hidden, so typed values and a search
// survive a switch.
const TABS = [
  { value: 'figures', label: 'Figures', icon: Shapes },
  { value: 'terrains', label: 'Terrains', icon: Layers },
  { value: 'behaviours', label: 'Behaviours', icon: Zap },
  { value: 'story', label: 'Story', icon: Clapperboard },
  { value: 'environments', label: 'Environments', icon: Mountain },
  { value: 'camera', label: 'Camera', icon: Camera },
  { value: 'audio', label: 'Audio', icon: Music },
  { value: 'settings', label: 'Settings', icon: SlidersHorizontal },
  { value: 'stream', label: 'Stream', icon: RadioTower },
] as const;

type Tab = (typeof TABS)[number]['value'];

// The open tab, and whether the panel is folded, are kept in this browser,
// so they survive the reloads a code change brings. They say nothing about
// the scene, so they are not in the URL.
const TAB_KEY = 'animation-panel-tab';
const FOLDED_KEY = 'animation-panel-folded';

function savedTab(): Tab {
  try {
    const found = TABS.find((t) => t.value === localStorage.getItem(TAB_KEY));
    if (found) return found.value;
  } catch {
    // storage can be blocked; start on Figures
  }
  return 'figures';
}

function savedFolded(): boolean {
  try {
    return localStorage.getItem(FOLDED_KEY) === 'yes';
  } catch {
    return false;
  }
}

function remember(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // not remembered, that's all
  }
}

export function Panel() {
  const [tab, setTab] = useState<Tab>(savedTab);
  const [folded, setFolded] = useState(savedFolded);
  const fold = (on: boolean) => {
    setFolded(on);
    remember(FOLDED_KEY, on ? 'yes' : 'no');
  };
  // Another tab opens, and unfolds the panel.
  const open = (value: unknown) => {
    const found = TABS.find((t) => t.value === value);
    if (!found) return;
    setTab(found.value);
    remember(TAB_KEY, found.value);
    fold(false);
  };
  // inert:hidden hides a panel the moment another tab opens; Base UI keeps it
  // shown until its exit animation ends, and side by side they would flicker.
  const panel = 'min-w-0 overflow-y-auto p-3 scrollbar-thin inert:hidden group-data-[folded=true]/panel:hidden';
  return (
    // Folded, the card is as wide as its rail, and the tabs' panels are hidden
    // but stay mounted, so what was typed or searched is still there.
    <Card
      size="sm"
      data-folded={folded}
      className={`group/panel fixed top-4 right-4 max-h-[calc(100vh-2rem)] max-w-[calc(100vw-2rem)] gap-0 py-0 shadow-md ${folded ? 'w-auto' : 'w-96'}`}
    >
      {/* flex-row-reverse puts the rail on the right while it stays first in
          the page, so Tab and screen readers reach the tabs before a panel. */}
      <Tabs orientation="vertical" value={tab} onValueChange={open} className="min-h-0 flex-1 flex-row-reverse gap-0">
        <div className={`m-2 flex w-21 shrink-0 flex-col gap-1 self-start ${folded ? '' : 'ml-0'}`}>
          <TabsList className="w-full">
            {TABS.map(({ value, label, icon: Icon }) => (
              <TabsTrigger
                key={value}
                value={value}
                // The open tab, clicked again, folds the panel or unfolds it.
                onClick={() => value === tab && fold(!folded)}
                title={value === tab && !folded ? `${label}: click again to collapse the panel` : undefined}
                className="h-auto flex-none flex-col gap-1 px-1 py-2 text-[11px] group-data-vertical/tabs:justify-center"
              >
                <Icon />
                {label}
              </TabsTrigger>
            ))}
          </TabsList>
          <Button
            variant="ghost"
            aria-expanded={!folded}
            className="h-auto flex-col gap-1 px-1 py-2 text-[11px] font-normal text-muted-foreground"
            onClick={() => fold(!folded)}
          >
            {folded ? <PanelRightOpen /> : <PanelRightClose />}
            {folded ? 'Expand' : 'Collapse'}
          </Button>
        </div>
        {/* A column, so the tree takes the height left under the search box. */}
        <TabsContent value="figures" keepMounted className={`${panel} flex flex-col`}>
          <FiguresTab />
        </TabsContent>
        <TabsContent value="terrains" keepMounted className={`${panel} flex flex-col`}>
          <TerrainsTab />
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
        <TabsContent value="camera" keepMounted className={panel}>
          <CameraTab />
        </TabsContent>
        <TabsContent value="audio" keepMounted className={panel}>
          <AudioTab />
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
// figures, so the tree rests while it plays and keeps its pick. A figure
// opens to list its parts, to show one on its own. While a terrain is shown
// no figure is picked, and picking one shows it again.
function FiguresTab() {
  const value = useSettings((s) => (s.terrain ? '' : treeValue(s.figure, s.part)));
  const story = useSettings((s) => s.story);
  const terrain = useSettings((s) => s.terrain);
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      {story && <Note>The story {story} brings its own figures. The one picked here comes back when it stops.</Note>}
      {!story && terrain && <Note>The terrain {terrain} is shown. Pick a figure to show it instead.</Note>}
      <SearchTree
        label="Figures"
        nodes={figureTree}
        value={value}
        disabled={story !== null}
        onChange={(picked) => {
          const { figure, part } = fromTreeValue(picked);
          setFigure(figure, part);
        }}
      />
    </div>
  );
}

// The terrains in their folder under src/environtments/terrains, searchable,
// as the Figures tab lists the figures. Picking one shows it on its own, on
// bare land, in place of the figure; picking a figure shows the figure
// again. A story brings its own ground, so the tree rests while it plays.
function TerrainsTab() {
  const terrain = useSettings((s) => s.terrain);
  const story = useSettings((s) => s.story);
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      {story && <Note>The story {story} brings its own ground. The terrain picked here comes back when it stops.</Note>}
      {!story && !terrain && <Note>Pick a terrain to see it on its own, in place of the figure.</Note>}
      <SearchTree label="Terrains" nodes={terrainTree} value={terrain ?? ''} disabled={story !== null} onChange={setTerrain} />
    </div>
  );
}

// The shown figure's behaviours, or a note when its spec has none (the boat,
// the trees, the terrains) or a story plays. A part shown on its own has
// none: they are the whole figure's.
function BehavioursTab() {
  const count = useBehaviours((s) => s.behaviours.length);
  const shown = useSettings((s) => s.story ?? s.terrain ?? s.figure);
  const part = useSettings((s) => (s.story || s.terrain ? null : s.part));
  if (part) return <Note>Only the {shown}'s {part} is shown. Behaviours are the whole figure's: pick {shown} in Figures to run them.</Note>;
  if (count === 0) return <Note>{shown} has no behaviours in its spec.</Note>;
  return <BehaviourControls />;
}

// "none" shows the figure picked in Figures; a story brings its own figures
// and environment, so those two tabs rest while it plays. The lake meeting
// adds its chat below (ChatControls.tsx).
function StoryTab() {
  const story = useSettings((s) => s.story);
  const options = [
    { value: '', label: 'none', hint: 'the picked figure or terrain' },
    ...Object.keys(stories).map((name) => ({ value: name, label: name })),
  ];
  return (
    <div className="flex flex-col gap-4">
      <RadioList label="Story" value={story ?? ''} options={options} onChange={(name) => setStory(name || null)} />
      {story === 'lake_meeting' && <ChatControls />}
    </div>
  );
}

// The Time of day's choices: day and night taking turns, or held at one.
const TIME_OPTIONS = TIMES_OF_DAY.map((time) => ({
  value: time,
  label: time,
  hint: time === 'cycle' ? 'day, dusk, night, dawn' : undefined,
}));

// Shows the figure's own environment until one is picked; a pick stays when
// the figure changes. A story or a terrain brings its own, so the list rests
// while one is shown. Below, the time of day, which only the lake follows.
function EnvironmentsTab() {
  const environment = useSettings(environmentOf);
  const own = useSettings((s) => figureEnvironment(s.figure));
  const story = useSettings((s) => s.story);
  const terrain = useSettings((s) => (s.story ? null : s.terrain));
  const time = useSettings((s) => s.time);
  const options = Object.keys(environments).map((name) => ({
    value: name,
    label: name,
    hint: name === own ? "figure's own" : undefined,
  }));
  return (
    <div className="flex flex-col gap-3">
      {story && <Note>The story {story} plays in its own environment.</Note>}
      {terrain && <Note>The terrain {terrain} is shown on its own land. The environment picked here comes back with the figure.</Note>}
      <RadioList
        label="Environment"
        value={environment}
        options={options}
        disabled={story !== null || terrain !== null}
        onChange={setEnvironment}
      />
      <p className="mt-1 text-sm font-medium">Time of day</p>
      <RadioList
        label="Time of day"
        value={time}
        options={TIME_OPTIONS}
        onChange={(value) => {
          const picked = TIMES_OF_DAY.find((t) => t === value);
          if (picked) setTime(picked);
        }}
      />
      <Note>Only the lake has night.</Note>
    </div>
  );
}

// How everything looks: the painterly filter; felt is the only theme. Rows
// share three columns (label, control, value), so labels and controls line
// up.
function SettingsTab() {
  return (
    <div className="grid grid-cols-[max-content_1fr_max-content] items-center gap-2">
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
      {/* Keyed by the call's shape, so typed values survive a new environment
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
