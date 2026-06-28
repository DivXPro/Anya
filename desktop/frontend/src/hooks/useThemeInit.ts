import { useEffect } from 'react';
import { App } from '../../bindings/desktop';
import i18n from '@/i18n';

export type Theme = 'light' | 'dark' | 'system';

export function applyTheme(theme: Theme) {
  const root = document.documentElement;
  if (theme === 'system') {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    root.classList.toggle('dark', prefersDark);
  } else {
    root.classList.toggle('dark', theme === 'dark');
  }
}

export function useThemeInit() {
  useEffect(() => {
    App.GetSettings()
      .then((v) => {
        const s: Record<string, string> = {};
        for (const [k, val] of Object.entries(v || {})) {
          s[k] = val ?? '';
        }
        const theme = (s.theme as Theme) || 'system';
        applyTheme(theme);
        const lang = s.ui_language || i18n.language || 'zh';
        if (i18n.language !== lang) {
          i18n.changeLanguage(lang);
        }
      })
      .catch(() => {});
  }, []);
}
