import { useState } from 'react';
import { App } from '../../bindings/desktop';
import type { DiscoveredDevice } from '../../bindings/desktop/internal/discovery/models';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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

      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>附近设备</CardTitle>
              <CardDescription>扫描并连接同一网络下的 StickC 设备</CardDescription>
            </div>
            <Button size="sm" onClick={scan} disabled={scanning} className="gap-2">
              <Refresh className={`h-4 w-4 ${scanning ? 'animate-spin' : ''}`} />
              {scanning ? '扫描中...' : '扫描'}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {devices.length === 0 && !scanning && (
            <p className="text-sm text-muted-foreground">未发现设备，请确保 StickC 已开机并在同一网络</p>
          )}
          <div className="space-y-2">
            {devices.map((d) => (
              <div
                key={d.DeviceID}
                className="flex items-center justify-between rounded-lg border bg-card p-3"
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
        </CardContent>
      </Card>

      <DeviceAuth />
    </div>
  );
}

export default DeviceTab;
