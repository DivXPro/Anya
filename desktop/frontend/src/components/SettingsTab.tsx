import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { DownloadProgress } from '../../bindings/desktop/internal/speech/models';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Language, DashboardSpeed, MicrophoneCheck, SunLight, HalfMoon } from 'iconoir-react';
import { useAppSettings, type Theme } from '@/hooks/useAppSettings';

function formatBytes(n: number): string {
  if (n <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(n) / Math.log(1024));
  return `${(n / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function SettingsTab() {
  const { t } = useTranslation();
  const { theme, language, updateTheme, updateLanguage } = useAppSettings();
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [sttReady, setSttReady] = useState(false);
  const [progress, setProgress] = useState<DownloadProgress>({ downloading: false, total: 0, downloaded: 0 });

  useEffect(() => {
    App.GetSettings()
      .then((v) => {
        if (v) {
          const s: Record<string, string> = {};
          for (const [k, val] of Object.entries(v)) {
            s[k] = val ?? '';
          }
          setSettings(s);
        }
      })
      .catch(() => {});

    const poll = () => {
      App.STTReady()
        .then((ready) => setSttReady(ready))
        .catch(() => {});
      App.GetSTTDownloadProgress()
        .then((p) => setProgress(p))
        .catch(() => {});
    };
    poll();
    const timer = window.setInterval(poll, 500);
    return () => window.clearInterval(timer);
  }, []);

  const updateSetting = (key: string, value: string) => {
    setSettings((prev) => ({ ...prev, [key]: value }));
    App.SetSetting(key, value).catch(console.error);
  };

  const percent = progress.total > 0 ? Math.round((progress.downloaded / progress.total) * 100) : 0;

  const modelStatus = sttReady
    ? t('settings.voice.modelReady')
    : progress.downloading
    ? t('settings.voice.modelDownloading')
    : t('settings.voice.modelLoading');

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold">{t('settings.title')}</h1>
      </div>

      <div className="space-y-3">
        <h2 className="text-base font-semibold">{t('settings.appearance.title')}</h2>
        <div className="rounded-lg border bg-card">
          <div className="flex items-center justify-between border-b p-3">
            <Label className="flex items-center gap-2">
              <SunLight className="h-4 w-4 text-muted-foreground" />
              {t('settings.appearance.theme')}
            </Label>
            <Select value={theme} onValueChange={(v) => updateTheme(v as Theme)}>
              <SelectTrigger className="w-[160px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="light">{t('settings.appearance.themeLight')}</SelectItem>
                <SelectItem value="dark">{t('settings.appearance.themeDark')}</SelectItem>
                <SelectItem value="system">{t('settings.appearance.themeSystem')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex items-center justify-between p-3">
            <Label className="flex items-center gap-2">
              <HalfMoon className="h-4 w-4 text-muted-foreground" />
              {t('settings.appearance.language')}
            </Label>
            <Select value={language} onValueChange={(v) => updateLanguage(v)}>
              <SelectTrigger className="w-[160px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="zh">{t('settings.appearance.languageZh')}</SelectItem>
                <SelectItem value="en">{t('settings.appearance.languageEn')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      <div className="space-y-3">
        <h2 className="text-base font-semibold">{t('settings.voice.title')}</h2>
        <div className="rounded-lg border bg-card">
          <div className="flex items-center justify-between border-b p-3">
            <Label className="flex items-center gap-2">
              <Language className="h-4 w-4 text-muted-foreground" />
              {t('settings.voice.language')}
            </Label>
            <Select value={settings.stt_language || 'zh'} onValueChange={(v) => updateSetting('stt_language', v)}>
              <SelectTrigger className="w-[160px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="zh">{t('settings.voice.languageZh')}</SelectItem>
                <SelectItem value="en">{t('settings.voice.languageEn')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-3 p-3">
            <div className="flex items-center justify-between">
              <Label className="flex items-center gap-2">
                <MicrophoneCheck className="h-4 w-4 text-muted-foreground" />
                {t('settings.voice.model')}
              </Label>
              <Badge
                variant="secondary"
                className={
                  sttReady
                    ? 'bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400'
                    : progress.downloading
                    ? 'bg-blue-500/10 text-blue-700 dark:bg-blue-500/15 dark:text-blue-400'
                    : ''
                }
              >
                {modelStatus}
              </Badge>
            </div>

            {progress.downloading && progress.total > 0 && (
              <div className="space-y-1.5">
                <Progress value={percent} />
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>{percent}%</span>
                  <span>
                    {formatBytes(progress.downloaded)} / {formatBytes(progress.total)}
                  </span>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="space-y-3">
        <h2 className="text-base font-semibold">{t('settings.tts.title')}</h2>
        <div className="rounded-lg border bg-card">
          <div className="flex items-center justify-between border-b p-3">
            <div className="space-y-0.5">
              <Label htmlFor="tts-enabled">{t('settings.tts.enabled')}</Label>
              <p className="text-xs text-muted-foreground">{t('settings.tts.enabledDesc')}</p>
            </div>
            <Switch
              id="tts-enabled"
              checked={settings.tts_enabled !== 'false'}
              onCheckedChange={(checked) => updateSetting('tts_enabled', checked ? 'true' : 'false')}
            />
          </div>

          <div className="flex items-center justify-between p-3">
            <Label className="flex items-center gap-2">
              <DashboardSpeed className="h-4 w-4 text-muted-foreground" />
              {t('settings.tts.speed')}
            </Label>
            <Select value={settings.tts_speed || '+0%'} onValueChange={(v) => updateSetting('tts_speed', v)}>
              <SelectTrigger className="w-[160px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="-20%">{t('settings.tts.speedSlow')}</SelectItem>
                <SelectItem value="+0%">{t('settings.tts.speedNormal')}</SelectItem>
                <SelectItem value="+20%">{t('settings.tts.speedFast')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>


    </div>
  );
}

export default SettingsTab;
