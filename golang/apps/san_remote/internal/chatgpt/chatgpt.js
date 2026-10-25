// What san_remote knows of ChatGPT's page, as found on 2026-10-25 with the
// interface in English (lang en-US, the account Get Resolved, Free): the
// composer, the Temporary chat button, and the turns of a chat. Like x.js, it
// runs in the extension's isolated world, where the page cannot see it.
//
// ChatGPT's class names are generated, so controls are found by their
// aria-labels and the data- attributes of the turns:
//   - the composer is a ProseMirror editor ("Ask ChatGPT"); execCommand's
//     insertText reaches it, and each line becomes a paragraph
//   - each turn is a [data-turn-key], holding the prompt and then the answer,
//     with data-talvt-turn-state in_progress, then complete
//   - while ChatGPT writes, the composer's button is "Stop"; then "Send"
globalThis.__sanChatGPT = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const label = (l) => [...document.querySelectorAll(`button[aria-label="${l}"]`)].find(vis);
  const editor = () => [...document.querySelectorAll('.ProseMirror[contenteditable=true], #prompt-textarea[contenteditable=true]')].find(vis);
  const turns = () => [...document.querySelectorAll('[data-turn-key]')];
  const lastTurn = () => turns().at(-1);
  // The answer of a turn: what follows ChatGPT's own heading ("ChatGPT
  // said:", marked data-conversation-role=assistant).
  const answerOf = (t) => {
    const h = t?.querySelector('[data-conversation-role=assistant]');
    if (!h) return null;
    const md = [...t.querySelectorAll('[data-markdown-copy]')].filter((m) => !m.parentElement.closest('[data-markdown-copy]'));
    return md.length ? md : [h.parentElement];
  };
  // press clicks e as a mouse does: the focus leaves the editor first.
  const press = (e) => {
    if (!e) return false;
    document.activeElement?.blur?.();
    e.focus?.();
    e.click();
    return true;
  };

  return {
    // state is what the page shows, for the waits.
    state() {
      const t = lastTurn();
      const send = label('Send') ?? document.querySelector('[data-testid=send-button]');
      return {
        url: location.href,
        lang: document.documentElement.lang,
        composer: !!editor(),
        signedOut: [...document.querySelectorAll('button, a')].filter(vis).some((b) => /^(Log in|Sign up for free)$/.test(b.innerText.trim())),
        challenge: /Just a moment/.test(document.title) || !!document.querySelector('iframe[src*="challenges.cloudflare.com"], #challenge-form'),
        temporary: new URL(location.href).searchParams.get('temporary-chat') === 'true',
        temporaryButton: !!label('Temporary chat'),
        turns: turns().length,
        turnState: t?.querySelector('[data-talvt-turn-state]')?.dataset.talvtTurnState ?? '',
        answered: !!answerOf(t),
        // chars is how long the answer is so far: a new turn reads complete
        // for a moment before the answer starts (2026-10-25).
        chars: (answerOf(t) ?? []).reduce((n, m) => n + m.innerText.trim().length, 0),
        busy: !!(label('Stop') ?? label('Stop streaming') ?? document.querySelector('[data-testid=stop-button]')),
        send: !!send && !send.disabled,
      };
    },
    // account is the name and plan the profile button shows ("Get
    // Resolved", "Free").
    account() {
      const b = [...document.querySelectorAll('button[aria-label="Open profile menu"]')].find((x) => vis(x) && x.getAttribute('aria-busy') !== 'true');
      if (!b) return null;
      const lines = b.innerText.split('\n').map((l) => l.trim()).filter(Boolean);
      return { name: lines[0] ?? '', plan: lines[1] ?? '' };
    },
    temporary: () => press(label('Temporary chat')),
    // write replaces what the composer holds with text, as one insertion.
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
    // prompt is what the composer holds.
    prompt: () => editor()?.innerText ?? null,
    send: () => press(label('Send') ?? document.querySelector('[data-testid=send-button]')),
    // answer is the last turn's answer as the page shows it, as plain text:
    // what is read when the Copy button gives nothing.
    answer() {
      const md = answerOf(lastTurn());
      return md ? md.map((m) => m.innerText).join('\n\n').trim() : null;
    },
    // turnText is the last turn's whole text, for an error ChatGPT shows in
    // place of an answer.
    turnText: () => lastTurn()?.innerText ?? '',
  };
})();
