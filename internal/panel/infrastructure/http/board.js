(() => {
  const root = document.documentElement;
  const THEMES = ['light', 'dark', 'auto'];
  const THEME_KEY = 'plan-view-theme';
  const store = {
    get() { try { return localStorage.getItem(THEME_KEY) || 'auto'; } catch { return 'auto'; } },
    set(v) { try { localStorage.setItem(THEME_KEY, v); } catch { return; } },
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
  });
})();
