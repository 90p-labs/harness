/* Served pages only. A morph swap resets the nav item and the reply button, so both sync again after it. */
(() => {
  let current = null;
  document.addEventListener('htmx:beforeSwap', () => {
    current = document.querySelector('#nav a.on')?.getAttribute('href') || current;
  });
  document.addEventListener('htmx:afterSwap', () => {
    const send = document.querySelector('button[data-act=send-corrections]');
    if (send) send.disabled = !(document.getElementById('reply')?.value || '').trim();
    if (current) document.querySelectorAll('#nav a').forEach((a) => a.classList.toggle('on', a.getAttribute('href') === current));
  });
})();
