try {
  var isDark = localStorage.getItem('wg-theme') === 'dark';
  document.documentElement.classList.toggle('dark', isDark);
  document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';
} catch (_) {}
