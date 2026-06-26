import { useEffect, useState, useCallback } from 'react';
import { App } from '../../bindings/desktop';
import i18n from '@/i18n';

export type Theme = 'light' | 'dark' | 'system';

function applyTheme(theme: Theme) {
  const root = document.documentElement;
  if (theme === 'system') {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    root.classList.toggle('dark', prefersDark);
  } else {
    root.classList.toggle('dark', theme === 'dark');
  }
}

export function useAppSettings() {
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    App.GetSettings()
      .then((v) => {
        const s: Record<string, string> = {};
        for (const [k, val] of Object.entries(v || {})) {
          s[k] = val ?? '';
        }
        setSettings(s);
        setLoaded(true);

        const theme = (s.theme as Theme) || 'system';
        applyTheme(theme);

        const lang = s.ui_language || i18n.language || 'zh';
        if (i18n.language !== lang) {
          i18n.changeLanguage(lang);
        }
      })
      .catch(() => setLoaded(true));
  }, []);

  const updateSetting = useCallback(async (key: string, value: string) => {
    setSettings((prev) => ({ ...prev, [key]: value }));
    await App.SetSetting(key, value);
  }, []);

  const updateTheme = useCallback(
    async (theme: Theme) => {
      applyTheme(theme);
      await updateSetting('theme', theme);
    },
    [updateSetting]
  );

  const updateLanguage = useCallback(
    async (lang: string) => {
      await i18n.changeLanguage(lang);
      await updateSetting('ui_language', lang);
    },
    [updateSetting]
  );

  return {
    settings,
    loaded,
    theme: (settings.theme as Theme) || 'system',
    language: settings.ui_language || i18n.language || 'zh',
    updateSetting,
    updateTheme,
    updateLanguage,
  };
}
