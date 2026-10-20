// Shared by index.html (16:9) and compositions/vertical.html (9:16). Builds one paused GSAP timeline with:
// the camera on each view of the recorded screen, the cursor ring and click ripples (positions
// from the take's input log in tutorial-data.js, in screen pixels), and the entrances of the
// caption and key-label clips. The page registers the returned timeline itself.
(function () {
  const SRC_W = 1920;
  const SRC_H = 1080;

  // A camera key is [time, centre x, centre y, scale, transition seconds]. The centre is
  // clamped so the view never shows past the edge of the recording.
  function pose(view, key) {
    const s = key[3];
    const half = (size, src, c) => {
      const h = size / (2 * s);
      return 2 * h >= src ? src / 2 : Math.min(Math.max(c, h), src - h);
    };
    const cx = half(view.w, SRC_W, key[1]);
    const cy = half(view.h, SRC_H, key[2]);
    return { x: view.w / 2 - cx * s, y: view.h / 2 - cy * s, scale: s };
  }

  function scaleAt(view, t) {
    let s = view.keys[0][3];
    for (const k of view.keys) if (k[0] <= t) s = k[3];
    return s;
  }

  function camera(tl, view) {
    const cam = view.el.querySelector(".cam");
    const ring = view.el.querySelector(".ring-scale");
    view.keys.forEach((k, i) => {
      const p = pose(view, k);
      if (i === 0) {
        tl.set(cam, { ...p, transformOrigin: "0 0" }, 0);
        tl.set(ring, { scale: 1 / p.scale }, 0);
        return;
      }
      const d = k[4] === undefined ? 0.8 : k[4];
      tl.to(cam, { ...p, duration: d, ease: "power2.inOut" }, k[0]);
      // Keep the ring the same size on screen whatever the zoom.
      tl.to(ring, { scale: 1 / p.scale, duration: d, ease: "power2.inOut" }, k[0]);
    });
  }

  function cursor(tl, view, path) {
    const ring = view.el.querySelector(".ring");
    tl.set(ring, { x: path[0][1], y: path[0][2] }, 0);
    for (let i = 1; i < path.length; i++) {
      const t0 = path[i - 1][0];
      const [t, x, y] = path[i];
      tl.to(ring, { x, y, duration: Math.max(0.001, t - t0), ease: "none" }, t0);
    }
  }

  function ripples(tl, view, clicks) {
    const layer = view.el.querySelector(".ripples");
    for (const [t, x, y] of clicks) {
      const r = document.createElement("div");
      r.className = "ripple";
      r.style.left = x + "px";
      r.style.top = y + "px";
      layer.appendChild(r);
      const end = 1.5 / scaleAt(view, t);
      tl.set(r, { scale: 0.3, opacity: 0 }, 0);
      tl.fromTo(r, { scale: 0.3 / scaleAt(view, t), opacity: 1 },
        { scale: end, opacity: 0, duration: 0.6, ease: "power2.out", immediateRender: false }, t);
    }
  }

  function entrances(tl) {
    document.querySelectorAll(".caption").forEach((c) => {
      tl.fromTo(c, { opacity: 0, y: 18 },
        { opacity: 1, y: 0, duration: 0.35, ease: "power3.out", immediateRender: false }, parseFloat(c.dataset.start));
    });
    document.querySelectorAll(".chip").forEach((c) => {
      tl.fromTo(c, { opacity: 0, scale: 0.86 },
        { opacity: 1, scale: 1, duration: 0.2, ease: "back.out(2)", immediateRender: false }, parseFloat(c.dataset.start));
    });
  }

  window.buildTutorial = function (views) {
    const data = window.TUTORIAL;
    const tl = gsap.timeline({ paused: true });
    for (const view of views) {
      camera(tl, view);
      cursor(tl, view, data.path);
      ripples(tl, view, data.clicks);
    }
    entrances(tl);
    return tl;
  };

  // Times from the take, for the camera keys: step start, nth key press with a label.
  window.stepAt = (id) => window.TUTORIAL.steps.find((s) => s.id === id).t;
  window.keyAt = (label, n = 0) => window.TUTORIAL.keys.filter((k) => k[1] === label)[n][0];
})();
