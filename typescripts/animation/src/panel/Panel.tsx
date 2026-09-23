import { useState } from 'react';
import { useBehaviours, type Behaviour } from '../behaviours';
import { environments, previews } from '../previews';
import {
  environmentOf,
  KUWAHARA_MAX_RADIUS,
  setEnvironment,
  setFigure,
  setKuwahara,
  setStory,
  setTheme,
  useSettings,
} from '../settings';
import { stories } from '../stories';
import { themes } from '../theme';
import { Action, Choice, Field, Select, Slider, Toggle } from './controls';
import './panel.css';

// The control panel, top right over the canvas. Each control reads its
// setting with useSettings and changes it with a setter from settings.ts;
// main.tsx passes every change on to the stage.
export function Panel() {
  return (
    <div className="controls">
      <StorySelect />
      <FigureSelect />
      <EnvironmentSelect />
      <ThemeSelect />
      <KuwaharaControls />
      <BehaviourControls />
    </div>
  );
}

// The shown figure's behaviours, named as in its spec: a button for each,
// beside an input for each of its arguments. The first one run stops the
// demo, so the figure does only what it is told; Restart demo brings the demo
// back. A value the figure can't read (an unknown chord) shows its error below.
function BehaviourControls() {
  const list = useBehaviours((s) => s.behaviours);
  const restart = useBehaviours((s) => s.restart);
  if (list.length === 0) return null;
  return (
    <>
      <h2 className="section">Behaviours</h2>
      {/* Keyed by the call's shape, so typed values survive a theme change
          but not a switch to a figure whose behaviour takes other arguments. */}
      {list.map((behaviour) => (
        <BehaviourRow key={signature(behaviour)} behaviour={behaviour} />
      ))}
      <Action label="Restart demo" wide onClick={restart} />
    </>
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
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };
  return (
    <div className="behaviour">
      <Action label={behaviour.name} onClick={run} />
      <div className="params">
        {params.map((p, i) =>
          p.options ? (
            <Choice key={p.name} name={p.name} value={values[i]} options={p.options} onChange={(v) => set(i, v)} />
          ) : (
            <Field key={p.name} name={p.name} value={values[i]} onChange={(v) => set(i, v)} onSubmit={run} />
          ),
        )}
      </div>
      {error && <p className="error">{error}</p>}
    </div>
  );
}

// "none" shows the figure picked below; a story brings its own figures and
// environment, so the two menus below rest while it plays.
function StorySelect() {
  const story = useSettings((s) => s.story);
  const options = [{ value: '', label: 'none' }, ...Object.keys(stories).map((name) => ({ value: name, label: name }))];
  return <Select label="Story" value={story ?? ''} options={options} onChange={(name) => setStory(name || null)} />;
}

function FigureSelect() {
  const figure = useSettings((s) => s.figure);
  const playing = useSettings((s) => s.story !== null);
  const options = Object.keys(previews).map((name) => ({ value: name, label: name }));
  return <Select label="Figure" value={figure} options={options} disabled={playing} onChange={setFigure} />;
}

// Shows the figure's own environment until one is picked.
function EnvironmentSelect() {
  const environment = useSettings(environmentOf);
  const playing = useSettings((s) => s.story !== null);
  const options = Object.keys(environments).map((name) => ({ value: name, label: name }));
  return (
    <Select label="Environment" value={environment} options={options} disabled={playing} onChange={setEnvironment} />
  );
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
