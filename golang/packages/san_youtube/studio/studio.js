// Helpers the tool runs in YouTube's pages, as window.__sanYouTube. They read
// the page and never change it: every change is a real click or real typing
// from the Go side. The selectors are YouTube's and Studio's markup as of
// 2026-10-13; a redesign is a change here and in upload.go's selectors.
window.__sanYouTube = (() => {
  // Shown: laid out, and not faded out. After Save, Studio fades the upload
  // dialog to opacity 0 and leaves it laid out (2026-10-13).
  const visible = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return false;
    for (let x = e; x && x !== document.body; x = x.parentElement) {
      if (getComputedStyle(x).opacity === '0') return false;
    }
    return true;
  };
  const first = (sel) => [...document.querySelectorAll(sel)].find(visible) || null;
  const text = (e) => (e ? (e.innerText || '').replace(/\s+/g, ' ').trim() : '');
  const dialog = () => first('ytcp-uploads-dialog tp-yt-paper-dialog') && document.querySelector('ytcp-uploads-dialog');

  return {
    // Where the tab is, and whether it asks for a sign-in.
    state() {
      return {
        url: location.href,
        signin: location.host === 'accounts.google.com' ||
          !!first('ytd-masthead a[href*="accounts.google.com/ServiceLogin"]'),
      };
    },

    // youtube.com's home page signed in: the account button and Create.
    homeReady() {
      return !!first('ytd-masthead #avatar-btn') && !!first('ytd-masthead #end #buttons ytd-button-renderer button');
    },

    // The rect of the first shown match of sel, as a string, so that two
    // reads can be compared; null while there is none. An element outside the
    // window is scrolled into the middle of it first.
    rect(sel) {
      const e = first(sel);
      if (!e) return null;
      const r = e.getBoundingClientRect();
      if (r.top < 0 || r.bottom > innerHeight) {
        e.scrollIntoView({ block: 'center' });
        return null;
      }
      return [r.x, r.y, r.width, r.height].map(Math.round).join(',');
    },

    shown(sel) { return !!first(sel); },

    // Studio's channel name, from its left menu.
    channel() { return text(document.querySelector('#entity-name')); },

    // The video's link, which the dialog shows as soon as Studio has made
    // the video: https://youtube.com/shorts/<id> or https://youtu.be/<id>.
    link() {
      const d = dialog();
      if (!d) return '';
      const a = [...d.querySelectorAll('a[href]')].find((a) => /^https:\/\/(youtube\.com\/shorts\/|youtu\.be\/)/.test(a.href));
      return a ? a.href : '';
    },

    // Puts the caret in the field sel and selects what it holds, so typing
    // replaces it.
    selectField(sel) {
      const e = first(sel);
      if (!e) return false;
      e.focus();
      if (e.select) {
        e.select();
      } else {
        getSelection().selectAllChildren(e);
      }
      return true;
    },

    // What the details step holds.
    details() {
      const d = dialog();
      if (!d) return null;
      const box = (sel) => {
        const e = d.querySelector(sel);
        return e ? e.innerText.replace(/\n$/, '') : null;
      };
      const kids = d.querySelector('tp-yt-paper-radio-button[name=VIDEO_MADE_FOR_KIDS_NOT_MFK]');
      return {
        title: box('#title-textarea #textbox'),
        description: box('#description-textarea #textbox'),
        notForKids: !!kids && kids.classList.contains('iron-selected'),
        tags: [...d.querySelectorAll('#chip-bar ytcp-chip')].map((c) => c.innerText.trim()),
        tagsShown: !!first('ytcp-uploads-dialog #chip-bar input#text-input'),
      };
    },

    // The visibility step: which choice is selected, and the schedule.
    visibility() {
      const d = dialog();
      if (!d || !first('ytcp-uploads-dialog #done-button')) return null;
      const radio = [...d.querySelectorAll('tp-yt-paper-radio-button')].find((r) => visible(r) && r.classList.contains('iron-selected'));
      const s = first('ytcp-uploads-dialog ytcp-visibility-scheduler');
      return {
        selected: radio ? radio.getAttribute('name') : '',
        privateShown: !!first('ytcp-uploads-dialog tp-yt-paper-radio-button[name=PRIVATE]'),
        scheduleShown: !!s,
        date: s ? text(s.querySelector('#datepicker-trigger')) : '',
        time: s ? (s.querySelector('#time-of-day-container input') || {}).value || '' : '',
        timeInvalid: !!s && !!s.querySelector('#time-of-day-container tp-yt-paper-input[invalid]'),
        datePickerOpen: !!first('ytcp-date-picker tp-yt-paper-input#textbox input'),
        done: text(d.querySelector('#done-button')),
        doneDisabled: !!d.querySelector('#done-button[aria-disabled=true]'),
      };
    },

    // The browser's offset from UTC, in minutes east, at the instant ms: the
    // time zone Studio's "Local Time" schedules in.
    tzOffset(ms) { return -new Date(ms).getTimezoneOffset(); },

    // Studio's progress line at the dialog's foot, for messages.
    progress() {
      const d = dialog();
      return d ? text(d.querySelector('ytcp-video-upload-progress')) : '';
    },

    // Whether the upload dialog is still open.
    dialogOpen() { return !!dialog(); },

    // Whether the upload dialog's form has gone: Save has taken effect.
    formGone() { return !first('ytcp-uploads-dialog #done-button'); },

    // A dialog Studio shows after saving, such as "Video processing" or
    // "Video scheduled": its close button, marked with data-san-close for the
    // tool to click, and the dialog's text; null when there is none.
    afterDialog() {
      const b = first('tp-yt-paper-dialog #close-button');
      if (!b) return null;
      document.querySelectorAll('[data-san-close]').forEach((e) => e.removeAttribute('data-san-close'));
      b.setAttribute('data-san-close', '');
      return { sel: '[data-san-close]', text: text(b.closest('tp-yt-paper-dialog')).slice(0, 120) };
    },

    // The shown dialogs, for an error message.
    dialogs() {
      return [...document.querySelectorAll('tp-yt-paper-dialog')].filter(visible).map((d) => {
        const host = d.getRootNode().host || d.parentElement;
        const buttons = [...d.querySelectorAll('ytcp-button, ytcp-icon-button, button')].filter(visible)
          .map((b) => `${b.tagName.toLowerCase()}#${b.id}`);
        return `${host ? host.tagName.toLowerCase() : ''} > ${text(d).slice(0, 80)} [${buttons.join(' ')}]`;
      });
    },

    // The content list's row of video id, on the tab shown: its title,
    // visibility and date as Studio shows them; null when it is not listed.
    row(id) {
      // A saved video's row links to /video/<id>/edit; a draft's has no link,
      // but names the id inside.
      const rows = [...document.querySelectorAll('ytcp-video-row')];
      const r = rows.find((r) => r.querySelector(`a[href*="/video/${id}/"]`)) || rows.find((r) => r.outerHTML.includes(id));
      if (!r) return null;
      return {
        title: text(r.querySelector('#video-title')),
        visibility: text(r.querySelector('.tablecell-visibility')),
        date: text(r.querySelector('.tablecell-date')),
      };
    },

    // The upload dialog's close (X) button, marked data-san-close so that the
    // Go side can give it a real click; false when there is none.
    markClose() {
      document.querySelectorAll('[data-san-close]').forEach((e) => e.removeAttribute('data-san-close'));
      const d = document.querySelector('ytcp-uploads-dialog');
      if (!d) return false;
      const c = [...d.querySelectorAll('[id*=close i], [aria-label]')].filter(visible)
        .find((e) => /close/i.test(e.id) || /^(close|tutup)$/i.test(e.getAttribute('aria-label') || ''));
      if (!c) return false;
      c.setAttribute('data-san-close', '');
      return true;
    },

    // The upload dialog's buttons, for an error when Studio's markup changed.
    dialogButtons() {
      const d = document.querySelector('ytcp-uploads-dialog');
      if (!d) return [];
      return [...d.querySelectorAll('button, [role=button], ytcp-icon-button, ytcp-button')].filter(visible)
        .map((e) => `${e.tagName.toLowerCase()}#${e.id} "${e.getAttribute('aria-label') || text(e)}"`).slice(0, 20);
    },

    // Every row of the content list's open tab, as row reads one.
    rows() {
      return [...document.querySelectorAll('ytcp-video-row')].map((r) => ({
        title: text(r.querySelector('#video-title')),
        visibility: text(r.querySelector('.tablecell-visibility')),
        date: text(r.querySelector('.tablecell-date')),
      }));
    },
  };
})();
