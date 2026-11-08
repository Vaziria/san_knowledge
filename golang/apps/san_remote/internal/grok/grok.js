// What san_remote knows of Grok's page (grok.com), as found on 2026-11-08
// with the interface in English (lang en, the account Persatuan Bangsa,
// Free): the prompt box, the Private chat toggle, and the turns of a chat.
// Like gemini.js, it runs in the extension's isolated world, where the page
// cannot see it.
//
// grok.com's class names are generated (Tailwind), so controls are found by
// their aria-labels and data-testid attributes:
//   - the prompt box is a tiptap (ProseMirror) editor, "Ask Grok anything",
//     in form[data-composer]; execCommand's insertText reaches it
//   - Send is button[data-testid=chat-submit] ("Submit"), aria-disabled
//     true while the prompt box is empty
//   - each prompt is a [data-testid=user-message], each answer a
//     [data-testid=assistant-message], both inside a div#response-<id> that
//     also holds the turn's action buttons; an answer's text is its
//     .response-content-markdown (the "Thought for" part above it is not)
//   - a written answer has "Copy response", "Regenerate", "Like"
//   - "Private" (aria-label "Switch to Private Chat") makes the next chat a
//     private one, which "won't appear in your history"; the toggle then
//     reads "Switch to Default Chat", and the chat's address ends in
//     #private
//   - the account button's aria-label is "Account menu, <name>"
//   - images go in through the composer's hidden file input
//     (input[type=file][name=files]), as on claude.ai; each shows as a chip
//     of the prompt box's "Conversation attachments" list, with its name
//     and, once uploaded, a preview from assets.grok.com
globalThis.__sanGrok = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const label = (l) => [...document.querySelectorAll(`button[aria-label="${l}"], a[aria-label="${l}"]`)].find(vis);
  const composer = () => [...document.querySelectorAll('form[data-composer]')].find(vis) ?? document.querySelector('form[data-composer]');
  const editor = () =>
    [...document.querySelectorAll('form[data-composer] .ProseMirror[contenteditable=true], .tiptap.ProseMirror[contenteditable=true]')].find(vis);
  const prompts = () => [...document.querySelectorAll('[data-testid="user-message"]')];
  const answers = () => [...document.querySelectorAll('[data-testid="assistant-message"]')];
  // turnOf is the div#response-<id> around an answer, which holds its
  // action buttons.
  const turnOf = (a) => a?.closest('[id^="response-"]') ?? a?.parentElement ?? null;
  // markdownOf is an answer's text, without its thinking part.
  const markdownOf = (a) =>
    [...(a?.querySelectorAll('.response-content-markdown') ?? [])].find((m) => !m.closest('.thinking-container')) ?? null;
  const copyOf = (a) => turnOf(a)?.querySelector('button[aria-label="Copy response"]') ?? null;
  const submit = () => (composer() ?? document).querySelector('button[data-testid="chat-submit"], button[aria-label="Submit"]');
  const account = () => [...document.querySelectorAll('button[aria-label^="Account menu"]')].find(vis);
  // press clicks e as a mouse does: the focus leaves the editor first.
  const press = (e) => {
    if (!e) return false;
    document.activeElement?.blur?.();
    e.focus?.();
    e.click();
    return true;
  };
  // Images an ask attaches come in base64 pieces (internal/attach); each
  // becomes a Blob at once.
  const staged = new Map();
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };
  // filesOf takes the staged files ([{id, name, type}]) as a DataTransfer,
  // or null when one is not staged.
  const filesOf = (files) => {
    const dt = new DataTransfer();
    for (const f of files) {
      const parts = staged.get(f.id);
      if (!parts) return null;
      staged.delete(f.id);
      dt.items.add(new File(parts, f.name, { type: f.type, lastModified: Date.now() }));
    }
    return dt;
  };

  return {
    // state is what the page shows, for the waits.
    state() {
      const a = answers().at(-1);
      const md = markdownOf(a);
      const send = submit();
      const busy = [...document.querySelectorAll('button[aria-label]')].some((b) => vis(b) && /^Stop/.test(b.getAttribute('aria-label')));
      return {
        url: location.href,
        lang: document.documentElement.lang,
        composer: !!editor(),
        signedOut:
          !account() &&
          [...document.querySelectorAll('a, button')].filter(vis).some((b) => /^(Sign in|Sign up|Log in)$/i.test((b.innerText || b.getAttribute('aria-label') || '').trim())),
        private: location.hash === '#private' || !!label('Switch to Default Chat'),
        privateButton: !!label('Switch to Private Chat'),
        turns: prompts().length,
        answers: answers().length,
        answered: !!md,
        chars: md ? md.innerText.trim().length : 0,
        copy: !!copyOf(a),
        busy,
        send: !!send && !send.disabled && send.getAttribute('aria-disabled') !== 'true',
      };
    },
    // account is the account's name, from its button's label ("Account
    // menu, Name").
    account() {
      const b = account();
      if (!b) return null;
      return { name: b.getAttribute('aria-label').replace(/^Account menu,?\s*/, '').trim() };
    },
    private: () => press(label('Switch to Private Chat')),
    // write replaces what the prompt box holds with text, as one insertion.
    write(text) {
      const ed = editor();
      if (!ed) return false;
      ed.focus();
      const r = document.createRange();
      r.selectNodeContents(ed);
      const s = getSelection();
      s.removeAllRanges();
      s.addRange(r);
      return document.execCommand('insertText', false, text);
    },
    // prompt is what the prompt box holds.
    prompt: () => editor()?.innerText ?? null,
    send: () => press(submit()),
    // answer is the last answer as the page shows it, as plain text, with
    // its links as markdown, [text](url), as the Copy button gives them:
    // what is read when the Copy button gives nothing. The text is the
    // shown one's (innerText keeps its paragraphs, which a copy out of the
    // page would lose), each link's text in it made a link in turn.
    answer() {
      const md = markdownOf(answers().at(-1));
      if (!md) return null;
      let text = md.innerText.trim();
      let from = 0;
      for (const l of md.querySelectorAll('a[href]')) {
        const t = l.innerText.trim();
        const at = t ? text.indexOf(t, from) : -1;
        if (at < 0) continue;
        const link = `[${t}](${l.href})`;
        text = text.slice(0, at) + link + text.slice(at + t.length);
        from = at + link.length;
      }
      return text;
    },
    // turnText is the last answer's whole text, for an error Grok shows in
    // place of an answer.
    turnText: () => turnOf(answers().at(-1))?.innerText ?? '',
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // attach gives the prompt box the staged files, as if they were chosen
    // in its file dialog: "" or what went wrong.
    attach(files) {
      const input = (composer() ?? document).querySelector('input[type=file][name=files]') ?? document.querySelector('input[type=file]');
      if (!input) return 'no file input in the prompt box';
      const dt = filesOf(files);
      if (!dt) return 'a file is not staged';
      input.files = dt.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // attachments: the files the prompt box shows, by name (each chip of
    // its "Conversation attachments" list), and whether one still uploads:
    // a chip whose preview is not yet Grok's own copy (assets.grok.com once
    // uploaded, 2026-11-08), or that shows a spinner.
    attachments() {
      const f = composer();
      const list = f?.querySelector('[role=list][aria-label="Conversation attachments"]');
      const chips = [...(list?.querySelectorAll('[role=listitem]') ?? [])];
      const names = chips.map((c) => c.querySelector('button[aria-label="Open attachment"] span')?.textContent.trim() ?? '').filter(Boolean);
      const uploading = chips.some((c) => {
        const img = c.querySelector('img');
        return /^(blob|data):/.test(img?.src ?? '') || !!c.querySelector('[role=progressbar], .animate-spin, [aria-busy=true]');
      });
      return { names, count: chips.length, uploading, problem: '' };
    },
  };
})();
