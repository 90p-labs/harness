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

  const ITEM = '.criterion, .task, .spec, .tr.file, .commits li, .signals tbody tr, .matrix tbody tr, .chip[data-key], .matrix thead a';
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

  document.querySelectorAll('[data-copy]').forEach((btn) => btn.addEventListener('click', async () => {
    try { await navigator.clipboard.writeText(btn.dataset.copy); btn.textContent = 'Copied'; btn.classList.add('done'); } catch { btn.textContent = 'Select'; }
    setTimeout(() => { btn.textContent = 'Copy'; btn.classList.remove('done'); }, 1400);
  }));

  const openLine = () => {
    if (location.hash.startsWith('#L')) {
      document.getElementById('source-details').open = true;
      document.getElementById(location.hash.slice(1))?.scrollIntoView({ block: 'center' });
    }
  };
  addEventListener('hashchange', openLine);
  openLine();

  renderDiagrams();
})();
