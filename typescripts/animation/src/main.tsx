import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Panel } from './panel/Panel';
import { StreamPreview } from './panel/StreamTab';
import { environments, previews } from './previews';
import { environmentOf, setCamera, settings, type Settings } from './settings';
import { Stage } from './Stage';
import { stories } from './stories';
import { connectStream, streamOptions } from './stream';
import { drawForStream, rendererSize } from './streamRenderer';
import { themes } from './theme';

// The preview. The stage draws the figure with Three.js and the panel (React)
// changes the settings. Every settings change is applied to the stage here;
// its setters do nothing when a value is unchanged, so React never touches
// the scene and never runs per frame. The one way back: dragging the view
// away from the picked camera direction sets it to free.
//
// The dev server's hidden browser opens the page as the stream's renderer
// (?renderer=1280x720, streamRenderer.ts): the stage at the stream's size,
// following the panel of another page, and no panel of its own.
const size = rendererSize();
const stage = new Stage(document.body, size ?? undefined);

function apply(s: Settings): void {
  // Before show(), so a new figure appears from the picked direction at once.
  stage.setCameraDirection(s.camera);
  // A story brings its own figures and environment; otherwise the picked ones.
  const story = s.story ? stories[s.story] : null;
  if (story) stage.show(story.create, environments[story.environment], themes[s.theme]);
  else stage.show(previews[s.figure], environments[environmentOf(s)], themes[s.theme]);
  stage.setKuwahara(s.kuwahara.on ? s.kuwahara.radius : 0);
}
apply(settings.getState());
settings.subscribe(apply);

if (size) {
  drawForStream(stage);
} else {
  stage.onDirectionLost = () => setCamera('free');
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
