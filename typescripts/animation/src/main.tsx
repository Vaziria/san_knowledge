import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { playHere } from './audio';
import { Panel } from './panel/Panel';
import { StreamPreview } from './panel/StreamTab';
import { partPreview } from './parts';
import { environments, previews } from './previews';
import { environmentOf, setCamera, settings, type Settings } from './settings';
import { Stage } from './Stage';
import { stories } from './stories';
import { connectStream, streamOptions } from './stream';
import { drawForStream, rendererSize } from './streamRenderer';
import { terrains } from './terrains';
import { defaultTheme } from './theme';
import { listenForWalkKeys, walking } from './walk';

// The preview. The stage draws the figure with Three.js and the panel (React)
// changes the settings. Every settings change is applied to the stage here;
// its setters do nothing when a value is unchanged, so React never touches
// the scene and never runs per frame. Two ways back: dragging the view away
// from the picked camera direction sets it to free, and a story claiming the
// camera back from the viewer (the lake meeting's next comment) sets it to
// the figure's own.
//
// The dev server's hidden browser opens the page as the stream's renderer
// (?renderer=1280x720, streamRenderer.ts): the stage at the stream's size,
// following the panel of another page, and no panel of its own.
const size = rendererSize();
const stage = new Stage(document.body, size ?? undefined);

function apply(s: Settings): void {
  stage.setWalk(s.walkSpeed, s.eyeHeight);
  // Before show(), so a new environment is built into it.
  stage.setTimeOfDay(s.time);
  // Before show(), so a new figure appears from the picked direction at once.
  stage.setCameraDirection(s.camera);
  // A story brings its own figures and environment, and a terrain its own
  // land (terrains.ts); otherwise the picked figure, or only the picked part
  // of it.
  const own = s.story ? stories[s.story] : s.terrain ? terrains[s.terrain] : null;
  if (own) stage.show(own.create, own.environment, defaultTheme);
  else stage.show(s.part ? partPreview(s.figure, s.part) : previews[s.figure], environments[environmentOf(s)], defaultTheme);
  stage.setKuwahara(s.kuwahara.on ? s.kuwahara.radius : 0);
}
apply(settings.getState());
settings.subscribe(apply);
// Walking the camera: what the Camera tab's pad and the keys hold down.
walking.subscribe((w) => stage.setMoves(w.held));

if (size) {
  drawForStream(stage);
} else {
  stage.onDirectionLost = () => setCamera('free');
  stage.onDirectionTaken = () => setCamera('figure');
  listenForWalkKeys();
  // The Audio tab's file plays here too, with Play here (the stream's sound
  // is the dev server's).
  playHere();
  // The Stream tab streams what the stage draws, or has the dev server draw
  // it; with "stream only" this page doesn't draw it.
  connectStream(stage);
  const draw = () => stage.setDrawing(streamOptions.getState().render !== 'stream');
  draw();
  streamOptions.subscribe(draw);

  createRoot(document.getElementById('panel')!).render(
    <StrictMode>
      <StreamPreview />
      <Panel />
    </StrictMode>,
  );
}
