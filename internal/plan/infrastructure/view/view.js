(() => {
  const data = JSON.parse(document.getElementById('plan-data').textContent);
  const root = document.documentElement;
  const THEMES = ['light', 'dark', 'auto'];
  const THEME_KEY = 'plan-view-theme';
  const KINDS = [
    ['c', 'Criteria'], ['t', 'Tasks'], ['s-', 'Specs'], ['f-', 'Files'], ['k', 'Commits'], ['g', 'Signals'], ['pr', 'PR'],
  ];
  const kindOf = (key) => (KINDS.find(([p]) => key.startsWith(p)) || [''])[0];
  const store = {
    get() { try { return localStorage.getItem(THEME_KEY) || 'auto'; } catch { return 'auto'; } },
    set(v) { try { localStorage.setItem(THEME_KEY, v); } catch { return; } },
  };
  const cls = (word) => word.replace(' ', '-');
  const dark = () => root.dataset.theme === 'dark' || (root.dataset.theme !== 'light' && matchMedia('(prefers-color-scheme: dark)').matches);

  let renderRun = 0;
  const renderDiagrams = async () => {
    if (!window.mermaid) return;
    const run = ++renderRun;
    window.mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: dark() ? 'dark' : 'default' });
    for (const node of document.querySelectorAll('.mm[data-diagram]')) {
      const idx = Number(node.dataset.diagram);
      try {
        const { svg } = await window.mermaid.render(`mm-${run}-${idx}`, data.diagrams[idx]);
        if (run !== renderRun) return;
        node.innerHTML = svg;
      } catch (error) {
        node.textContent = `The diagram did not render: ${error.message}`;
      }
    }
  };

  const toggle = document.querySelector('[data-theme-toggle]');
  const applyTheme = (t) => {
    if (t === 'auto') delete root.dataset.theme; else root.dataset.theme = t;
    toggle.querySelector('span').textContent = t;
  };
  applyTheme(store.get());
  toggle.addEventListener('click', () => {
    const next = THEMES[(THEMES.indexOf(store.get()) + 1) % THEMES.length];
    store.set(next);
    applyTheme(next);
    renderDiagrams();
  });
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', renderDiagrams);

  const bindCopy = (btn) => btn.addEventListener('click', async () => {
    const label = !btn.children.length;
    try { await navigator.clipboard.writeText(btn.dataset.copy); if (label) btn.textContent = 'Copied'; btn.classList.add('done', 'copied'); if (!label) toast('Copied'); } catch { if (label) btn.textContent = 'Select'; }
    setTimeout(() => { if (label) btn.textContent = 'Copy'; btn.classList.remove('done', 'copied'); }, 1400);
  });
  document.querySelectorAll('[data-copy]').forEach(bindCopy);

  const openLine = () => {
    if (location.hash.startsWith('#L')) {
      document.getElementById('source-details').open = true;
      document.getElementById(location.hash.slice(1))?.scrollIntoView({ block: 'center' });
    }
  };
  addEventListener('hashchange', openLine);
  openLine();

  /* ---- peek, palette, reply and keys: the plan page only ---- */
  const $ = (s) => document.querySelector(s);
  const KIND_NAME = { c: 'Criterion', t: 'Task', 's-': 'Spec', 'f-': 'File', k: 'Commit', g: 'Signal', pr: 'PR' };
  const ORDER = ['c', 't', 's-', 'k', 'f-', 'g', 'pr'];
  const APPROVAL = 'Plan approved. Go ahead with the build.';
  const PHASE = data.phase || '';
  const taskStates = data.tasks || {};
  const reduced = () => matchMedia('(prefers-reduced-motion: reduce)').matches;
  const esc = (s) => CSS.escape(s);
  const mk = (tag, className, text) => {
    const n = document.createElement(tag);
    if (className) n.className = className;
    if (text !== undefined) n.textContent = text;
    return n;
  };

  let index = null;
  const idOf = (key) => (['c', 't', 'k'].includes(kindOf(key)) ? key.toUpperCase() : '');
  const titleOf = (key) => {
    const node = document.getElementById(key);
    const text = (sel) => node?.querySelector(sel)?.textContent.trim();
    const kind = kindOf(key);
    if (kind === 'c') return text('.txt p') || data.labels[key];
    if (kind === 't' || kind === 's-') return text('h3') || data.labels[key];
    return data.labels[key] || key;
  };
  const items = () => {
    if (index) return index;
    index = {};
    Object.keys(data.labels || {})
      .sort((a, b) => ORDER.indexOf(kindOf(a)) - ORDER.indexOf(kindOf(b)))
      .forEach((key) => { index[key] = { key, kind: KIND_NAME[kindOf(key)] || 'Item', id: idOf(key), title: titleOf(key) }; });
    return index;
  };
  const stateOf = (key) => (kindOf(key) === 't' ? taskStates[key] : data.state[key]) || '';
  const draftBox = () => $('#reply');
  const draft = () => (draftBox()?.value || '').trim();

  let peekKey = null;
  const linkRow = (key) => {
    const o = items()[key];
    const a = mk('a', 'chip', `${o.id ? o.id + ' ' : ''}${o.title}`);
    a.href = `#${key}`;
    a.dataset.open = key;
    a.title = o.title;
    a.style.cssText = 'max-width:100%;height:auto;min-height:20px;white-space:normal;padding:3px 6px;overflow-wrap:anywhere';
    return a;
  };
  const group = (head, ...nodes) => {
    const g = mk('div');
    g.append(mk('h4', '', head), ...nodes);
    return g;
  };
  function openPeek(key) {
    const o = items()[key];
    const peek = $('#peek');
    if (!o || !peek) return;
    peekKey = key;
    const kind = kindOf(key);
    const node = document.getElementById(key);
    $('#peek-k').textContent = `${o.kind}${o.id ? ' · ' + o.id : ''}`;
    const head = mk('div');
    head.style.cssText = 'display:grid;gap:8px';
    const state = stateOf(key);
    if (state) {
      const pill = mk('span', `st st-${cls(state)}`, state);
      pill.style.justifySelf = 'start';
      head.append(pill);
    }
    head.append(mk('h3', '', o.title));
    let extra = '';
    if (kind === 't') extra = node?.querySelector('.tf')?.textContent.trim();
    if (kind === 's-') extra = node?.querySelector('header + p')?.textContent.trim();
    if (extra) head.append(mk('p', '', extra));
    if (PHASE === 'awaiting_approval' && draftBox()) {
      const add = mk('button', 'btn', 'Add correction');
      add.type = 'button';
      add.dataset.act = 'add-correction';
      add.style.justifySelf = 'start';
      head.append(add);
    }
    const parts = [head];
    if (kind === 'c') {
      const num = key.slice(1);
      const proof = [...document.querySelectorAll('#proofs li')].find((li) => li.querySelector('.cn-mini')?.textContent.replace(/\D/g, '') === num)
        ?.querySelector('code')?.textContent;
      if (proof) {
        const copy = mk('button', 'copy', 'Copy');
        copy.type = 'button';
        copy.dataset.copy = proof;
        bindCopy(copy);
        parts.push(group('Proof', mk('code', '', proof), document.createTextNode(' '), copy));
      }
    }
    if (kind === 't') {
      const after = [...(node?.querySelectorAll('.lk') || [])].find((lk) => lk.querySelector('em')?.textContent.trim().toLowerCase() === 'after');
      const keys = [...(after?.querySelectorAll('a[data-key]') || [])].map((a) => a.dataset.key).filter((k) => items()[k]);
      if (keys.length) parts.push(group('Starts after', ...keys.map(linkRow)));
    }
    const near = [...new Set(data.graph[key] || [])].filter((k) => items()[k]);
    for (const p of ORDER) {
      const keys = near.filter((k) => kindOf(k) === p);
      if (!keys.length) continue;
      const more = keys.length > 12 ? [mk('p', '', `and ${keys.length - 12} more`)] : [];
      parts.push(group(`${KIND_NAME[p]}s · ${keys.length}`, ...keys.slice(0, 12).map(linkRow), ...more));
    }
    $('#peek-body').replaceChildren(...parts);
    peek.classList.add('open');
    select(key);
  }
  function closePeek() {
    $('#peek')?.classList.remove('open');
    peekKey = null;
  }
  function select(key) {
    document.querySelectorAll('.sel').forEach((n) => n.classList.remove('sel'));
    if (key) document.querySelectorAll(`[data-key="${esc(key)}"]`).forEach((n) => n.classList.add('sel'));
  }

  let palIdx = 0;
  let palItems = [];
  function openPal() {
    $('#scrim').classList.add('open');
    $('#pal-in').value = '';
    filterPal();
    setTimeout(() => $('#pal-in').focus(), 0);
  }
  function closePal() { $('#scrim').classList.remove('open'); }
  function filterPal() {
    const q = $('#pal-in').value.toLowerCase().trim();
    palItems = Object.values(items())
      .filter((o) => !q || `${o.id} ${o.title} ${o.kind}`.toLowerCase().includes(q))
      .slice(0, 40).map((o) => o.key);
    palIdx = 0;
    drawPal();
  }
  function drawPal() {
    const list = $('#pal-list');
    const rows = palItems.map((key, i) => {
      const o = items()[key];
      const li = mk('li', i === palIdx ? 'on' : '');
      li.dataset.pal = key;
      li.append(mk('span', 'id', o.id), mk('span', 't', o.title), mk('span', 'k', o.kind));
      return li;
    });
    if (!rows.length) rows.push(mk('li', '', 'No match'));
    list.replaceChildren(...rows);
    list.querySelector('.on')?.scrollIntoView({ block: 'nearest' });
  }
  function jump(key) {
    closePal();
    document.getElementById(key)?.scrollIntoView({ block: 'center', behavior: reduced() ? 'auto' : 'smooth' });
    openPeek(key);
  }

  // A sticky toast stays until the caller clears it.
  function toast(msg, sticky) {
    const t = $('#toast');
    if (!t) return;
    t.textContent = msg;
    t.classList.add('show');
    clearTimeout(toast.t);
    if (!sticky) toast.t = setTimeout(() => t.classList.remove('show'), 1800);
  }
  function copyReply(text, message) {
    // On failure the reply is selected so the user can press the copy shortcut.
    const fail = () => {
      const box = document.createElement('textarea');
      box.value = text;
      box.readOnly = true;
      box.setAttribute('aria-label', 'Reply to copy');
      box.style.cssText = 'position:fixed;left:8px;bottom:8px;width:min(32rem,90vw);height:6rem;z-index:99';
      box.addEventListener('blur', () => { box.remove(); $('#toast')?.classList.remove('show'); });
      document.body.appendChild(box);
      box.focus();
      box.select();
      toast('Copy failed, press \u2318C to copy the selected reply', true);
    };
    try { navigator.clipboard.writeText(text).then(() => toast(message)).catch(fail); } catch { fail(); }
  }
  function approve() {
    if (draft() && !confirm('You have unsent corrections. Approve anyway?')) return;
    copyReply(APPROVAL, 'Approval copied, paste it to the agent');
  }
  function sendCorrections() {
    const lines = draft().split('\n').map((l) => l.trim()).filter(Boolean).map((l) => (l.startsWith('- ') ? l : `- ${l}`));
    if (!lines.length) { focusReply(); return; }
    copyReply(`Please change the plan before I approve it:\n${lines.join('\n')}`, 'Corrections copied, paste it to the agent');
  }
  function focusReply() {
    const box = draftBox();
    if (!box) return;
    box.scrollIntoView({ block: 'center', behavior: reduced() ? 'auto' : 'smooth' });
    box.focus();
    box.setSelectionRange(box.value.length, box.value.length);
  }
  function addCorrection() {
    const box = draftBox();
    const o = items()[peekKey];
    if (!box || !o) return;
    const prefix = box.value.trim() ? box.value.replace(/\s*$/, '\n') : '';
    box.value = `${prefix}${o.id || o.title}: `;
    syncReply();
    focusReply();
  }
  function syncReply() {
    const send = $('button[data-act=send-corrections]');
    if (send) send.disabled = !draft();
  }

  const initPage = () => {
    const ACTS = { approve, 'send-corrections': sendCorrections, 'focus-reply': focusReply, 'add-correction': addCorrection, 'close-peek': closePeek };
    document.addEventListener('input', (e) => { if (e.target.id === 'reply') syncReply(); });
    document.addEventListener('click', (e) => {
      const act = e.target.closest('[data-act]');
      if (act) { ACTS[act.dataset.act]?.(); return; }
      const open = e.target.closest('[data-open]');
      if (open) { e.preventDefault(); jump(open.dataset.open); return; }
      const pal = e.target.closest('[data-pal]');
      if (pal) { jump(pal.dataset.pal); return; }
      if (e.target === $('#scrim')) { closePal(); return; }
      const item = e.target.closest('main [data-key]');
      if (item && items()[item.dataset.key] && !e.target.closest('button, summary, details, pre.src, textarea, input')) openPeek(item.dataset.key);
    });
    $('#open-pal')?.addEventListener('click', openPal);
    $('#pal-in').addEventListener('input', filterPal);

    // Files: filter rows by change and collapse folders. Without this script every row stays visible and open.
    let fileChange = 'all';
    const applyFiles = () => {
      document.querySelectorAll('#trees .rows').forEach((box) => {
        const rows = [...box.querySelectorAll(':scope > .tr')];
        let shut = null;
        rows.forEach((row, i) => {
          const depth = Number(row.dataset.depth) || 0;
          if (shut !== null && depth <= shut) shut = null;
          const isDir = row.classList.contains('dir-row');
          let match = fileChange === 'all' || (!isDir && row.dataset.change === fileChange);
          if (isDir && fileChange !== 'all') {
            for (let j = i + 1; j < rows.length && (Number(rows[j].dataset.depth) || 0) > depth; j++) {
              if (rows[j].dataset.change === fileChange) { match = true; break; }
            }
          }
          row.hidden = shut !== null || !match;
          if (isDir && shut === null && row.getAttribute('aria-expanded') === 'false') shut = depth;
        });
      });
    };
    const toggleDir = (row) => {
      row.setAttribute('aria-expanded', row.getAttribute('aria-expanded') === 'false' ? 'true' : 'false');
      applyFiles();
    };
    document.addEventListener('click', (e) => {
      const btn = e.target.closest('[data-filter]');
      if (btn) {
        const bar = btn.closest('#ftool');
        if (!bar) return;
        fileChange = btn.dataset.filter;
        bar.querySelectorAll('[data-filter]').forEach((b) => {
          const on = b === btn;
          b.classList.toggle('on', on);
          b.setAttribute('aria-pressed', String(on));
        });
        applyFiles();
        return;
      }
      const dir = e.target.closest('#trees .dir-row');
      if (dir) toggleDir(dir);
    });
    document.addEventListener('keydown', (e) => {
      if (e.target.matches('input, textarea') || e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key !== 'Enter' && e.key !== ' ') return;
      const dir = e.target.closest?.('#trees .dir-row');
      if (dir) { e.preventDefault(); e.stopImmediatePropagation(); toggleDir(dir); }
    });

    const SECTIONS = { c: 'criteria', t: 'tasks', f: 'trees', q: 'problems', d: 'notes', s: 'specs' };
    const ROWS = '.criterion, .task, .spec, .tr.file, .commits li';
    let gPending = false;
    let gTimer;
    document.addEventListener('keydown', (e) => {
      const palOpen = $('#scrim').classList.contains('open');
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k' && (palOpen || !e.target.matches('textarea'))) { e.preventDefault(); if (palOpen) closePal(); else openPal(); return; }
      if (palOpen) {
        if (e.key === 'Escape') closePal();
        else if (e.key === 'ArrowDown') { e.preventDefault(); palIdx = Math.min(palIdx + 1, palItems.length - 1); drawPal(); }
        else if (e.key === 'ArrowUp') { e.preventDefault(); palIdx = Math.max(palIdx - 1, 0); drawPal(); }
        else if (e.key === 'Enter' && palItems[palIdx]) { e.preventDefault(); jump(palItems[palIdx]); }
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && PHASE === 'awaiting_approval' && draftBox()) {
        e.preventDefault();
        if (draft()) sendCorrections(); else approve();
        return;
      }
      if (e.target.matches('input, textarea')) { if (e.key === 'Escape') e.target.blur(); return; }
      if (e.key === 'Escape') { closePeek(); select(null); return; }
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key === '/') { e.preventDefault(); openPal(); return; }
      if (gPending) {
        gPending = false;
        clearTimeout(gTimer);
        const id = SECTIONS[e.key];
        if (id) document.getElementById(id)?.scrollIntoView({ behavior: reduced() ? 'auto' : 'smooth' });
        return;
      }
      if (e.key === 'g') { gPending = true; gTimer = setTimeout(() => { gPending = false; }, 800); return; }
      if (e.key === 'j' || e.key === 'k') {
        const rows = [...document.querySelectorAll(ROWS)].filter((n) => n.offsetParent);
        if (!rows.length) return;
        const cur = rows.findIndex((n) => n.classList.contains('sel'));
        const next = rows[Math.max(0, Math.min(rows.length - 1, cur + (e.key === 'j' ? 1 : -1)))];
        select(next.dataset.key);
        next.scrollIntoView({ block: 'nearest' });
        if (peekKey) openPeek(next.dataset.key);
        return;
      }
      if (e.key === 'Enter') {
        const cur = document.querySelector(ROWS.split(', ').map((s) => `${s}.sel`).join(', '));
        if (cur) openPeek(cur.dataset.key);
      }
    });

    if ('IntersectionObserver' in window) {
      const io = new IntersectionObserver((entries) => entries.forEach((en) => {
        if (!en.isIntersecting) return;
        document.querySelectorAll('#nav a').forEach((a) => a.classList.toggle('on', a.getAttribute('href') === `#${en.target.id}`));
      }), { rootMargin: '-50% 0px -45% 0px' });
      document.querySelectorAll('main > section').forEach((s) => io.observe(s));
    }
  };
  if ($('#peek') && $('#scrim') && $('#pal-in') && $('#pal-list')) initPage();

  const tracer = document.getElementById('tracer');
  if (!tracer) {
    renderDiagrams();
    return;
  }

  const neighbours = (key) => data.graph[key] || [];
  const traceOf = (key) => {
    const kind = kindOf(key);
    const lit = new Set([key, ...neighbours(key)]);
    for (const n of neighbours(key)) {
      if (kindOf(n) !== 't' || kind === 't') continue;
      for (const m of neighbours(n)) if (kindOf(m) !== kind) lit.add(m);
    }
    return lit;
  };

  const body = document.getElementById('tracer-body');
  let pinned = null;
  const clear = () => {
    document.body.classList.remove('tracing');
    document.querySelectorAll('.lit').forEach((el) => el.classList.remove('lit', 'focus'));
  };
  const chipOf = (prefix, k) => {
    const a = document.createElement('a');
    a.className = 'chip';
    a.href = `#${k}`;
    a.dataset.key = k;
    a.textContent = prefix === 'f-' ? data.labels[k].split('/').pop() : data.labels[k];
    a.title = data.labels[k];
    const state = data.state[k];
    if (state) {
      const st = document.createElement('span');
      st.className = `st st-${cls(state)}`;
      st.textContent = state;
      a.append(st);
    }
    return a;
  };
  const groupOf = ([prefix, name], keys) => {
    const grp = document.createElement('div');
    grp.className = 'grp';
    const em = document.createElement('em');
    em.textContent = `${name} ${keys.length}`;
    const list = document.createElement('div');
    list.append(...keys.map((k) => chipOf(prefix, k)));
    grp.append(em, list);
    return grp;
  };
  const show = (key) => {
    clear();
    if (!key) { tracer.hidden = true; return; }
    const lit = traceOf(key);
    document.body.classList.add('tracing');
    document.querySelectorAll('[data-key]').forEach((el) => {
      if (lit.has(el.dataset.key)) el.classList.add('lit');
      if (el.dataset.key === key && !el.classList.contains('chip')) el.classList.add('focus');
    });
    document.getElementById('tracer-title').textContent = data.labels[key] || key;
    const groups = KINDS.map((kind) => [kind, [...lit].filter((k) => k !== key && kindOf(k) === kind[0])]);
    body.replaceChildren(...groups.filter(([, keys]) => keys.length).map(([kind, keys]) => groupOf(kind, keys)));
    tracer.hidden = false;
    tracer.classList.toggle('pinned', pinned === key);
  };

  const ITEM = '.criterion, .proofs li, .task, .spec, .tr.file, .commits li, .signals tbody tr, .matrix tbody tr, .chip[data-key], .matrix thead a';
  const main = document.querySelector('main');
  let hoverTimer;
  main.addEventListener('pointerover', (e) => {
    const el = e.target.closest(ITEM);
    clearTimeout(hoverTimer);
    hoverTimer = setTimeout(() => show(el ? el.dataset.key : pinned), el ? 60 : 220);
  });
  main.addEventListener('pointerleave', () => { clearTimeout(hoverTimer); show(pinned); });
  document.addEventListener('focusin', (e) => {
    const el = e.target.closest('[data-key]');
    if (el && !tracer.contains(el)) show(el.dataset.key);
  });
  document.addEventListener('click', (e) => {
    const el = e.target.closest(ITEM);
    if (!el || e.target.closest('button, summary, pre.src')) return;
    pinned = pinned === el.dataset.key && !el.matches('a') ? null : el.dataset.key;
    show(pinned);
  });
  document.getElementById('tracer-close').addEventListener('click', () => { pinned = null; show(null); });
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') { pinned = null; show(null); } });

  renderDiagrams();
})();
