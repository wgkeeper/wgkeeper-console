try {
  var stored = localStorage.getItem('wg-theme');
  var isDark =
    stored === 'dark' ||
    (stored !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.classList.toggle('dark', isDark);
  document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';
  // Match the browser chrome to the sidebar ground, before first paint.
  document
    .querySelector('meta[name="theme-color"]')
    .setAttribute('content', isDark ? '#0a0a0a' : '#f7f7f7');
} catch {}
