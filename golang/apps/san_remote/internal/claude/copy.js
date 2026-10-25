// Runs in the page's own world, as the body of an async function: presses the
// Copy button of the last answer and returns what claude.ai writes to the
// clipboard, its text/plain (the answer's markdown) and text/html, or null.
//
// The clipboard calls are caught for this one press and put back before it
// returns, so nothing reaches the user's clipboard. claude.ai writes a
// ClipboardItem with both types (2026-10-25); writeText is caught too.
const cb = navigator.clipboard;
const write = cb.write, writeText = cb.writeText;
const own = Object.hasOwn(cb, 'write'), ownText = Object.hasOwn(cb, 'writeText');
let got = null;
cb.writeText = async (t) => { got = { text: String(t), html: '' }; };
cb.write = async (items) => {
  for (const it of items) {
    const o = { text: '', html: '' };
    if (it.types.includes('text/plain')) o.text = await (await it.getType('text/plain')).text();
    if (it.types.includes('text/html')) o.html = await (await it.getType('text/html')).text();
    got = o;
  }
};
try {
  const answer = [...document.querySelectorAll('[data-testid="assistant-message"]')].at(-1);
  // "Copy" in the answer is the answer's; the prompt's is data-testid
  // user-message-copy, outside it.
  const b = [...(answer?.querySelectorAll('button[aria-label="Copy"]') ?? [])].at(-1);
  if (!b) return null;
  b.click();
  for (let i = 0; i < 20 && !got; i++) await new Promise((r) => setTimeout(r, 100));
  return got;
} finally {
  if (own) cb.write = write;
  else delete cb.write;
  if (ownText) cb.writeText = writeText;
  else delete cb.writeText;
}
