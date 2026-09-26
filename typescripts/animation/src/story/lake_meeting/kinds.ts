import * as THREE from 'three';
import type { Animal } from '../../figures/animals/Animal';
import { Bear } from '../../figures/animals/Bear';
import { Bird } from '../../figures/animals/Bird';
import { Cat } from '../../figures/animals/Cat';
import { Deer } from '../../figures/animals/Deer';
import { Fox } from '../../figures/animals/Fox';
import { Frog } from '../../figures/animals/Frog';
import { Lion } from '../../figures/animals/Lion';
import { Penguin } from '../../figures/animals/Penguin';
import { Snake } from '../../figures/animals/Snake';
import { Wolf } from '../../figures/animals/Wolf';
import type { Theme } from '../../theme';
import type { Kind } from './events';

// The figures at the lake meeting: the host's Bear and the kinds a viewer's
// animal can be. They share the behaviours the meeting uses: Jump(), Walk(),
// Run(), Stop() and Speech(text, speaker); the penguin, an animal too, can
// also Flap().
export type Figure = Animal;

const MAKERS: Record<Kind | 'bear', (theme: Theme) => Figure> = {
  bear: (theme) => new Bear({ theme }),
  cat: (theme) => new Cat({ theme }),
  wolf: (theme) => new Wolf({ theme }),
  deer: (theme) => new Deer({ theme }),
  bird: (theme) => new Bird({ theme }),
  fox: (theme) => new Fox({ theme }),
  snake: (theme) => new Snake({ theme }),
  penguin: (theme) => new Penguin({ theme }),
  frog: (theme) => new Frog({ theme }),
  lion: (theme) => new Lion({ theme }),
};

export function makeFigure(kind: Kind | 'bear', theme: Theme): Figure {
  const figure = MAKERS[kind](theme);
  figure.rotation.order = 'YXZ'; // heading first, then leaning with the ground's slope
  return figure;
}

export function isPenguin(figure: Figure): figure is Penguin {
  return figure instanceof Penguin;
}

// Frees a figure's geometries and materials once it has left, and its speech
// bubble's drawing; other textures stay, since some are shared (the stage
// frees what is left when the story ends).
export function disposeFigure(root: THREE.Object3D): void {
  const resources = new Set<{ dispose(): void }>();
  root.traverse((object) => {
    if (object instanceof THREE.Sprite) {
      resources.add(object.material);
      if (object.material.map) resources.add(object.material.map);
    }
    if (!(object instanceof THREE.Mesh)) return;
    resources.add(object.geometry);
    for (const material of [object.material].flat()) resources.add(material);
  });
  for (const resource of resources) resource.dispose();
}
