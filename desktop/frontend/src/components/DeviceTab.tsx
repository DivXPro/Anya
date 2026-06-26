import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { DiscoveredDevice } from '../../bindings/desktop/internal/discovery/models';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { RefreshCw, Radio } from 'lucide-react';
import DeviceAuth from './DeviceAuth';

function DeviceTab() {
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [devices, setDevices] = useState<DiscoveredDevice[]>([]);
  const [scanning, setScanning] = useState(false);

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
  }, []);

  const scan = async () => {
    setScanning(true);
    try {
      const result = await App.ScanDevices();
      setDevices(result || []);
    } catch (e) {
      console.error(e);
    }
    setScanning(false);
  };

  const connect = async (d: DiscoveredDevice) => {
    try {
      await App.ConnectToDevice(d.IP, d.Port, d.DeviceID, d.Name);
    } catch (e) {
      console.error(e);
    }
  };

  const updateSetting = (key: string, value: string) => {
    setSettings((prev) => ({ ...prev, [key]: value }));
    App.SetSetting(key, value).catch(console.error);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-white">设备</h1>
        <p className="text-sm text-white/50">管理连接的 Elf 设备和语音设置</p>
      </div>

      <Card className="border-white/10 bg-[#2e2e2e]">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-white">附近设备</CardTitle>
              <CardDescription className="text-white/50">
                扫描并连接同一网络下的 StickC 设备
              </CardDescription>
            </div>
            <Button
              size="sm"
              onClick={scan}
              disabled={scanning}
              className="gap-2 bg-white/10 text-white hover:bg-white/20"
            >
              <RefreshCw className={`h-4 w-4 ${scanning ? 'animate-spin' : ''}`} />
              {scanning ? '扫描中...' : '扫描'}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {devices.length === 0 && !scanning && (
            <p className="text-sm text-white/40">未发现设备，请确保 StickC 已开机并在同一网络</p>
          )}
          <div className="space-y-2">
            {devices.map((d) => (
              <div
                key={d.DeviceID}
                className="flex items-center justify-between rounded-lg border border-white/5 bg-white/5 p-3"
              >
                <div className="flex items-center gap-3">
                  <Radio className="h-4 w-4 text-emerald-400" />
                  <div>
                    <p className="text-sm font-medium text-white">{d.Name}</p>
                    <p className="text-xs text-white/40">{d.DeviceID.slice(-8)}</p>
                  </div>
                </div>
                <Button size="sm" onClick={() => connect(d)} className="bg-white text-black hover:bg-white/90">
                  连接
                </Button>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <DeviceAuth />

      <Card className="border-white/10 bg-[#2e2e2e]">
        <CardHeader>
          <CardTitle className="text-white">语音设置</CardTitle>
          <CardDescription className="text-white/50">配置语音识别与播报参数</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label className="text-white/70">STT 引擎</Label>
              <Select value={settings.stt_engine || 'whisper'} onValueChange={(v) => updateSetting('stt_engine', v)}>
                <SelectTrigger className="border-white/10 bg-white/5 text-white">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="border-white/10 bg-[#2e2e2e] text-white">
                  <SelectItem value="whisper">faster-whisper</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label className="text-white/70">语言</Label>
              <Select value={settings.stt_language || 'zh'} onValueChange={(v) => updateSetting('stt_language', v)}>
                <SelectTrigger className="border-white/10 bg-white/5 text-white">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="border-white/10 bg-[#2e2e2e] text-white">
                  <SelectItem value="zh">中文</SelectItem>
                  <SelectItem value="en">English</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label className="text-white/70">语速</Label>
              <Select value={settings.tts_speed || '+0%'} onValueChange={(v) => updateSetting('tts_speed', v)}>
                <SelectTrigger className="border-white/10 bg-white/5 text-white">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="border-white/10 bg-[#2e2e2e] text-white">
                  <SelectItem value="-20%">慢速</SelectItem>
                  <SelectItem value="+0%">正常</SelectItem>
                  <SelectItem value="+20%">快速</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <Separator className="bg-white/10" />
          <div className="flex items-center justify-between">
            <Label htmlFor="tts-enabled" className="text-white/80">启用 TTS</Label>
            <Switch
              id="tts-enabled"
              checked={settings.tts_enabled !== 'false'}
              onCheckedChange={(checked) => updateSetting('tts_enabled', checked ? 'true' : 'false')}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export default DeviceTab;
