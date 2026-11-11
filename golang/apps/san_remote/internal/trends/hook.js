// The hook on the page's own fetch and XMLHttpRequest, run in its main world
// (service.go hookJS): it keeps the replies of the related queries' call
// (batchexecute rpc fXqlme) as the page receives them, so san_remote reads
// exactly what the page fetched; the page's calls carry a reCAPTCHA token
// only it makes. readJS takes the first reply and puts fetch and
// XMLHttpRequest back.
if (!window.__sanTrendsHook) {
  const cap = [];
  const want = (u) => /\/batchexecute\?/.test(u) && /[?&]rpcids=[^&]*fXqlme/.test(u);
  const f0 = window.fetch;
  const open0 = XMLHttpRequest.prototype.open;
  const send0 = XMLHttpRequest.prototype.send;
  window.fetch = function (input) {
    const p = f0.apply(this, arguments);
    try {
      const u = String((input && input.url) || input);
      if (want(u)) {
        p.then(
          (r) => r.clone().text().then((text) => cap.push({ status: r.status, text }), () => cap.push({ status: r.status, text: '' })),
          (e) => cap.push({ status: 0, text: '', error: String(e) }),
        );
      }
    } catch (e) {
      // never in the page's way
    }
    return p;
  };
  XMLHttpRequest.prototype.open = function (method, u) {
    this.__sanTrendsUrl = String(u);
    return open0.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function () {
    if (want(this.__sanTrendsUrl || '')) {
      this.addEventListener('loadend', () => cap.push({ status: this.status, text: String(this.responseText || '') }));
    }
    return send0.apply(this, arguments);
  };
  window.__sanTrendsHook = {
    cap,
    undo() {
      window.fetch = f0;
      XMLHttpRequest.prototype.open = open0;
      XMLHttpRequest.prototype.send = send0;
      delete window.__sanTrendsHook;
    },
  };
}
return true;
