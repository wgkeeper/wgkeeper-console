import { createContext, use, useEffect, useMemo, useState } from 'react';

export type Theme = 'dark' | 'light' | 'system';

const STORAGE_KEY = 'wg-theme';

type ThemeContextValue = {
  theme: Theme;
  resolvedTheme: 'dark' | 'light';
  setTheme: (theme: Theme) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

const readStoredTheme = (): Theme => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    return stored === 'dark' || stored === 'light' ? stored : 'system';
  } catch {
    return 'system';
  }
};

const systemPrefersDark = () => window.matchMedia('(prefers-color-scheme: dark)').matches;

// The shadcn/ui Vite theme provider, keyed to the same storage entry that
// public/theme-init.js reads before first paint.
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readStoredTheme);
  const [systemDark, setSystemDark] = useState(systemPrefersDark);

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => setSystemDark(media.matches);
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, []);

  const resolvedTheme = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme;

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle('dark', resolvedTheme === 'dark');
    root.style.colorScheme = resolvedTheme;
    // The browser chrome follows the theme chosen here, not only the OS.
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute('content', resolvedTheme === 'dark' ? '#0a0a0a' : '#f7f7f7');
  }, [resolvedTheme]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      theme,
      resolvedTheme,
      setTheme: (next) => {
        try {
          if (next === 'system') localStorage.removeItem(STORAGE_KEY);
          else localStorage.setItem(STORAGE_KEY, next);
        } catch {
          // storage may be unavailable; the choice still applies to this tab
        }
        setThemeState(next);
      },
    }),
    [theme, resolvedTheme],
  );

  return <ThemeContext value={value}>{children}</ThemeContext>;
}

export function useTheme() {
  const context = use(ThemeContext);
  if (!context) throw new Error('useTheme must be used within a ThemeProvider');
  return context;
}
