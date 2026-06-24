import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { DiscoveredDevice } from '../../bindings/desktop/internal/discovery/models';
import DeviceAuth from './DeviceAuth';

function DeviceTab() {
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [devices, setDevices] = useState<DiscoveredDevice[]>([]);
  const [scanning, setScanning] = useState(false);

  useEffect(() => {
    App.GetSettings().then(v => {
      if (v) {
        const s: Record<string, string> = {};
        for (const [k, val] of Object.entries(v)) {
          s[k] = val ?? '';
        }
        setSettings(s);
      }
    }).catch(() => {});
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

  return (
    <div className="device-tab">
      <section className="card">
        <h3>附近设备</h3>
        <button onClick={scan} disabled={scanning}>
          {scanning ? '扫描中...' : '🔍 扫描设备'}
        </button>
        {devices.length === 0 && !scanning && <p>未发现设备，请确保 StickC 已开机并在同一网络</p>}
        {devices.map(d => (
          <div key={d.DeviceID} className="device-row">
            <span>{d.Name}</span>
            <code>{d.DeviceID.slice(-8)}</code>
            <button onClick={() => connect(d)}>连接</button>
          </div>
        ))}
      </section>

      <DeviceAuth />

      <section className="card">
        <h3>语音设置</h3>
        <label>
          STT 引擎:
          <select value={settings.stt_engine || 'whisper'} onChange={e => App.SetSetting('stt_engine', e.target.value)}>
            <option value="whisper">faster-whisper</option>
          </select>
        </label>
        <label>
          语言:
          <select value={settings.stt_language || 'zh'} onChange={e => App.SetSetting('stt_language', e.target.value)}>
            <option value="zh">中文</option>
            <option value="en">English</option>
          </select>
        </label>
        <label>
          语速:
          <select value={settings.tts_speed || '+0%'} onChange={e => App.SetSetting('tts_speed', e.target.value)}>
            <option value="-20%">慢速</option>
            <option value="+0%">正常</option>
            <option value="+20%">快速</option>
          </select>
        </label>
        <label className="checkbox">
          <input type="checkbox" checked={settings.tts_enabled !== 'false'}
            onChange={e => App.SetSetting('tts_enabled', e.target.checked ? 'true' : 'false')}
          />
          启用 TTS
        </label>
      </section>
    </div>
  );
}

export default DeviceTab;
