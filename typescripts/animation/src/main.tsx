import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Panel } from './panel/Panel';
import { environments, previews } from './previews';
import { environmentOf, settings, type Settings } from './settings';
import { Stage } from './Stage';
import { stories } from './stories';
import { themes } from './theme';

// The preview. The stage draws the figure with Three.js and the panel (React)
// changes the settings. Every settings change is applied to the stage here;
// its setters do nothing when a value is unchanged, so React never touches
// the scene and never runs per frame.
const stage = new Stage(document.body);

function apply(s: Settings): void {
  // A story brings its own figures and environment; otherwise the picked ones.
  const story = s.story ? stories[s.story] : null;
  if (story) stage.show(story.create, environments[story.environment], themes[s.theme]);
  else stage.show(previews[s.figure], environments[environmentOf(s)], themes[s.theme]);
  stage.setKuwahara(s.kuwahara.on ? s.kuwahara.radius : 0);
}
apply(settings.getState());
settings.subscribe(apply);

createRoot(document.getElementById('panel')!).render(
  <StrictMode>
    <Panel />
  </StrictMode>,
);
