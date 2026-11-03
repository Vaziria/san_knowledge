// What san_remote knows of Gemini's page, as found on 2026-10-25 with the
// interface in English (lang en-GB): the prompt box, the Temporary chat
// button, and the turns of a chat. Like chatgpt.js, it runs in the
// extension's isolated world, where the page cannot see it.
//
// Gemini is an Angular app with generated class names, so controls are found
// by their aria-labels and its own element names:
//   - the prompt box is a Quill editor (rich-textarea .ql-editor, "Enter a
//     prompt for Gemini"); execCommand's insertText reaches it
//   - each prompt is a user-query, each answer a model-response whose
//     message-content holds a .markdown with aria-busy, false once written
//   - while Gemini writes, the button is "Stop response"; once done the
//     answer has "Copy", "Redo" and "Show more options"
//   - a new chat is a Temporary chat after the "Temporary chat" button: its
//     greeting says "Temporary chats ... don't appear in recent chats", and
//     once a prompt is sent a "Temporary chat" label stands in the header
//   - the prompt box has no file input until its "Upload & tools" menu
//     opens a file dialog, so images are pasted into it, as a person pastes
//     a copied image (2026-11-03); the first upload of an account asks to
//     agree to "Creating content from images and files" in a dialog
globalThis.__sanGemini = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const label = (l) => [...document.querySelectorAll(`button[aria-label="${l}"]`)].find(vis);
  const editor = () => [...document.querySelectorAll('.ql-editor[contenteditable=true]')].find(vis);
  const queries = () => [...document.querySelectorAll('user-query')];
  const responses = () => [...document.querySelectorAll('model-response')];
  const markdownOf = (r) => r?.querySelector('message-content .markdown');
  // leaf says whether a shown element without children has exactly text.
  const leaf = (text) =>
    [...document.querySelectorAll('span, div, h1, h2, p')].some((e) => e.children.length === 0 && vis(e) && e.textContent.trim() === text);
  // press clicks e as a mouse does: the focus leaves the editor first.
  const press = (e) => {
    if (!e) return false;
    document.activeElement?.blur?.();
    e.focus?.();
    e.click();
    return true;
  };
  // area is the prompt box's whole area, which shows the files attached.
  const area = () => editor()?.closest('input-area-v2') ?? document.querySelector('input-area-v2');
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
      const r = responses().at(-1);
      const md = markdownOf(r);
      const greeting = [...document.querySelectorAll('h1')].filter(vis).map((h) => h.innerText).join(' ');
      return {
        url: location.href,
        lang: document.documentElement.lang,
        composer: !!editor(),
        signedOut: [...document.querySelectorAll('a, button')].filter(vis).some((b) => /^Sign in$/.test((b.innerText || b.getAttribute('aria-label') || '').trim())),
        temporary: /Temporary chats/.test(greeting) || leaf('Temporary chat'),
        temporaryButton: !!label('Temporary chat'),
        turns: queries().length,
        answers: responses().length,
        answered: !!md,
        chars: md ? md.innerText.trim().length : 0,
        written: !!md && md.getAttribute('aria-busy') === 'false',
        copy: !!r?.querySelector('button[aria-label="Copy"]'),
        busy: !!label('Stop response'),
        send: !!label('Send message'),
      };
    },
    // account is the Google account's name, from its avatar's label
    // ("Google Account: Name (email)").
    account() {
      const a = [...document.querySelectorAll('a[aria-label^="Google Account:"], button[aria-label^="Google Account:"]')].find(vis);
      if (!a) return null;
      const m = a.getAttribute('aria-label').match(/^Google Account:\s*(.*?)\s*(\(.*\))?$/);
      return { name: m ? m[1] : '' };
    },
    temporary: () => press(label('Temporary chat')),
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
    send: () => press(label('Send message')),
    // answer is the last answer as the page shows it, as plain text: what is
    // read when the Copy button gives nothing.
    answer: () => markdownOf(responses().at(-1))?.innerText?.trim() ?? null,
    // turnText is the last answer's whole text, for an error Gemini shows in
    // place of an answer.
    turnText: () => responses().at(-1)?.innerText ?? '',
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // attach pastes the staged files into the prompt box: "" or what went
    // wrong.
    attach(files) {
      const ed = editor();
      if (!ed) return 'no prompt box';
      const dt = filesOf(files);
      if (!dt) return 'a file is not staged';
      ed.focus();
      ed.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
      return '';
    },
    // attachments: how many files the prompt box shows (their previews, or
    // their remove buttons), whether one still uploads, and the dialog to
    // agree to before the first upload.
    attachments() {
      const a = area();
      const dialog = [...document.querySelectorAll('mat-dialog-container, [role=dialog]')].find(
        (d) => vis(d) && /Creating content from images and files/.test(d.innerText),
      );
      const removes = [...(a?.querySelectorAll('button[aria-label^="Remove"]') ?? [])].filter(vis);
      const previews = [...(a?.querySelectorAll('img') ?? [])].filter(vis);
      const uploading = [...(a?.querySelectorAll('[role=progressbar], mat-progress-spinner, mat-progress-bar, [aria-busy=true]') ?? [])].some(vis);
      return {
        names: [],
        count: Math.max(removes.length, previews.length),
        uploading,
        problem: dialog
          ? 'Gemini asks to agree to "Creating content from images and files" before its first upload: press Agree once in Gemini (gemini.google.com), then ask again'
          : '',
      };
    },
  };
})();
