import * as THREE from 'three';
import { OVERLAY_LAYER } from '../overlay';
import type { Theme } from '../theme';

// A comic speech bubble that says a text: a rounded box with a tail pointing
// down at the speaker, in the theme's light colour, with the outline and the
// words in its dark colour. A bubble holds at most MAX_WORDS words, so longer
// text is split into several; a word longer than MAX_WORD_LENGTH characters
// is first cut into pieces that long, each counted as a word. In a bubble the
// words wrap into lines no wider than MAX_WIDTH, and the bubble is as tall as
// its lines. The bubbles pop up from the tail one after another, each staying
// long enough to read, then pop away.
//
// Its origin is the tip of the tail: place it just above the speaker's head.
// Sizes are in meters, so the bubble belongs to the scene and grows as the
// camera comes closer. It always faces the camera (a sprite) and is drawn on
// the overlay layer, on top of everything and untouched by effects (see
// ../overlay.ts). Call update(delta) once per frame.

const FONT_SIZE = 0.032; // m, the text's em
const LINE_HEIGHT = 1.25; // ems from one line to the next
const MAX_WIDTH = 0.34; // m, the longest line
const MAX_WORDS = 9; // words in one bubble
const MAX_WORD_LENGTH = 50; // characters; a longer word is cut into pieces this long
const PADDING = 0.02; // m, between the text and the outline
const CORNER = 0.03; // m, the box's corner radius
const OUTLINE = 0.004; // m
const TAIL = { width: 0.036, length: 0.035 }; // m
const FONT = (px: number) => `600 ${px}px system-ui, -apple-system, "Segoe UI", sans-serif`;
const PX_PER_M = 1600; // the drawing's resolution: sharp even up close

// Timing, in seconds: a bubble pops up, stays for its reading time (about 16
// characters a second, never less than READ.min), pops away, and the next
// one follows after a short gap.
const POP = 0.18;
const READ = { base: 0.8, perChar: 0.06, min: 1.4 };
const GAP = 0.12;

const px = (meters: number) => meters * PX_PER_M;
let measurer: CanvasRenderingContext2D | null = null;

function measuring(): CanvasRenderingContext2D {
  measurer ??= document.createElement('canvas').getContext('2d')!;
  measurer.font = FONT(px(FONT_SIZE));
  return measurer;
}

// What each bubble says: the text's words, MAX_WORDS to a bubble, after any
// word longer than MAX_WORD_LENGTH characters is cut into pieces that long.
export function splitSpeech(text: string): string[] {
  const words = text
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .flatMap((word) => word.match(new RegExp(`.{1,${MAX_WORD_LENGTH}}`, 'gu')) ?? []);
  const bubbles: string[] = [];
  for (let i = 0; i < words.length; i += MAX_WORDS) bubbles.push(words.slice(i, i + MAX_WORDS).join(' '));
  return bubbles;
}

// A bubble's words in lines no wider than MAX_WIDTH. A word longer than a
// whole line (such as a 50-character piece) is cut to fit.
function wrap(text: string): string[] {
  const ctx = measuring();
  const fits = (s: string) => ctx.measureText(s).width <= px(MAX_WIDTH);
  const lines: string[] = [];
  let line = '';
  for (let word of text.trim().split(/\s+/).filter(Boolean)) {
    while (!fits(word)) {
      let cut = word.length - 1;
      while (cut > 1 && !fits(word.slice(0, cut))) cut--;
      if (line) lines.push(line);
      lines.push(word.slice(0, cut));
      line = '';
      word = word.slice(cut);
    }
    const longer = line ? `${line} ${word}` : word;
    if (line && !fits(longer)) {
      lines.push(line);
      line = word;
    } else {
      line = longer;
    }
  }
  if (line) lines.push(line);
  return lines;
}

// A bubble holding the given lines, drawn on a canvas, and where its tail's
// tip is (as a share of the canvas height, from the bottom).
function draw(lines: string[], fill: string, ink: string): { canvas: HTMLCanvasElement; tip: number } {
  const ctx = measuring();
  const lineHeight = px(FONT_SIZE * LINE_HEIGHT);
  const boxWidth = Math.max(...lines.map((l) => ctx.measureText(l).width)) + 2 * px(PADDING);
  const boxHeight = lines.length * lineHeight + 2 * px(PADDING);
  const margin = px(OUTLINE); // room for the outline outside the box

  const canvas = document.createElement('canvas');
  canvas.width = Math.ceil(boxWidth + 2 * margin);
  canvas.height = Math.ceil(boxHeight + px(TAIL.length) + 2 * margin);
  const g = canvas.getContext('2d')!;

  const [x0, y0, x1, y1] = [margin, margin, margin + boxWidth, margin + boxHeight];
  const r = px(CORNER);
  const middle = canvas.width / 2;
  const half = px(TAIL.width) / 2;
  const tipY = y1 + px(TAIL.length);
  g.beginPath();
  g.moveTo(x0 + r, y0);
  g.arcTo(x1, y0, x1, y1, r);
  g.arcTo(x1, y1, x0, y1, r);
  g.lineTo(middle + half, y1);
  g.lineTo(middle, tipY);
  g.lineTo(middle - half, y1);
  g.arcTo(x0, y1, x0, y0, r);
  g.arcTo(x0, y0, x1, y0, r);
  g.closePath();
  g.fillStyle = fill;
  g.fill();
  g.lineWidth = px(OUTLINE);
  g.lineJoin = 'round';
  g.strokeStyle = ink;
  g.stroke();

  g.font = FONT(px(FONT_SIZE));
  g.fillStyle = ink;
  g.textAlign = 'center';
  g.textBaseline = 'middle';
  lines.forEach((line, i) => g.fillText(line, middle, y0 + px(PADDING) + (i + 0.5) * lineHeight));
  return { canvas, tip: (canvas.height - tipY) / canvas.height };
}

// Rises past full size and settles back, for p from 0 to 1.
function popUp(p: number): number {
  const s = 1.7;
  const q = p - 1;
  return 1 + (s + 1) * q * q * q + s * q * q;
}

export class SpeechBubble extends THREE.Group {
  private readonly fill: string;
  private readonly ink: string;
  private sprite: THREE.Sprite | null = null;
  private readonly size = new THREE.Vector2(); // m, the shown bubble at full size
  private bubbles: string[] = []; // what each bubble still to show says, the shown one first
  private elapsed = 0; // seconds since the shown bubble began
  private stay = 0; // seconds it stays up at full size

  constructor(theme: Theme) {
    super();
    this.name = 'speech bubble';
    this.fill = `#${new THREE.Color(theme.colors.light).getHexString()}`;
    this.ink = `#${new THREE.Color(theme.colors.dark).getHexString()}`;
  }

  // Whether it is still saying something.
  get speaking(): boolean {
    return this.bubbles.length > 0;
  }

  // Says the text, in as many bubbles as it takes, replacing anything it was
  // still saying. An empty text just stops it.
  say(text: string): void {
    this.hide();
    this.bubbles = splitSpeech(text);
    if (this.speaking) this.begin();
  }

  update(delta: number): void {
    if (!this.sprite) return;
    this.elapsed += delta;
    const t = this.elapsed;
    let scale = 1;
    if (t < POP) scale = popUp(t / POP);
    else if (t > POP + this.stay) scale = Math.max(0, 1 - ((t - POP - this.stay) / POP) ** 2);
    this.sprite.scale.set(this.size.x * scale, this.size.y * scale, 1);

    if (t >= 2 * POP + this.stay + GAP) {
      this.hide();
      this.bubbles.shift();
      if (this.speaking) this.begin();
    }
  }

  private begin(): void {
    const words = this.bubbles[0];
    const { canvas, tip } = draw(wrap(words), this.fill, this.ink);
    const texture = new THREE.CanvasTexture(canvas);
    texture.colorSpace = THREE.SRGBColorSpace;
    const material = new THREE.SpriteMaterial({ map: texture, depthTest: false, depthWrite: false });
    this.sprite = new THREE.Sprite(material);
    this.sprite.center.set(0.5, tip); // the tail's tip sits at the origin
    this.sprite.layers.set(OVERLAY_LAYER);
    this.sprite.scale.set(0, 0, 1);
    this.size.set(canvas.width / PX_PER_M, canvas.height / PX_PER_M);
    this.add(this.sprite);

    this.stay = Math.max(READ.min, READ.base + READ.perChar * words.length);
    this.elapsed = 0;
  }

  private hide(): void {
    if (!this.sprite) return;
    this.remove(this.sprite);
    this.sprite.material.map?.dispose();
    this.sprite.material.dispose();
    this.sprite = null;
  }
}
