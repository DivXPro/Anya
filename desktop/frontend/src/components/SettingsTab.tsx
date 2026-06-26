import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { DownloadProgress } from '../../bindings/desktop/internal/speech/models';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Microphone, Language, MessageText, DashboardSpeed, MicrophoneCheck } from 'iconoir-react';

function formatBytes(n: number): string {
  if (n <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(n) / Math.log(1024));
  return `${(n / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function SettingsTab() {
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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">设置</h1>
        <p className="text-sm text-muted-foreground">配置语音输入、播报与模型状态</p>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Microphone className="h-5 w-5 text-primary" />
            <CardTitle>语音识别</CardTitle>
          </div>
          <CardDescription>本地 Whisper 语音转文字配置</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-2">
            <Label className="flex items-center gap-2">
              <Language className="h-4 w-4 text-muted-foreground" />
              识别语言
            </Label>
            <Select value={settings.stt_language || 'zh'} onValueChange={(v) => updateSetting('stt_language', v)}>
              <SelectTrigger className="w-[240px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="zh">中文</SelectItem>
                <SelectItem value="en">English</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              更改语言后，下一次启动时生效
            </p>
          </div>

          <Separator />

          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label className="flex items-center gap-2">
                  <MicrophoneCheck className="h-4 w-4 text-muted-foreground" />
                  语音模型
                </Label>
                <p className="text-xs text-muted-foreground">
                  {sttReady
                    ? '已加载，可识别语音'
                    : progress.downloading
                    ? '正在下载模型...'
                    : '等待下载或加载模型...'}
                </p>
              </div>
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
                {sttReady ? '就绪' : progress.downloading ? '下载中' : '加载中'}
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
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <MessageText className="h-5 w-5 text-primary" />
            <CardTitle>语音播报</CardTitle>
          </div>
          <CardDescription>Edge TTS 文本转语音配置</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="tts-enabled">启用 TTS 语音播报</Label>
              <p className="text-xs text-muted-foreground">将 Agent 的文本回复转为语音并通过设备播放</p>
            </div>
            <Switch
              id="tts-enabled"
              checked={settings.tts_enabled !== 'false'}
              onCheckedChange={(checked) => updateSetting('tts_enabled', checked ? 'true' : 'false')}
            />
          </div>

          <Separator />

          <div className="space-y-2">
            <Label className="flex items-center gap-2">
              <DashboardSpeed className="h-4 w-4 text-muted-foreground" />
              播报语速
            </Label>
            <Select value={settings.tts_speed || '+0%'} onValueChange={(v) => updateSetting('tts_speed', v)}>
              <SelectTrigger className="w-[240px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="-20%">慢速</SelectItem>
                <SelectItem value="+0%">正常</SelectItem>
                <SelectItem value="+20%">快速</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export default SettingsTab;
