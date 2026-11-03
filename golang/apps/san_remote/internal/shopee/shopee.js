// What san_remote knows of Shopee Seller Centre (seller.shopee.co.id), as
// found on 2026-11-03 with the interface in Indonesian (the shop
// getresolved.id). Like the other sites' scripts, it runs in the extension's
// isolated world, where the page cannot see it.
//
// Reads come from the requests Seller Centre's own pages make (sent from
// here with the page's cookies, SPC_CDS in the query):
//   - /api/selleraccount/shop_info/, /api/selleraccount/user_info/: the shop
//   - /api/v3/opt/mpsku/list/v2/get_product_list (page_number, page_size,
//     list_type all|live_all|delisted|reviewing|banned|draft) and
//     search_product_list (keyword): Produk Saya
//   - /api/v3/opt/mpsku/list/get_bumped_product_list: Naikkan Produk's
//     slots, {config: {total_slots: 5, cooldown_seconds: 14400}, products:
//     [{id, cool_down_seconds}]}
//
// Changes are made by pressing the page's own controls:
//   - Produk Saya: the search box ("Cari Nama Produk, SKU Induk, Kode
//     Variasi, ID Produk") and Terapkan; a row's Lainnya menu (Salin,
//     Tampilkan, Hapus for an archived product; Salin, Tampilan Produk,
//     Arsipkan, Naikkan Produk, Atur Komisi Affiliate for a live one, where a
//     bumped product shows its countdown "03:59:26" instead). The product
//     rows and the action rows are two tables, row for row; a product row
//     holds a checkbox named with the product's id.
//   - Tambah Produk Baru (/portal/product/new): fields carry
//     data-product-edit-field-unique-id (images, promotionImages, name,
//     category, brandAndAttributes, description, variation, tierVariation_N,
//     priceModel_<uuid>, stockModel_<uuid>, skuModel_<uuid>, price, stock,
//     weight, dimension.*, dangersGoods, logistic, trafficAcceleration,
//     condition, parentSku). Inputs take a value through the native setter
//     and an input event (Vue). Photos go to the field's file input through
//     a DataTransfer; the photos open "Pembuat Foto AI", whose "Terapkan ke
//     Produk (N)" puts them in. The description is a Quill editor that takes
//     execCommand insertText and insertParagraph. A variation's next option
//     is typed into its "virtual" empty option.
globalThis.__sanShopee = (() => {
  const vis = (e) => {
    if (!e || !(e instanceof Element)) return false;
    const r = e.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return false;
    for (let x = e; x && x !== document.body; x = x.parentElement) {
      const s = getComputedStyle(x);
      if (s.display === 'none' || s.visibility === 'hidden' || s.opacity === '0') return false;
    }
    return true;
  };
  const all = (sel, root = document) => [...root.querySelectorAll(sel)].filter(vis);
  const text = (e) => (e ? (e.innerText || e.textContent || '').replace(/\s+/g, ' ').trim() : '');
  const own = (e) => (e ? (e.innerText || e.textContent || '').trim() : '');
  const same = (a, b) => text({ innerText: a }).toLowerCase() === text({ innerText: b }).toLowerCase();
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const press = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    e.click();
    return true;
  };
  const field = (id) => document.querySelector(`[data-product-edit-field-unique-id="${id}"]`);
  const fields = (prefix) => [...document.querySelectorAll(`[data-product-edit-field-unique-id^="${prefix}"]`)];
  // setValue types v into an input or textarea as Vue hears it: the native
  // setter, then input and change.
  const setValue = (inp, v) => {
    if (!inp) return false;
    inp.scrollIntoView({ block: 'center' });
    inp.focus();
    const proto = inp instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(inp, String(v));
    inp.dispatchEvent(new Event('input', { bubbles: true }));
    inp.dispatchEvent(new Event('change', { bubbles: true }));
    inp.blur();
    inp.dispatchEvent(new Event('blur', { bubbles: true }));
    return true;
  };
  const button = (label, root = document) => all('button', root).find((b) => own(b) === label);
  const errorsIn = (root) => all('.product-edit-form-item-error, .eds-form-item__error', root).map(own).filter(Boolean);
  const cds = () => (document.cookie.match(/(?:^|;\s*)SPC_CDS=([^;]+)/) || [])[1] || '';
  const api = async (path, params = {}) => {
    const q = new URLSearchParams({ SPC_CDS: cds(), SPC_CDS_VER: '2', ...params });
    const r = await fetch(`${path}?${q}`, { credentials: 'include', headers: { accept: 'application/json' } });
    const t = await r.text();
    let j;
    try { j = JSON.parse(t); } catch { return { code: -1, message: `HTTP ${r.status}: ${t.slice(0, 200)}` }; }
    return j;
  };
  const product = (p) => ({
    id: p.id,
    name: p.name,
    status: p.status,
    unlist: !!p.tag?.unlist,
    priceMin: Math.round(Number(p.price_detail?.price_min ?? 0)),
    priceMax: Math.round(Number(p.price_detail?.price_max ?? 0)),
    stock: p.stock_detail?.total_available_stock ?? 0,
    sold: p.statistics?.sold_count ?? 0,
    sku: p.parent_sku ?? '',
    cover: p.cover_image ? `https://down-id.img.susercontent.com/file/${p.cover_image}` : '',
    created: p.create_time ?? 0,
    modified: p.modify_time ?? 0,
    models: (p.model_list ?? []).map((m) => ({
      id: m.id,
      name: m.name ?? '',
      sku: m.sku ?? '',
      tier: m.tier_index ?? [],
      price: Math.round(Number(m.price_detail?.origin_price ?? 0)),
      stock: m.stock_detail?.total_available_stock ?? 0,
      default: !!m.is_default,
    })),
  });
  const staged = new Map();
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };
  // The product list's two tables: product rows (with a checkbox named by
  // the product's id) and action rows (Ubah, Iklankan, Lainnya), row for row.
  const productRows = () => [...document.querySelectorAll('tr.eds-table__row')].filter((r) => r.querySelector('input.eds-checkbox__input[name]:not([name=""])'));
  const actionRows = () => [...document.querySelectorAll('tr.eds-table__row')].filter((r) => r.querySelector('.list-view-action'));
  const rowOf = (id) => {
    const rows = productRows();
    const i = rows.findIndex((r) => r.querySelector('input.eds-checkbox__input').name === String(id));
    if (i < 0) return null;
    return { row: rows[i], actions: actionRows()[i] ?? null };
  };
  const menuEl = () => all('.more-dropdown-menu').find((m) => m.querySelector('li'));
  const modalBox = () => all('.eds-modal__box').pop() ?? null;
  // The attribute items of Spesifikasi, by name.
  const attrItems = () => [...document.querySelectorAll('[data-product-edit-field-unique-id="brandAndAttributes"] .attribute-select-item')];
  const attrItem = (name) => attrItems().find((i) => same(own(i.querySelector('.item-title-text')), name));
  // A select's list: a popover at the end of the page (Spesifikasi), or
  // inside the open select itself (Kondisi).
  const selectPopper = () => all('.eds-popper.eds-select-popover-content').pop() ?? all('.eds-select.is-show .eds-popper').pop() ?? null;
  const tier = (i) => field(`tierVariation_${i}`);
  const realOptions = (i) => [...(tier(i)?.querySelectorAll('.variation-option-panel .options-item:not(.virtual-options-item) input[placeholder="Masukkan"]') ?? [])];
  const categoryModal = () => all('.eds-modal__box').find((m) => m.querySelector('.category-selector'));
  const editor = () => field('description')?.querySelector('.ql-editor');
  // Foto Produk's photos, in order: the boxes of its image manager that hold
  // a photo (not the cover's manager, not the box that adds one).
  const galleryBoxes = () => {
    const f = field('images');
    const m = [...(f?.querySelectorAll('.shopee-image-manager') ?? [])].find((x) => !x.closest('[data-product-edit-field-unique-id="promotionImages"]'));
    return [...(m?.querySelectorAll('.shopee-image-manager__itembox') ?? [])].filter((b) => b.querySelector('img.shopee-image-manager__image'));
  };

  return {
    // state is what the page shows, for the waits.
    state() {
      const p = location.pathname;
      return {
        url: location.href,
        lang: (document.cookie.match(/(?:^|;\s*)language=([^;]+)/) || [])[1] || '',
        signin: /^\/account\/(signin|login)/.test(p) || /accounts\.shopee/.test(location.host),
        portal: /^\/portal\//.test(p) || p === '/',
        list: /^\/portal\/product\/list/.test(p),
        listReady: /^\/portal\/product\/list/.test(p) && !!document.querySelector('input[placeholder^="Cari Nama Produk"]'),
        // edit: a product's own page, /portal/product/<id>.
        edit: /^\/portal\/product\/\d+/.test(p),
        form: !!field('name') && !!field('images'),
        full: !!field('category'),
        sections: !!field('brandAndAttributes') && !!field('logistic'),
        modal: text(modalBox()).slice(0, 300),
      };
    },

    // ---- reads, from Seller Centre's own requests ----
    async shop() {
      const s = await api('/api/selleraccount/shop_info/');
      const u = await api('/api/selleraccount/user_info/');
      return {
        code: s.code ?? -1,
        message: s.message ?? '',
        shop: s.data?.name ?? '',
        shopId: s.data?.shop_id ?? 0,
        user: u.data?.user_name ?? '',
        language: u.data?.language ?? '',
      };
    },
    // products is a page of Produk Saya: one tab (list type), or the search's
    // results for keyword.
    async products(listType, page, size, keyword) {
      const j = keyword
        ? await api('/api/v3/opt/mpsku/list/v2/search_product_list', { page_size: String(size), list_type: listType, keyword, request_attribute: '', operation_sort_by: 'recommend_v4' })
        : await api('/api/v3/opt/mpsku/list/v2/get_product_list', { page_number: String(page), page_size: String(size), list_type: listType });
      return { code: j.code ?? -1, message: j.message ?? '', total: j.data?.page_info?.total ?? 0, products: (j.data?.products ?? []).map(product) };
    },
    async bumped() {
      const j = await api('/api/v3/opt/mpsku/list/get_bumped_product_list');
      return {
        code: j.code ?? -1,
        message: j.message ?? '',
        total: j.data?.config?.total_slots ?? 0,
        cooldown: j.data?.config?.cooldown_seconds ?? 0,
        products: (j.data?.products ?? []).map((p) => ({ id: p.id, left: p.cool_down_seconds ?? 0 })),
      };
    },
    // detail is a product as its own page reads it (get_product_info): its
    // photos' ids in order and its video (video_list: vid, duration in
    // seconds, thumb_url), which the list does not give.
    async detail(id) {
      const j = await api('/api/v3/product/get_product_info', { product_id: String(id), is_draft: 'false' });
      const p = j.data?.product_info;
      return {
        code: j.code ?? -1,
        message: j.message ?? '',
        found: !!p,
        name: p?.name ?? '',
        status: p?.status ?? 0,
        unlist: !!p?.is_unlisted,
        images: p?.images ?? [],
        videos: (p?.video_list ?? []).map((v) => ({ id: v.vid ?? '', duration: v.duration ?? 0, cover: v.thumb_url ?? '' })),
      };
    },

    // ---- Produk Saya ----
    pressAddProduct: () => press(all('button, a').find((b) => own(b) === 'Tambah Produk Baru')),
    // search types keyword into the list's search box and presses Terapkan.
    search(keyword) {
      const inp = document.querySelector('input[placeholder^="Cari Nama Produk"]');
      if (!inp) return false;
      setValue(inp, keyword);
      return press(button('Terapkan'));
    },
    searchText: () => document.querySelector('input[placeholder^="Cari Nama Produk"]')?.value ?? null,
    // row is the list's row of the product id: its text, and the action
    // buttons beside it; null when the list does not show it.
    row(id) {
      const r = rowOf(id);
      if (!r) return null;
      return { text: text(r.row).slice(0, 400), actions: r.actions ? all('button', r.actions).map(own) : [] };
    },
    rowIds: () => productRows().map((r) => r.querySelector('input.eds-checkbox__input').name),
    pressMore(id) {
      const r = rowOf(id);
      return !!r?.actions && press([...r.actions.querySelectorAll('button')].find((b) => own(b) === 'Lainnya'));
    },
    // menu is the open Lainnya menu: each item's label, whether it is
    // disabled, and its tip (a bumped product's \"5 slot naikkan produk …\").
    menu() {
      const m = menuEl();
      if (!m) return null;
      return [...m.querySelectorAll('li')].map((l) => ({
        label: text(l.querySelector('.eds-popover__ref') ?? l),
        disabled: /disabled/.test(l.className) || l.getAttribute('aria-disabled') === 'true',
        tip: text(l.querySelector('.eds-popover__content')),
      }));
    },
    pressMenu(label) {
      const m = menuEl();
      return !!m && press([...m.querySelectorAll('li')].find((l) => same(text(l.querySelector('.eds-popover__ref') ?? l), label)));
    },
    closeMenu() {
      document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      document.body.click();
      return true;
    },
    // modal is the dialog in front: its title, text and buttons.
    modal() {
      const m = modalBox();
      if (!m) return null;
      return { title: text(m.querySelector('.eds-modal__title')), text: text(m).slice(0, 600), buttons: all('button', m).map(own) };
    },
    // pressModal presses button in the dialog whose text holds what.
    pressModal(what, label) {
      const m = all('.eds-modal__box').find((x) => text(x).includes(what));
      return !!m && press(button(label, m));
    },
    toasts: () => all('.eds-toast, .eds-message, [class*=toast]').map(text).filter((t) => t && !/^\d+$/.test(t)),

    // ---- Tambah Produk Baru ----
    // guides closes the onboarding a form shows over itself: the Standar
    // Produk Shopee modal (Lanjut) and popovers that only say Mengerti. It
    // returns what it closed.
    guides() {
      const closed = [];
      for (const m of all('.eds-modal__box')) {
        const b = button('Lanjut', m);
        if (b && /Standar Produk/i.test(text(m)) && !m.querySelector('input')) {
          closed.push(text(m.querySelector('.eds-modal__title') ?? m).slice(0, 80));
          press(b);
        }
      }
      for (const c of all('.eds-popover__content')) {
        const bs = all('button', c);
        if (bs.length === 1 && own(bs[0]) === 'Mengerti') {
          closed.push(text(c).slice(0, 80));
          press(bs[0]);
        }
      }
      return closed;
    },
    // A file to upload comes in base64 pieces; each becomes a Blob at once.
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // give hands the field's file input (images, promotionImages, video) the
    // staged files, as if they were chosen in its dialog. It returns "" or
    // what went wrong.
    give(id, files) {
      const f = field(id);
      const root = f?.closest('.edit-row') ?? f;
      const input = root?.querySelector('input[type=file].eds-upload__input:not(.shopee-image-manager__replace-input)');
      if (!input) return `no file input for ${id}`;
      const list = new DataTransfer();
      for (const x of files) {
        const parts = staged.get(x.id);
        if (!parts) return `nothing staged for ${x.name}`;
        staged.delete(x.id);
        list.items.add(new File(parts, x.name, { type: x.type, lastModified: Date.now() }));
      }
      input.files = list.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // photos: how many photos the form holds ("(2/9)"), and the Pembuat Foto
    // AI drawer's apply button when it is open ("Terapkan ke Produk (2)").
    photos() {
      const f = field('images');
      const m = text(f).match(/\((\d+)\/(\d+)\)/);
      const apply = all('.drawer-creator-editor button, .drawer button').find((b) => /^Terapkan ke Produk/.test(own(b)));
      // The button reads "Terapkan ke Produk" and is greyed until the
      // drawer has taken the photos in, then "Terapkan ke Produk (2)".
      const ready = apply && !apply.disabled && !String(apply.className).includes('disabled');
      return {
        count: m ? Number(m[1]) : -1, max: m ? Number(m[2]) : 0, drawer: apply ? own(apply) : '', ready: !!ready,
        errors: errorsIn(f?.closest('.edit-row') ?? f ?? document),
      };
    },
    pressApplyPhotos: () => press(all('.drawer-creator-editor button, .drawer button').find((b) => /^Terapkan ke Produk/.test(own(b)))),
    // gallery: the photos of Foto Produk in order, by their links (the cover
    // has its own image manager, promotionImages, which is left out).
    gallery: () => galleryBoxes().map((b) => b.querySelector('img.shopee-image-manager__image').src),
    // pressDeletePhoto presses Delete in photo i's own menu (Ubah dengan AI,
    // Ubah, Preview, Delete), which takes it away at once.
    pressDeletePhoto(i) {
      const box = galleryBoxes()[i];
      return press([...(box?.querySelectorAll('.action-popup-content__actions__item') ?? [])].find((a) => /^(Delete|Hapus)$/.test(own(a))));
    },

    // Video Produk: empty (its file input, "Tambah Video"), uploading (a
    // progress bar, "0%" to "101%"), processing ("Diproses", once uploaded),
    // or ready (the video's cover, <vid>_cover, and its length, "00:15").
    // The cover's link holds the video's id from processing on. An
    // error-container that holds neither the progress nor "Diproses" is
    // Seller Centre's complaint.
    video() {
      const f = field('video');
      if (!f) return null;
      const main = f.querySelector('.video-main') ?? f;
      const cover = [...main.querySelectorAll('img')].map((i) => i.src).find((s) => /_cover/.test(s)) ?? '';
      const ready = !!main.querySelector('.video-container');
      const uploading = !ready && !!main.querySelector('.upload-process-percent, .eds-progress');
      const processing = !ready && !uploading && !!main.querySelector('.transcoding-text');
      const empty = !ready && !uploading && !processing && !!main.querySelector('input[type=file]');
      const complaints = all('.error-container', main).filter((e) => !e.querySelector('.transcoding-text, .upload-process-percent, .eds-progress')).map(own);
      return {
        state: ready ? 'ready' : uploading ? 'uploading' : processing ? 'processing' : empty ? 'empty' : 'other',
        id: (cover.match(/\/([^/?#]+)_cover/) || [])[1] ?? '',
        duration: own(main.querySelector('.video-duration')),
        progress: own(main.querySelector('.upload-process-percent')),
        text: text(main).slice(0, 120),
        errors: [...errorsIn(f), ...complaints].filter(Boolean),
      };
    },
    // pressRemoveVideo presses the video's delete icon (the last of its
    // tools: Preview, then the bin), or while it is processing, its delete.
    // Seller Centre takes it away at once, without asking.
    pressRemoveVideo() {
      const main = field('video')?.querySelector('.video-main');
      const icons = [...(main?.querySelectorAll('.action-tools .action-tools-icon') ?? [])];
      return press(icons.length ? icons[icons.length - 1] : main?.querySelector('.action-delete'));
    },
    // cover: the cover's uploaded photo, "" when there is none yet.
    cover() {
      const f = field('promotionImages');
      const img = [...(f?.querySelectorAll('img') ?? [])].find((i) => /susercontent\.com/.test(i.src));
      return img ? img.src : '';
    },
    setName: (v) => setValue(field('name')?.querySelector('input'), v),
    name: () => ({ value: field('name')?.querySelector('input')?.value ?? null, errors: errorsIn(field('name') ?? document) }),
    pressNext: () => press(button('Selanjutnya')),

    // The category: its box, and the picker (Ubah Kategori) with a column of
    // names per level.
    category: () => (field('category') ? own(field('category').querySelector('.product-category-text') ?? field('category')) : null),
    pressCategory: () => press(field('category')?.querySelector('.product-category-box-inner')),
    picker() {
      const m = categoryModal();
      if (!m) return null;
      return {
        columns: [...m.querySelectorAll('ul.scroll-item')].map((u) => [...u.querySelectorAll('li.category-item')].map((l) => own(l.querySelector('p') ?? l))),
        selected: [...m.querySelectorAll('ul.scroll-item')].map((u) => own(u.querySelector('li.category-item.selected p'))).filter(Boolean),
        picked: text([...m.querySelectorAll('*')].find((e) => e.children.length < 4 && /^Dipilih\s*:/.test(own(e)))).replace(/\s*(Cancel|Confirm|Batal|Konfirmasi).*$/, ''),
      };
    },
    pickCategory(level, name) {
      const m = categoryModal();
      const ul = m?.querySelectorAll('ul.scroll-item')[level];
      return press([...(ul?.querySelectorAll('li.category-item') ?? [])].find((l) => same(own(l.querySelector('p') ?? l), name)));
    },
    confirmCategory() {
      const m = categoryModal();
      return !!m && press(button('Confirm', m) ?? button('Konfirmasi', m));
    },

    // attributes: Spesifikasi's attributes as they read: name, whether it
    // is required, a list or a text box, its value ("" when unset).
    attributes() {
      return attrItems().map((i) => {
        const inner = i.querySelector('.eds-selector__inner');
        const inp = i.querySelector('input.eds-input__input');
        const multi = !!i.querySelector('.multiple-selector, .eds-selector--multiple, .eds-tag');
        return {
          name: own(i.querySelector('.item-title-text')),
          required: !!i.querySelector('.mandatory-icon'),
          kind: inner ? (multi ? 'multi' : 'list') : inp ? 'text' : 'other',
          value: inner ? (inner.classList.contains('placeholder') ? '' : own(inner)) : inp ? inp.value : '',
        };
      });
    },
    openAttribute(name) {
      const i = attrItem(name);
      return !!i && press(i.querySelector('.eds-selector'));
    },
    // options are the open list's options, and whether it has a filter box.
    options() {
      const p = selectPopper();
      if (!p) return null;
      return { filter: !!p.querySelector('.eds-select__filter input'), items: [...p.querySelectorAll('.eds-option')].map(own).filter(Boolean) };
    },
    filterOptions(v) {
      const p = selectPopper();
      const inp = p?.querySelector('.eds-select__filter input');
      if (!inp) return false;
      inp.focus();
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(inp, v);
      inp.dispatchEvent(new Event('input', { bubbles: true }));
      return true;
    },
    pickOption(v) {
      const p = selectPopper();
      return !!p && press([...p.querySelectorAll('.eds-option')].find((o) => same(own(o), v)));
    },
    closeOptions() {
      const p = selectPopper();
      if (!p) return true;
      document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      document.body.click();
      return true;
    },
    setAttributeText: (name, v) => setValue(attrItem(name)?.querySelector('input.eds-input__input'), v),

    // The description (Quill): emptied, then each line typed, with a
    // paragraph between lines, as a person types it. Quill moves its
    // selection a moment after each edit, so the caret is put at the end
    // again, after a pause, before the next line.
    async setDescription(t) {
      const ed = editor();
      if (!ed) return false;
      ed.scrollIntoView({ block: 'center' });
      ed.focus();
      const select = (collapse) => {
        const s = getSelection();
        const r = document.createRange();
        r.selectNodeContents(ed);
        if (collapse) r.collapse(false);
        s.removeAllRanges();
        s.addRange(r);
      };
      select(false);
      document.execCommand('delete');
      const lines = t.split('\n');
      for (let i = 0; i < lines.length; i++) {
        await sleep(80);
        ed.focus();
        select(true);
        if (i > 0) {
          document.execCommand('insertParagraph');
          await sleep(80);
          select(true);
        }
        if (lines[i]) document.execCommand('insertText', false, lines[i]);
      }
      await sleep(80);
      ed.blur();
      return true;
    },
    description() {
      const ed = editor();
      if (!ed) return null;
      const counter = own(field('description').querySelector('.plain-text-counter'));
      return { text: [...ed.querySelectorAll(':scope > p, :scope > div, :scope > li')].map((p) => p.textContent).join('\n'), counter };
    },

    // ---- Variasi ----
    pressEnableVariation: () => press(all('button', field('variation') ?? document).find((b) => /Aktifkan Variasi/.test(own(b)))),
    pressAddTier: () => press(all('button', field('variation') ?? document).find((b) => /Tambah Variasi/.test(own(b)))),
    // tiers: each variation's name, options and errors as they read.
    tiers() {
      const out = [];
      for (let i = 0; tier(i); i++) {
        out.push({
          name: tier(i).querySelector('.variation-name-panel input')?.value ?? '',
          options: realOptions(i).map((x) => x.value),
          errors: errorsIn(tier(i)),
        });
      }
      return out;
    },
    setTierName: (i, v) => setValue(tier(i)?.querySelector('.variation-name-panel input'), v),
    // Seller Centre's "Isi Otomatis Variasi" may fill variations by itself
    // (from the photos and the name); these take them away again.
    pressDeleteTier: (i) => press(tier(i)?.querySelector('.variation-delete-btn')),
    pressRemoveOption: (i, j) => press([...(tier(i)?.querySelectorAll('.variation-option-panel .options-item:not(.virtual-options-item) .options-remove-btn') ?? [])][j]),
    // setOption types option j of variation i: into its box, or for the
    // next one, into the empty one after the last.
    setOption(i, j, v) {
      const real = realOptions(i);
      const inp = j < real.length ? real[j] : tier(i)?.querySelector('.variation-option-panel .virtual-options-item input[placeholder="Masukkan"]');
      return setValue(inp, v);
    },
    // models: the table's rows, in order: each row's options (the left
    // columns), price, stock and SKU.
    models() {
      const prices = fields('priceModel_').map((f) => f.querySelector('input'));
      const stocks = fields('stockModel_').map((f) => f.querySelector('input'));
      const skus = fields('skuModel_').map((f) => f.querySelector('textarea, input'));
      const t = document.querySelector('.variation-model-table');
      const labels = [];
      if (t) {
        // The left columns: a group per option of the first variation, its
        // cell (first-variation-cell) beside a cell per option of the
        // second. A variation's photo upload sits in the first cell; the
        // rows of inputs (price, stock...) are fields.
        for (const g of t.querySelectorAll('.table-cell-wrapper')) {
          const first = g.querySelector('.first-variation-cell, .table-cell.first');
          if (!first || g.querySelector('[data-product-edit-field-unique-id], input:not([type=file]), textarea')) continue;
          const name = own(first);
          const subs = [...g.querySelectorAll('.table-cell')]
            .filter((c) => c !== first && !c.contains(first) && !c.classList.contains('table-header') && !c.querySelector('.table-cell'))
            .map(own).filter(Boolean);
          if (subs.length) subs.forEach((s) => labels.push([name, s]));
          else labels.push([name]);
        }
      }
      return prices.map((p, i) => ({
        options: labels[i] ?? [],
        price: p?.value ?? '',
        stock: stocks[i]?.value ?? '',
        sku: skus[i]?.value ?? '',
        errors: [...errorsIn(fields('priceModel_')[i] ?? document.body), ...errorsIn(fields('stockModel_')[i] ?? document.body)],
      }));
    },
    setModel(i, price, stock, sku) {
      const p = fields('priceModel_')[i]?.querySelector('input');
      const s = fields('stockModel_')[i]?.querySelector('input');
      const k = fields('skuModel_')[i]?.querySelector('textarea, input');
      if (!p || !s) return false;
      setValue(p, price);
      setValue(s, stock);
      if (k && sku !== null) setValue(k, sku);
      return true;
    },

    // setField types v into a plain field: price, stock, weight,
    // dimension.length, parentSku.
    setField: (id, v) => setValue(field(id)?.querySelector('input, textarea'), v),
    fieldValue: (id) => field(id)?.querySelector('input, textarea')?.value ?? null,
    // danger: the checked Produk Berbahaya value ("0" Tidak, "1"
    // Mengandung …), "" when none is.
    danger: () => [...(field('dangersGoods')?.querySelectorAll('input[type=radio]') ?? [])].find((r) => r.checked)?.value ?? '',
    setDanger(v) {
      const r = [...(field('dangersGoods')?.querySelectorAll('input[type=radio]') ?? [])].find((x) => x.value === String(v));
      return !!r && press(r.closest('label') ?? r);
    },
    // couriers: Jasa Kirim, each with whether it is on and its price; and the
    // page's note while none is on.
    couriers() {
      const l = field('logistic');
      if (!l) return null;
      return {
        items: [...l.querySelectorAll('.logistics-item')].map((i) => ({
          name: own(i.querySelector('.logistics-item-name')),
          on: /eds-switch--open/.test(i.querySelector('.eds-switch')?.className ?? ''),
          disabled: /eds-switch--disabled/.test(i.querySelector('.eds-switch')?.className ?? ''),
          price: own(i.querySelector('.shopee-price')),
        })),
        note: own(l.querySelector('.logistics-global-error-tips')),
      };
    },
    toggleCourier(name) {
      const i = [...(field('logistic')?.querySelectorAll('.logistics-item') ?? [])].find((x) => same(own(x.querySelector('.logistics-item-name')), name));
      return press(i?.querySelector('.eds-switch'));
    },
    // ads: whether "Iklankan Produk dan Dapatkan Voucher" is on. The tool
    // never turns it on.
    ads: () => /eds-switch--open/.test(field('trafficAcceleration')?.querySelector('.eds-switch')?.className ?? ''),
    condition: () => own(field('condition')?.querySelector('.eds-selector__inner') ?? field('condition')),
    openCondition: () => press(field('condition')?.querySelector('.eds-selector')),
    preOrder: () => [...(field('preOrder')?.querySelectorAll('input[type=radio]') ?? [])].filter((r) => r.checked).map((r) => own(r.closest('label')))[0] ?? '',
    // errors: every error the form shows, with its field's label.
    errors() {
      return all('.product-edit-form-item-error, .eds-form-item__error, .logistics-global-error-tips').map((e) => {
        const row = e.closest('.edit-row, .attribute-select-item, .variation-edit-item');
        const label = own(row?.querySelector('.edit-label, .item-title-text, .variation-edit-label'));
        return (label ? label.replace(/\s+/g, ' ').replace(/^\*\s*/, '') + ': ' : '') + own(e);
      }).filter(Boolean);
    },
    saveButtons: () => all('button').map(own).filter((t) => /^Simpan & /.test(t)),
    pressSave: (label) => press(all('button').find((b) => own(b) === label && !b.disabled)),
    // A product's own page has its buttons at the bottom: Kembali, then for
    // an archived product Hapus, Tampilkan and Update.
    editButtons: () => all('.product-detail-button-container button').map((b) => ({ label: own(b), disabled: b.disabled || /disabled/.test(b.className) })),
    pressEditButton: (label) => press(all('.product-detail-button-container button').find((b) => own(b) === label && !b.disabled)),
  };
})();
