// Installed with Page.addScriptToEvaluateOnNewDocument, so it runs before any
// application code and before the app captures its own reference to fetch.
//
// The response body is teed, not intercepted: one half is handed back to the
// caller inside an equivalent Response so the UI streams and renders exactly
// as it would have, and the other half is read here and forwarded to Go. We
// never construct a request, and we never alter one, because the request is
// the part carrying credentials and the proof of work we cannot reproduce.
(() => {
  if (window.__goGPTInstalled) return;
  window.__goGPTInstalled = true;

  const BINDING = '__goGPTChunk';
  // /backend-api/conversation, and the /f/ variant some builds route through.
  const CONVERSATION = /\/backend-api\/(f\/)?conversation$/;

  const report = (kind, data) => {
    try {
      const fn = window[BINDING];
      if (typeof fn === 'function') fn(JSON.stringify({ kind, data }));
    } catch (e) {
      // A failed report must never break the page.
    }
  };

  const pump = async (stream) => {
    const reader = stream.getReader();
    const decoder = new TextDecoder();
    report('open', '');
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        // stream: true so a multi-byte rune split across two reads is not
        // mangled into replacement characters.
        report('data', decoder.decode(value, { stream: true }));
      }
      const tail = decoder.decode();
      if (tail) report('data', tail);
    } catch (e) {
      report('error', String(e));
    } finally {
      try { reader.releaseLock(); } catch (e) { /* already released */ }
      report('close', '');
    }
  };

  const isConversation = (input) => {
    try {
      const raw = typeof input === 'string' ? input
        : (input instanceof Request ? input.url : (input && input.url) || '');
      if (!raw) return false;
      return CONVERSATION.test(new URL(raw, location.origin).pathname);
    } catch (e) {
      return false;
    }
  };

  const original = window.fetch;
  window.fetch = async function (input, init) {
    const response = await original.apply(this, arguments);
    try {
      if (!isConversation(input) || !response.body) return response;
      const [toApp, toGo] = response.body.tee();
      // Not awaited: both halves must be drained concurrently or the shared
      // buffer fills and the app's own read blocks behind ours.
      pump(toGo);
      return new Response(toApp, {
        status: response.status,
        statusText: response.statusText,
        headers: response.headers,
      });
    } catch (e) {
      report('error', String(e));
      return response;
    }
  };
})();
