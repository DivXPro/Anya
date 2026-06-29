import { useEffect, useState, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { DownloadProgress } from '../../bindings/desktop/internal/speech/models';
import { FlashStage } from '../../bindings/desktop/internal/firmware/models';
import type { SerialPortInfo, FlashProgress } from '../../bindings/desktop/internal/firmware/models';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Events } from '@wailsio/runtime';
import { Dialogs } from '@wailsio/runtime';
import {
  RiTranslate,
  RiDashboardLine,
  RiMicLine,
  RiSunLine,
  RiMoonLine,
  RiUsbLine,
  RiFolderLine,
} from '@remixicon/react';
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
  const [ports, setPorts] = useState<SerialPortInfo[]>([]);
  const [selectedPort, setSelectedPort] = useState('');
  const [hasFirmware, setHasFirmware] = useState(false);
  const [firmwareVersion, setFirmwareVersion] = useState('');
  const [deviceVersion, setDeviceVersion] = useState('');
  const [checkingVersion, setCheckingVersion] = useState(false);
  const [flashProgress, setFlashProgress] = useState<FlashProgress>({
    running: false,
    stage: FlashStage.StageIdle,
    percent: 0,
    message: '',
    error: '',
  });
  const workingDirectoryRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const off = Events.On('navigate-to-working-directory', () => {
      workingDirectoryRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
      workingDirectoryRef.current?.focus();
    });
    return () => off();
  }, []);

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

    App.HasEmbeddedFirmware()
      .then((v) => setHasFirmware(v))
      .catch(() => {});

    App.CurrentFirmwareVersion()
      .then((v) => setFirmwareVersion(v))
      .catch(() => {});

    refreshPorts();

    const poll = () => {
      App.STTReady()
        .then((ready) => setSttReady(ready))
        .catch(() => {});
      App.GetSTTDownloadProgress()
        .then((p) => setProgress(p))
        .catch(() => {});
      App.GetFlashProgress()
        .then((p) => setFlashProgress(p))
        .catch(() => {});
    };
    poll();
    const timer = window.setInterval(poll, 500);
    return () => window.clearInterval(timer);
  }, []);

  const refreshPorts = () => {
    App.ListSerialPorts()
      .then((list) => {
        setPorts(list || []);
        if (selectedPort === '' && list && list.length > 0) {
          setSelectedPort(list[0].path);
        }
      })
      .catch(() => {
        setPorts([]);
      });
  };

  const startFlash = async () => {
    if (!selectedPort) return;
    if (!window.confirm(t('settings.firmware.confirmDesc') || undefined)) return;
    try {
      await App.FlashFirmware(selectedPort);
    } catch (e) {
      console.error(e);
    }
  };

  const cancelFlash = () => {
    App.CancelFlash().catch(console.error);
  };

  const checkDeviceVersion = async () => {
    if (!selectedPort) return;
    setCheckingVersion(true);
    setDeviceVersion('');
    try {
      const v = await App.ReadDeviceFirmwareVersion(selectedPort);
      setDeviceVersion(v || '');
    } catch (e) {
      console.error(e);
      setDeviceVersion('');
    } finally {
      setCheckingVersion(false);
    }
  };

  const upgradeAvailable =
    hasFirmware &&
    firmwareVersion &&
    deviceVersion &&
    firmwareVersion !== deviceVersion;

  const updateSetting = (key: string, value: string) => {
    setSettings((prev) => ({ ...prev, [key]: value }));
    App.SetSetting(key, value).catch(console.error);
  };

  const handlePickWorkingDirectory = async () => {
    try {
      const result = await Dialogs.OpenFile({
        CanChooseDirectories: true,
        CanChooseFiles: false,
        Title: t('settings.workingDirectory.dialogTitle') || undefined,
        Directory: settings.agent_cwd || undefined,
      });
      if (result) {
        const path = Array.isArray(result) ? result[0] : result;
        if (path) {
          await updateSetting('agent_cwd', path);
        }
      }
    } catch (err) {
      console.error('failed to pick working directory', err);
    }
  };

  const handleResetWorkingDirectory = async () => {
    await updateSetting('agent_cwd', '');
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
              <RiSunLine className="h-4 w-4 text-muted-foreground" />
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
              <RiMoonLine className="h-4 w-4 text-muted-foreground" />
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
              <RiTranslate className="h-4 w-4 text-muted-foreground" />
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
                <RiMicLine className="h-4 w-4 text-muted-foreground" />
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
              <RiDashboardLine className="h-4 w-4 text-muted-foreground" />
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

      <div ref={workingDirectoryRef} className="space-y-3" tabIndex={-1}>
        <h2 className="text-base font-semibold">{t('settings.workingDirectory.title')}</h2>
        <div className="rounded-lg border bg-card">
          <div className="flex items-center gap-2 border-b p-3">
            <RiFolderLine className="h-4 w-4 text-muted-foreground shrink-0" />
            <input
              type="text"
              value={settings.agent_cwd || ''}
              readOnly
              placeholder={t('settings.workingDirectory.placeholder') || undefined}
              className="flex-1 rounded-md border px-3 py-2 text-sm bg-background"
            />
            <Button onClick={handlePickWorkingDirectory}>
              {t('settings.workingDirectory.browse')}
            </Button>
            {settings.agent_cwd && (
              <Button variant="outline" onClick={handleResetWorkingDirectory}>
                {t('settings.workingDirectory.reset')}
              </Button>
            )}
          </div>
          <p className="p-3 text-xs text-muted-foreground">
            {t('settings.workingDirectory.description')}
          </p>
        </div>
      </div>

      <div className="space-y-3">
        <h2 className="text-base font-semibold">{t('settings.firmware.title')}</h2>
        <div className="rounded-lg border bg-card">
          <div className="flex items-center justify-between border-b p-3">
            <Label className="flex items-center gap-2">
              <RiUsbLine className="h-4 w-4 text-muted-foreground" />
              {t('settings.firmware.currentVersion')}
            </Label>
            <Badge variant="secondary">
              {hasFirmware ? firmwareVersion || 'unknown' : t('settings.firmware.noFirmware')}
            </Badge>
          </div>

          <div className="flex items-center justify-between border-b p-3">
            <div className="space-y-0.5">
              <Label>{t('settings.firmware.deviceVersion')}</Label>
              <p className="text-xs text-muted-foreground">
                {deviceVersion
                  ? t('settings.firmware.deviceVersionValue', { version: deviceVersion })
                  : t('settings.firmware.deviceVersionEmpty')}
                {upgradeAvailable && (
                  <span className="ml-2 text-amber-600 dark:text-amber-400">
                    {t('settings.firmware.upgradeAvailable')}
                  </span>
                )}
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={checkDeviceVersion}
              disabled={checkingVersion || !selectedPort}
            >
              {checkingVersion ? t('settings.firmware.checkingVersion') : t('settings.firmware.checkVersion')}
            </Button>
          </div>

          <div className="flex items-center justify-between border-b p-3">
            <Label>{t('settings.firmware.selectPort')}</Label>
            <div className="flex items-center gap-2">
              <Select
                value={selectedPort}
                onValueChange={setSelectedPort}
                disabled={flashProgress.running || ports.length === 0}
              >
                <SelectTrigger className="w-[180px]">
                  <SelectValue placeholder={t('settings.firmware.noPorts')} />
                </SelectTrigger>
                <SelectContent>
                  {ports.map((p) => (
                    <SelectItem key={p.path} value={p.path}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                size="sm"
                onClick={refreshPorts}
                disabled={flashProgress.running}
              >
                {t('settings.firmware.refreshPorts')}
              </Button>
            </div>
          </div>

          <div className="space-y-3 p-3">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">{t('settings.firmware.hintBoot')}</p>
              {flashProgress.running ? (
                <Button variant="secondary" size="sm" onClick={cancelFlash}>
                  {t('settings.firmware.cancel')}
                </Button>
              ) : (
                <Button
                  size="sm"
                  onClick={startFlash}
                  disabled={!hasFirmware || !selectedPort}
                >
                  {t('settings.firmware.flash')}
                </Button>
              )}
            </div>

            {flashProgress.running && (
              <div className="space-y-1.5">
                <Progress value={flashProgress.percent} />
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>{flashProgress.percent}%</span>
                  <span className="truncate max-w-[280px]">{flashProgress.message}</span>
                </div>
              </div>
            )}

            {!flashProgress.running && flashProgress.stage === FlashStage.StageDone && (
              <p className="text-sm text-emerald-700 dark:text-emerald-400">{t('settings.firmware.success')}</p>
            )}

            {!flashProgress.running && flashProgress.error && (
              <div className="space-y-1">
                <p className="text-sm text-red-700 dark:text-red-400">{t('settings.firmware.failed')}</p>
                <pre className="text-xs text-muted-foreground whitespace-pre-wrap break-all">
                  {flashProgress.error}
                </pre>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default SettingsTab;
