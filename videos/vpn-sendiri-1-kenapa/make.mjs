// Builds part 1 of "Bikin VPN sendiri": kinetic-type beats (blueprint kinetic-type-beats, sub-shape B; rule
// kinetic-beat-slam), 26 s, in three scenes of four beats. Run: node make.mjs (writes index.html and compositions/).
import { chrome, writeScene, writeHost } from "../vpn-sendiri-shared/frame.mjs";

const C = chrome("bikin vpn sendiri", "01 / 03");

const DIAGRAM_CSS = `
    .box { position: absolute; display: flex; align-items: center; justify-content: center; border: 4px solid #f0ece5;
      background: #1a1a18; font-family: "IBM Plex Mono"; font-weight: 600; font-size: 34px; letter-spacing: 0.1em;
      text-transform: uppercase; color: #f0ece5; }
    .box.vps { border-color: #e85d26; flex-direction: column; gap: 6px; }
    .box.vps .big { font-family: "Barlow"; font-weight: 900; font-size: 110px; line-height: 0.9; letter-spacing: -0.03em;
      color: #e85d26; text-transform: none; }
    .box.vps .tag { font-size: 24px; color: #888880; letter-spacing: 0.14em; }
    .link { position: absolute; height: 0; border-top: 6px dashed #505048; transform-origin: 0 50%; }
    .cross { position: absolute; width: 110px; height: 110px; }
    .cross i { position: absolute; left: 50%; top: 50%; width: 110px; height: 14px; margin: -7px 0 0 -55px; background: #e85d26; }
    .cross i:first-child { transform: rotate(45deg); }
    .cross i:last-child { transform: rotate(-45deg); }
    .say { position: absolute; left: 59px; width: 842px; font-family: "Barlow"; font-weight: 800; font-size: 92px; line-height: 0.95;
      letter-spacing: -0.03em; }
    .say .line { display: block; white-space: nowrap; }
    svg.arrows { position: absolute; left: 0; top: 0; width: 1080px; height: 1920px; overflow: visible; }
    svg.arrows path { fill: none; stroke: #e85d26; stroke-width: 8; stroke-linecap: square; stroke-linejoin: miter; }`;

const BULLET_CSS = `
    .bul { display: flex; align-items: baseline; gap: 26px; font-family: "Barlow"; font-weight: 700; font-size: 62px;
      line-height: 1.12; margin-bottom: 30px; white-space: nowrap; }
    .bul b { font-family: "IBM Plex Mono"; font-weight: 600; color: #e85d26; }`;

const beat = (id, start, dur, reg, inner) =>
  `    <div id="${id}" class="clip beat ${reg}" data-start="${start}" data-duration="${dur}" data-track-index="0">
      ${C}
${inner}
    </div>`;

// ── scene 1, "masalah" (0–7.2 s): the question, the answer, the two reasons ──────────────────────────────────────
const masalah = {
  id: "masalah", duration: 7.2,
  body: [
    beat("m1", 0, 2, "dark", `      <div class="stage">
        <span id="m1k" class="kicker">PC kantor</span>
        <span id="m1a" class="w h1" style="font-size: 124px">mau diremote</span>
        <span id="m1b" class="w h1" style="font-size: 124px">dari rumah?</span>
      </div>`),
    beat("m2", 2, 1.6, "orange", `      <div class="stage">
        <span id="m2a" class="w" style="font-size: 260px">gak</span>
        <span id="m2b" class="w" style="font-size: 260px">bisa.</span>
      </div>`),
    beat("m3", 3.6, 1.8, "dark", `      <div class="stage">
        <span id="m3a" class="w h1" style="font-size: 100px">kantor gak punya</span>
        <span id="m3b" class="w acc" style="font-size: 172px">IP publik</span>
      </div>`),
    beat("m4", 5.4, 1.8, "dark", `      <div class="stage">
        <span id="m4a" class="w h1" style="font-size: 120px">rumah kena</span>
        <span id="m4b" class="w acc" style="font-size: 230px">CGNAT</span>
      </div>`),
  ].join("\n"),
  js: `
      // m1: kicker snaps in, the question slams with scale + blur
      tl.fromTo($("m1k"), { x: -60, opacity: 0 }, { x: 0, opacity: 1, duration: 0.4, ease: "expo.out" }, 0.05);
      tl.fromTo([$("m1a"), $("m1b")], { scale: 1.35, filter: "blur(14px)", opacity: 0, transformOrigin: "0% 50%" },
        { scale: 1, filter: "blur(0px)", opacity: 1, duration: 0.5, ease: "power4.out", stagger: 0.12 }, 0.1);
      // m2: hard flash on orange, each word lands with a short shake
      function shake(el, at, dir) {
        tl.fromTo(el, { x: 46 * dir, rotation: -3 * dir, transformOrigin: "0% 100%" }, { x: -22 * dir, rotation: 1.5 * dir, duration: 0.08, ease: "power2.out" }, at);
        tl.to(el, { x: 10 * dir, rotation: -0.6 * dir, duration: 0.08, ease: "power2.inOut" }, at + 0.08);
        tl.to(el, { x: 0, rotation: 0, duration: 0.12, ease: "power2.out" }, at + 0.16);
      }
      shake($("m2a"), 2, 1);
      shake($("m2b"), 2.06, -1);
      // m3: side snaps from opposite edges
      tl.fromTo($("m3a"), { x: -300, opacity: 0 }, { x: 0, opacity: 1, duration: 0.45, ease: "expo.out" }, 3.65);
      tl.fromTo($("m3b"), { x: 320, opacity: 0 }, { x: 0, opacity: 1, duration: 0.45, ease: "expo.out" }, 3.85);
      // m4: heavy rise, then the accent word scales up
      tl.fromTo($("m4a"), { y: 90, rotation: 4, opacity: 0, transformOrigin: "0% 100%" }, { y: 0, rotation: 0, opacity: 1, duration: 0.5, ease: "circ.out" }, 5.45);
      tl.fromTo($("m4b"), { scale: 0.55, opacity: 0, transformOrigin: "0% 60%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "power4.out" }, 5.7);`,
};

// ── scene 2, "ide" (7.2–15.8 s): neither can be called, both can call out, a meeting point, the VPS ──────────────────
const ide = {
  id: "ide", duration: 8.6, css: DIAGRAM_CSS,
  body: [
    beat("i1", 0, 2.4, "dark", `      <div id="i1l" class="box" style="left: 59px; top: 560px; width: 340px; height: 200px">PC rumah</div>
      <div id="i1r" class="box" style="left: 681px; top: 560px; width: 340px; height: 200px">PC kantor</div>
      <div id="i1line" class="link" style="left: 399px; top: 657px; width: 282px"></div>
      <div id="i1x" class="cross" style="left: 485px; top: 605px"><i></i><i></i></div>
      <div id="i1s" class="say" style="top: 940px">
        <span class="line">dua-duanya gak</span>
        <span class="line acc">bisa ditelpon</span>
      </div>`),
    beat("i2", 2.4, 1.8, "orange", `      <div class="stage">
        <span class="mask"><span id="i2a" class="w h1" style="font-size: 112px">tapi dua-duanya</span></span>
        <span class="mask"><span id="i2b" class="w h1" style="font-size: 112px">bisa nelpon</span></span>
        <span class="mask"><span id="i2c" class="w" style="font-size: 112px">keluar</span></span>
      </div>`),
    beat("i3", 4.2, 1.8, "dark", `      <div class="stage">
        <span id="i3a" class="w h1" style="font-size: 120px">jadi kasih</span>
        <span id="i3b" class="w acc" style="font-size: 200px">titik</span>
        <span id="i3c" class="w acc" style="font-size: 200px">tengah</span>
      </div>`),
    beat("i4", 6.0, 2.6, "dark", `      <div id="i4v" class="box vps" style="left: 340px; top: 420px; width: 400px; height: 230px">
        <span class="big">VPS</span><span class="tag">IP publik</span>
      </div>
      <svg class="arrows" viewBox="0 0 1080 1920" aria-hidden="true">
        <path id="i4p1" d="M229 1000 L470 670 M470 670 L433 686 M470 670 L466 710" />
        <path id="i4p2" d="M711 1000 L610 670 M610 670 L601 709 M610 670 L639 697" />
      </svg>
      <div id="i4l" class="box" style="left: 59px; top: 1000px; width: 340px; height: 170px">PC rumah</div>
      <div id="i4r" class="box" style="left: 541px; top: 1000px; width: 340px; height: 170px">PC kantor</div>
      <div id="i4s" class="say" style="top: 1260px; font-size: 76px">
        <span class="line">dua-duanya nyambung</span>
        <span class="line acc">ke VPS</span>
      </div>`),
  ].join("\n"),
  js: `
      // i1: the two PCs slide in, the dashed link draws, the cross lands, then the statement
      tl.fromTo($("i1l"), { x: -240, opacity: 0 }, { x: 0, opacity: 1, duration: 0.45, ease: "power3.out" }, 0.05);
      tl.fromTo($("i1r"), { x: 240, opacity: 0 }, { x: 0, opacity: 1, duration: 0.45, ease: "power3.out" }, 0.12);
      tl.fromTo($("i1line"), { scaleX: 0 }, { scaleX: 1, duration: 0.4, ease: "power2.inOut" }, 0.4);
      tl.fromTo($("i1x"), { scale: 0, rotation: -90 }, { scale: 1, rotation: 0, duration: 0.4, ease: "back.out(2)" }, 0.8);
      tl.fromTo($("i1s"), { y: 50, opacity: 0 }, { y: 0, opacity: 1, duration: 0.45, ease: "power3.out" }, 1.05);
      // i2: bottom-up masked slide, line by line
      tl.fromTo([$("i2a"), $("i2b"), $("i2c")], { yPercent: 110 }, { yPercent: 0, duration: 0.45, ease: "power3.out", stagger: 0.16 }, 2.45);
      // i3: the first line drops in, "titik tengah" squeezes in from a wide stretch (scaleX, not letter-spacing)
      tl.fromTo($("i3a"), { y: -50, opacity: 0 }, { y: 0, opacity: 1, duration: 0.4, ease: "power3.out" }, 4.25);
      tl.fromTo([$("i3b"), $("i3c")], { scaleX: 1.7, opacity: 0, transformOrigin: "0% 50%" }, { scaleX: 1, opacity: 1, duration: 0.55, ease: "power3.out", stagger: 0.1 }, 4.5);
      // i4: the VPS pops, the PCs rise, the arrows draw up to it, then the caption
      tl.fromTo($("i4v"), { scale: 0.7, opacity: 0 }, { scale: 1, opacity: 1, duration: 0.45, ease: "power4.out" }, 6.05);
      tl.fromTo([$("i4l"), $("i4r")], { y: 60, opacity: 0 }, { y: 0, opacity: 1, duration: 0.4, ease: "power3.out", stagger: 0.08 }, 6.25);
      [["i4p1", 6.6], ["i4p2", 6.75]].forEach(function (a) {
        var p = $(a[0]); var len = 520;
        p.style.strokeDasharray = len + " " + len;
        tl.fromTo(p, { strokeDashoffset: len }, { strokeDashoffset: 0, duration: 0.5, ease: "power2.inOut" }, a[1]);
      });
      tl.fromTo($("i4s"), { y: 40, opacity: 0 }, { y: 0, opacity: 1, duration: 0.45, ease: "power3.out" }, 7.25);`,
};

// ── scene 3, "solusi" (15.8–26 s): WireGuard, why, the payoff, next part ────────────────────────────────────────────
const solusi = {
  id: "solusi", duration: 10.2, css: BULLET_CSS,
  body: [
    beat("s1", 0, 1.8, "orange", `      <div class="stage">
        <span id="s1k" class="kicker">terowongannya</span>
        <span id="s1a" class="w h1" style="font-size: 130px">dikunci</span>
        <span id="s1b" class="w" style="font-size: 165px">WireGuard</span>
      </div>`),
    beat("s2", 1.8, 2.8, "dark", `      <div class="stage">
        <span id="s2k" class="kicker">kenapa WireGuard</span>
        <div id="s2a" class="bul"><b>/</b><span>gratis &amp; open source</span></div>
        <div id="s2b" class="bul"><b>/</b><span>datanya terenkripsi</span></div>
        <div id="s2c" class="bul"><b>/</b><span>servernya punyamu sendiri</span></div>
      </div>`),
    beat("s3", 4.6, 2.4, "orange", `      <div class="stage">
        <span id="s3a" class="w" style="font-size: 220px">VPN-mu</span>
        <span id="s3b" class="w" style="font-size: 220px">sendiri</span>
      </div>`),
    beat("s4", 7.0, 3.2, "dark", `      <div id="s4st" class="stage">
        <span id="s4k" class="kicker">lanjut bagian 2</span>
        <span class="mask"><span id="s4a" class="w h1" style="font-size: 150px">bikin</span></span>
        <span class="mask"><span id="s4b" class="w h1 acc" style="font-size: 150px">servernya</span></span>
        <span id="s4n" class="note">5 langkah · di VPS</span>
      </div>`),
  ].join("\n"),
  js: `
      // s1: kicker and "dikunci" drop, "WireGuard" slams with blur
      tl.fromTo([$("s1k"), $("s1a")], { y: -60, opacity: 0 }, { y: 0, opacity: 1, duration: 0.4, ease: "power3.out", stagger: 0.08 }, 0.05);
      tl.fromTo($("s1b"), { scale: 1.5, filter: "blur(16px)", opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, filter: "blur(0px)", opacity: 1, duration: 0.5, ease: "power4.out" }, 0.3);
      // s2: the three reasons cascade in (waterfall)
      tl.fromTo($("s2k"), { opacity: 0 }, { opacity: 1, duration: 0.3, ease: "power2.out" }, 1.85);
      tl.fromTo([$("s2a"), $("s2b"), $("s2c")], { x: 90, opacity: 0 }, { x: 0, opacity: 1, duration: 0.45, ease: "expo.out", stagger: 0.38 }, 2.0);
      // s3: the payoff spring-pops, then holds
      tl.fromTo([$("s3a"), $("s3b")], { scale: 0.45, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.55, ease: "back.out(1.5)", stagger: 0.1 }, 4.65);
      // s4: next part, masked slides, then a slow finite breath to the end
      tl.fromTo($("s4k"), { x: -50, opacity: 0 }, { x: 0, opacity: 1, duration: 0.4, ease: "expo.out" }, 7.05);
      tl.fromTo([$("s4a"), $("s4b")], { yPercent: 110 }, { yPercent: 0, duration: 0.5, ease: "power3.out", stagger: 0.14 }, 7.15);
      tl.fromTo($("s4n"), { opacity: 0 }, { opacity: 1, duration: 0.4, ease: "power2.out" }, 7.7);
      tl.fromTo($("s4st"), { scale: 1, transformOrigin: "0% 50%" }, { scale: 1.025, duration: 2.2, ease: "sine.inOut" }, 7.9);`,
};

const scenes = [masalah, ide, solusi];
for (const s of scenes) writeScene(`compositions/${s.id}.html`, s);

// SFX on the beat grid (root time): a pop on every cut, a ting on the payoff.
const pop = (at, vol = 0.3) => ({ file: "pop.wav", at, dur: 0.18, vol });
const total = writeHost("index.html", {
  duration: 26,
  scenes,
  sfx: [pop(0.05), pop(2, 0.35), pop(3.6), pop(5.4), pop(7.2), pop(9.6, 0.35), pop(11.4), pop(13.2), pop(15.8, 0.35), pop(17.6),
    { file: "ting.wav", at: 20.4, dur: 0.45, vol: 0.4 }, pop(22.8)],
});
console.log("part 1:", scenes.length, "scenes,", total, "s");
