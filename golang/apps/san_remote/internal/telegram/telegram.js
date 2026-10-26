// What san_remote knows of Telegram Web K (web.telegram.org/k), found
// 2026-10-27 in the user's signed-in browser and in the client's source
// (github.com/morethanwords/tweb). It runs in the extension's isolated world,
// where the page cannot see it. Labels are read in English.
//
// What the steps use:
//   - the left column (#column-left): the chat list, its search, the New
//     Chats menu (#new-menu: New Group), and the pages New Group opens: Add
//     Members (.add-members-container), then the group's name
//     (.new-group-container), each with a Next corner button
//   - the open chat (#column-center .chat.active): its title bar (.chat-info,
//     which opens the profile; More, whose Delete opens the delete popup),
//     the message box (a ProseMirror editor; execCommand insertText reaches
//     it) and the send button, and the messages (.bubble[data-mid])
//   - the profile (#column-right): the Members list, whose right-click menu
//     has Remove from group, and the Add Members corner button
//   - popups (.popup: the delete and add confirmations) and toasts
//
// A chat row opens on mousedown, so rows and menus get a whole press (tap);
// buttons get a click.
globalThis.__sanTelegram = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const all = (sel, root = document) => (root ? [...root.querySelectorAll(sel)].filter(vis) : []);
  const first = (sel, root = document) => all(sel, root)[0] ?? null;
  // text drops the icon font's glyphs (private use characters), which come
  // first in menu items and buttons.
  const text = (e) => (e ? (e.innerText || e.textContent || '').replace(/[-]/g, '').replace(/\s+/g, ' ').trim() : '');
  const press = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    e.click();
    return true;
  };
  const at = (e) => {
    const r = e.getBoundingClientRect();
    return { bubbles: true, cancelable: true, view: window, button: 0, clientX: r.left + r.width / 2, clientY: r.top + r.height / 2 };
  };
  const tap = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    const o = at(e);
    for (const type of ['mousedown', 'mouseup', 'click']) e.dispatchEvent(new MouseEvent(type, o));
    return true;
  };
  // typeInto selects what the field holds and types t over it, as an input
  // method or a paste would.
  const typeInto = (e, t) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    e.focus();
    if (e.select) {
      e.select();
    } else {
      const r = document.createRange();
      r.selectNodeContents(e);
      getSelection().removeAllRanges();
      getSelection().addRange(r);
    }
    if (!document.execCommand('insertText', false, t)) {
      if ('value' in e) {
        e.value = t;
      } else {
        e.textContent = t;
      }
      e.dispatchEvent(new InputEvent('input', { bubbles: true, composed: true, inputType: 'insertText', data: t }));
    }
    return true;
  };

  const left = () => document.querySelector('#column-left .sidebar-slider-item.active');
  const right = () => (document.body.classList.contains('is-right-column-shown') ? document.querySelector('#column-right .sidebar-slider-item.active') : null);
  const side = (where) => (where === 'right' ? right() : left());
  const chat = () => document.querySelector('#column-center .chat.active');
  const rowsIn = (root) => all('.chatlist-chat[data-peer-id]', root).map((e) => ({ peer: e.dataset.peerId, title: text(e.querySelector('.peer-title')) }));
  const rowIn = (root, peer) => all(`.chatlist-chat[data-peer-id="${CSS.escape(peer)}"]`, root)[0] ?? null;
  const menu = () => first('.btn-menu.active');
  const menuItem = (label) => {
    const m = menu();
    return m ? all('.btn-menu-item', m).find((i) => text(i) === label) ?? null : null;
  };
  const popup = () => all('.popup').filter((p) => !p.classList.contains('hide')).pop() ?? null;
  // A popup's buttons are drawn in capitals (text-transform), which
  // innerText gives: "ADD" (2026-10-27). Labels are matched in any case.
  const named = (e, label) => text(e).toLowerCase() === label.toLowerCase();
  const button = (root, label) => (root ? all('button', root).find((b) => named(b, label)) ?? null : null);
  // A popup's check box is a row: label.popup-peer-checkbox-row, its name in
  // .row-title, its box a span.checkbox-field (2026-10-27).
  const checks = (root) => (root ? all('label', root).filter((l) => l.querySelector('input[type=checkbox]')) : []);
  const checkName = (l) => text(l.querySelector('.row-title') ?? l);
  const checkbox = (root, label) => checks(root).find((l) => checkName(l).toLowerCase() === label.toLowerCase()) ?? null;
  const mid = (b) => parseFloat(b.dataset.mid);
  // message is a bubble's text without its time and other parts.
  const message = (b) => {
    const m = b.querySelector('.message');
    if (!m) return '';
    const c = m.cloneNode(true);
    c.querySelectorAll('.time, .clearfix, .reactions, .bubble-beside-button, .reply-markup').forEach((x) => x.remove());
    return (c.textContent || '').trim();
  };
  const bubbles = () => {
    const c = chat();
    return c ? [...c.querySelectorAll('.bubble[data-mid]')].filter((b) => !b.classList.contains('service') && !isNaN(mid(b))) : [];
  };
  const sendButton = () => first('.btn-send', chat());

  return {
    // state is where the tab is, for the waits.
    state() {
      const l = left();
      const c = chat();
      const p = popup();
      const lc = l?.classList;
      return {
        url: location.href,
        lang: document.documentElement.lang,
        visible: document.visibilityState === 'visible',
        // Signed in: Telegram Web keeps the account's key in localStorage.
        auth: !!localStorage.getItem('user_auth'),
        signin: !!first('#auth-pages') || !!first('.page-signQR, .page-sign'),
        ready: !!document.querySelector('#column-left .item-main .chatlist-chat') && !!first('#new-menu'),
        leftPage: !lc ? '' : lc.contains('item-main') ? (lc.contains('is-search-active') ? 'search' : 'main') :
          lc.contains('new-group-container') ? 'new-group' : lc.contains('add-members-container') ? 'add-members' : 'other',
        rightPage: !right() ? '' : right().classList.contains('add-members-container') ? 'add-members' :
          right().classList.contains('shared-media-container') ? 'profile' : 'other',
        chat: c?.querySelector('.chat-info .peer-title')?.dataset.peerId ?? '',
        chatTitle: text(c?.querySelector('.chat-info .peer-title')),
        input: !!first('.input-message-input', c),
        menu: !!menu(),
        popup: p ? [...p.classList].filter((x) => x.startsWith('popup-') && x !== 'popup-peer').join(' ') || 'popup' : '',
        toast: all('.toast').map(text).join(' | '),
      };
    },

    // ---- the chat list -----------------------------------------------------------
    // rows are the chat rows the left column shows: the chat list, or the
    // search's results.
    rows: () => rowsIn(left()),
    openRow: (peer) => tap(rowIn(left(), peer)),
    search: (q) => typeInto(first('.sidebar-header .input-search-input', document.querySelector('#column-left')), q),
    closeSearch: () => press(first('#column-left .sidebar-header .sidebar-back-button')),
    pressNewMenu: () => tap(first('#new-menu')),
    // pressMenuItem presses the shown menu's item named label.
    pressMenuItem: (label) => tap(menuItem(label)),
    menuItems: () => (menu() ? all('.btn-menu-item', menu()).map(text) : []),
    closeMenu() {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', keyCode: 27, bubbles: true }));
      return true;
    },

    // ---- the open chat ------------------------------------------------------------
    pressChatMenu: () => tap(all('.chat-utils .btn-menu-toggle', chat()).find((b) => b.getAttribute('aria-label') === 'More') ?? null),
    lastMid: () => bubbles().reduce((m, b) => Math.max(m, mid(b)), 0),
    // after is the messages after mid, oldest first: who sent each, and its
    // text.
    after: (m) => bubbles().filter((b) => mid(b) > m).sort((a, b) => mid(a) - mid(b))
      .map((b) => ({ mid: mid(b), out: b.classList.contains('is-out'), text: message(b) })),
    typeMessage: (t) => typeInto(first('.input-message-input', chat()), t),
    draft: () => (first('.input-message-input', chat())?.innerText ?? '').trim(),
    // canSend: the send button is in its send state (it records voice
    // messages while the box is empty).
    canSend: () => !!sendButton()?.classList.contains('send'),
    pressSend() {
      const b = sendButton();
      return !!b && b.classList.contains('send') && press(b);
    },

    // ---- the profile --------------------------------------------------------------
    pressProfile: () => press(first('.chat-info', chat())),
    closeProfile: () => press(first('.sidebar-header .sidebar-close-button', right())),
    // pressTab presses the profile's tab named label (Members).
    pressTab: (label) => press(all('.search-super-tabs .menu-horizontal-div-item', right()).find((t) => text(t) === label) ?? null),
    tabs: () => all('.search-super-tabs .menu-horizontal-div-item', right()).map(text),
    members: () => rowsIn(right()?.querySelector('.search-super-content-members') ?? right()),
    // moreMembers scrolls the profile down, for the members it loads as it
    // scrolls.
    moreMembers() {
      const s = right()?.querySelector('.scrollable-y');
      if (!s) return false;
      s.scrollTop = s.scrollHeight;
      return true;
    },
    pressAddMembers: () => press(all('.btn-corner', right()).find((b) => b.getAttribute('aria-label') === 'Add Members' && !b.classList.contains('is-hidden')) ?? null),
    // openMemberMenu right-clicks a member's row.
    openMemberMenu(peer) {
      const r = rowIn(right(), peer);
      if (!r) return false;
      r.scrollIntoView({ block: 'center' });
      r.dispatchEvent(new MouseEvent('contextmenu', at(r)));
      return true;
    },

    // ---- Add Members and New Group (where: "left" or "right") ---------------------
    pickerSearch: (where, q) => typeInto(first('input.selector-search-input', side(where)), q),
    pickerRows: (where) => rowsIn(side(where)),
    // pick ticks a row; picked says whether it is ticked: a chip above the
    // search (which a tick clears, so the row is gone), or the row's box.
    pick: (where, peer) => press(rowIn(side(where), peer)),
    picked: (where, peer) => !!side(where)?.querySelector(`.selector-user[data-key="${CSS.escape(peer)}"]`) ||
      !!rowIn(side(where), peer)?.querySelector('input[type=checkbox]')?.checked,
    pressNext: (where) => press(all('.btn-corner', side(where)).find((b) => b.classList.contains('is-visible') && b.getAttribute('aria-label') === 'Next') ?? null),
    typeGroupName: (t) => typeInto(first('.input-field-input', left()), t),
    groupName: () => (first('.input-field-input', left())?.innerText ?? first('.input-field-input', left())?.value ?? '').trim(),

    // ---- popups -----------------------------------------------------------------------
    // popupView is the shown popup: its title, text, buttons and check
    // boxes.
    popupView() {
      const p = popup();
      if (!p) return null;
      return {
        title: text(p.querySelector('.popup-title')),
        text: text(p.querySelector('.popup-description')),
        buttons: all('button', p).map(text).filter(Boolean),
        checks: checks(p).map((l) => ({ label: checkName(l), checked: !!l.querySelector('input[type=checkbox]')?.checked })),
      };
    },
    pressPopupButton: (label) => press(button(popup(), label)),
    tickPopup: (label) => press(checkbox(popup(), label)),
    closePopup: () => press(first('.popup-close, .btn-icon.popup-close', popup())) || press(button(popup(), 'Cancel')),
  };
})();
