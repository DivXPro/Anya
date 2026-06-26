import { useState } from 'react';
import { App } from '../../bindings/desktop';
import type { DiscoveredDevice } from '../../bindings/desktop/internal/discovery/models';
import { Button } from '@/components/ui/button';
import { Refresh, AntennaSignal } from 'iconoir-react';
import DeviceAuth from './DeviceAuth';

function DeviceTab() {
  const [devices, setDevices] = useState<DiscoveredDevice[]>([]);
  const [scanning, setScanning] = useState(false);

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
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">设备</h1>
        <p className="text-sm text-muted-foreground">扫描并管理连接的 Elf 设备</p>
      </div>

      <DeviceAuth />

      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold">附近设备</h2>
          <Button size="sm" onClick={scan} disabled={scanning} className="gap-2">
            <Refresh className={`h-4 w-4 ${scanning ? 'animate-spin' : ''}`} />
            {scanning ? '扫描中...' : '扫描'}
          </Button>
        </div>

        <div className="rounded-lg border bg-card">
          {devices.length === 0 && !scanning && (
            <div className="h-12" />
          )}
          {devices.map((d, idx) => (
            <div
              key={d.DeviceID}
              className="flex items-center justify-between border-b p-3 last:border-b-0"
            >
              <div className="flex items-center gap-3">
                <AntennaSignal className="h-4 w-4 text-primary" />
                <div>
                  <p className="text-sm font-medium">{d.Name}</p>
                  <p className="text-xs text-muted-foreground">{d.DeviceID.slice(-8)}</p>
                </div>
              </div>
              <Button size="sm" onClick={() => connect(d)}>
                连接
              </Button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default DeviceTab;
