// What san_remote knows of claude.ai's page, as found on 2026-10-25 with the
// interface in English (lang en-US, the account sease, Free): the prompt box,
// the incognito button, and the messages of a chat. Like chatgpt.js, it runs
// in the extension's isolated world, where the page cannot see it.
//
// claude.ai's class names are generated, so controls are found by their
// aria-labels and data-testid attributes:
//   - the prompt box is a ProseMirror (tiptap) editor, data-testid chat-input
//     ("Write your prompt to Claude"); execCommand's insertText reaches it
//   - each prompt is a [data-testid=user-message], each answer a
//     [data-testid=assistant-message] with data-is-streaming true while
//     Claude writes, then false; its text is in .standard-markdown
//   - a written answer has "Copy", "Retry" and "Edit" (the prompt's own copy
//     button is data-testid user-message-copy)
//   - "Use incognito" makes the chat incognito: the address gains
//     ?incognito, which a sent chat keeps, and the button becomes "Exit
//     incognito"
//   - images go in through the prompt box's hidden file input
//     (data-testid file-upload); each shows as a [data-testid=file-thumbnail]
//     with "Remove <name>", data-state pending and aria-busy true while it
//     uploads (2026-11-03). Send stays enabled meanwhile.
globalThis.__sanClaude = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const label = (l) => [...document.querySelectorAll(`button[aria-label="${l}"]`)].find(vis);
  const editor = () =>
    [...document.querySelectorAll('[data-testid="chat-input"][contenteditable=true], .ProseMirror[contenteditable=true]')].find(vis);
  const prompts = () => [...document.querySelectorAll('[data-testid="user-message"]')];
  const answers = () => [...document.querySelectorAll('[data-testid="assistant-message"]')];
  // markdownOf is an answer's text blocks, outermost only.
  const markdownOf = (a) =>
    [...(a?.querySelectorAll('.standard-markdown') ?? [])].filter((m) => !m.parentElement.closest('.standard-markdown'));
  // press clicks e as a mouse does: the focus leaves the editor first.
  const press = (e) => {
    if (!e) return false;
    document.activeElement?.blur?.();
    e.focus?.();
    e.click();
    return true;
  };
  // box is the prompt box's fieldset, which holds the file input and the
  // thumbnails of the files attached.
  const box = () => editor()?.closest('fieldset') ?? document;
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
      const send = label('Send message') ?? document.querySelector('[data-testid="chat-input-send"]');
      return {
        url: location.href,
        lang: document.documentElement.lang,
        composer: !!editor(),
        signedOut:
          location.pathname.startsWith('/login') ||
          [...document.querySelectorAll('button')].filter(vis).some((b) => /^Continue with (Google|email)$/.test(b.innerText.trim())),
        incognito: new URL(location.href).searchParams.has('incognito') || !!label('Exit incognito'),
        incognitoButton: !!label('Use incognito'),
        turns: prompts().length,
        answers: answers().length,
        streaming: a?.dataset.isStreaming === 'true',
        answered: md.length > 0,
        chars: md.reduce((n, m) => n + m.innerText.trim().length, 0),
        copy: !!a?.querySelector('button[aria-label="Copy"]'),
        busy: [...document.querySelectorAll('button[aria-label]')].some((b) => vis(b) && /^Stop/.test(b.getAttribute('aria-label'))),
        send: !!send && !send.disabled,
      };
    },
    // account is the name and plan the account button shows ("sease",
    // "Free"); its first line is the avatar's initial.
    account() {
      const b = [...document.querySelectorAll('[data-testid="user-menu-button"]')].find(vis);
      if (!b) return null;
      const lines = b.innerText.split('\n').map((l) => l.trim()).filter((l) => l && l !== '·');
      if (lines.length >= 3) return { name: lines[1], plan: lines[2] };
      return { name: lines.at(-1) ?? '', plan: '' };
    },
    incognito: () => press(label('Use incognito')),
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
    send: () => press(label('Send message') ?? document.querySelector('[data-testid="chat-input-send"]')),
    // answer is the last answer as the page shows it, as plain text: what is
    // read when the Copy button gives nothing.
    answer() {
      const md = markdownOf(answers().at(-1));
      return md.length ? md.map((m) => m.innerText).join('\n\n').trim() : null;
    },
    // turnText is the last answer's whole text, for an error claude.ai shows
    // in place of an answer.
    turnText: () => answers().at(-1)?.innerText ?? '',
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // attach gives the prompt box the staged files, as if they were chosen
    // in its file dialog: "" or what went wrong.
    attach(files) {
      const input = box().querySelector('input[type=file][data-testid="file-upload"]') ?? document.querySelector('input[type=file][data-testid="file-upload"]');
      if (!input) return 'no file input in the prompt box';
      const dt = filesOf(files);
      if (!dt) return 'a file is not staged';
      input.files = dt.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // attachments: the files the prompt box shows, by name, and whether one
    // still uploads.
    attachments() {
      const b = box();
      const names = [...b.querySelectorAll('button[aria-label^="Remove "]')].filter(vis).map((x) => x.getAttribute('aria-label').slice(7));
      const tiles = [...b.querySelectorAll('[data-testid="file-thumbnail"]')];
      const uploading = tiles.some((t) => t.getAttribute('aria-busy') === 'true' || t.dataset.state === 'pending');
      return { names, count: tiles.length, uploading, problem: '' };
    },
  };
})();
