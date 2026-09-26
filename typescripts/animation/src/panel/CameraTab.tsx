import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  ChevronDown,
  ChevronUp,
  RotateCcw,
  RotateCw,
  type LucideIcon,
} from 'lucide-react';
import type { PointerEvent } from 'react';
import { CAMERA_DIRECTIONS, isCameraDirection } from '../camera';
import { EYE_HEIGHT, setCamera, setEyeHeight, setWalkSpeed, useSettings, WALK_SPEED } from '../settings';
import { press, release, useWalking, type Move } from '../walk';
import { Action, Select, Slider } from './controls';
import { Button } from './ui/button';

// The Camera tab: the View menu, which way the camera looks at the figure
// (camera.ts), and walking the camera round the scene at eye height (walk):
// a pad whose buttons walk while held, how fast it walks and how high its
// eyes are. The menu says "free (dragged)" once the view is dragged away, so
// the same view can be picked again. Rows share the panel's three columns,
// like the Settings tab.

export function CameraTab() {
  const camera = useSettings((s) => s.camera);
  const options = CAMERA_DIRECTIONS.map(({ value, label }) => ({ value, label }));
  return (
    <div className="grid grid-cols-[max-content_1fr_max-content] items-center gap-2">
      <Select label="View" value={camera} options={options} onChange={(v) => isCameraDirection(v) && setCamera(v)} />
      {camera === 'walk' ? (
        <WalkControls />
      ) : (
        <>
          <Action label="Walk from here" wide onClick={() => setCamera('walk')} />
          <p className="col-span-full text-xs text-muted-foreground">
            Walks the camera round the scene at eye height, over the ground and the water, from where it looks now.
          </p>
        </>
      )}
    </div>
  );
}

interface PadButton {
  move: Move;
  label: string;
  icon: LucideIcon;
  keys: string;
}

// Two rows: turn, forward, turn and look up; sideways, back, sideways and
// look down.
const PAD: PadButton[] = [
  { move: 'turnLeft', label: 'Turn left', icon: RotateCcw, keys: 'Q or ←' },
  { move: 'forward', label: 'Forward', icon: ArrowUp, keys: 'W or ↑' },
  { move: 'turnRight', label: 'Turn right', icon: RotateCw, keys: 'E or →' },
  { move: 'lookUp', label: 'Look up', icon: ChevronUp, keys: 'R' },
  { move: 'left', label: 'Step left', icon: ArrowLeft, keys: 'A' },
  { move: 'back', label: 'Back', icon: ArrowDown, keys: 'S or ↓' },
  { move: 'right', label: 'Step right', icon: ArrowRight, keys: 'D' },
  { move: 'lookDown', label: 'Look down', icon: ChevronDown, keys: 'F' },
];

function WalkControls() {
  const speed = useSettings((s) => s.walkSpeed);
  const eyeHeight = useSettings((s) => s.eyeHeight);
  return (
    <>
      <div role="group" aria-label="Walk" className="col-span-full grid grid-cols-4 gap-1.5">
        {PAD.map((button) => (
          <PadKey key={button.move} {...button} />
        ))}
      </div>
      <Slider
        label="Speed"
        value={speed}
        min={WALK_SPEED.least}
        max={WALK_SPEED.most}
        step={0.1}
        unit="m/s"
        onChange={setWalkSpeed}
      />
      <Slider
        label="Eye height"
        value={eyeHeight}
        min={EYE_HEIGHT.least}
        max={EYE_HEIGHT.most}
        step={0.1}
        unit="m"
        onChange={setEyeHeight}
      />
      <p className="col-span-full text-xs text-muted-foreground">
        Hold a button, or the keys: W S forward and back, A D sideways, Q E or ← → to turn, R F to look up and
        down, Shift to run. Drag the view to look round; the wheel steps. Pick another view to stop walking.
      </p>
    </>
  );
}

// Walks while it is held: by a pointer (a mouse button, a finger), or by
// Space or Enter while it has the focus. It lights up while its move is
// held, whichever way (a key too).
function PadKey({ move, label, icon: Icon, keys }: PadButton) {
  const held = useWalking((w) => w.held.includes(move));
  const by = (event: PointerEvent) => `pointer ${event.pointerId}`;
  const letGo = (event: PointerEvent) => release(move, by(event));
  return (
    <Button
      variant={held ? 'default' : 'outline'}
      className="h-9 touch-none select-none"
      aria-label={label}
      aria-pressed={held}
      title={`${label} (${keys})`}
      onPointerDown={(event) => {
        if (event.button !== 0) return;
        event.currentTarget.setPointerCapture(event.pointerId);
        press(move, by(event));
      }}
      onPointerUp={letGo}
      onPointerCancel={letGo}
      onLostPointerCapture={letGo}
      onContextMenu={(event) => event.preventDefault()} // a long touch shows no menu
      onKeyDown={(event) => {
        if ((event.key === ' ' || event.key === 'Enter') && !event.repeat) press(move, 'focus');
      }}
      onKeyUp={(event) => {
        if (event.key === ' ' || event.key === 'Enter') release(move, 'focus');
      }}
      onBlur={() => release(move, 'focus')}
    >
      <Icon />
    </Button>
  );
}
