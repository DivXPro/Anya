import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { DiscoveredDevice } from '../../bindings/desktop/internal/discovery/models';
import { Button } from '@/components/ui/button';
import { Refresh, AntennaSignal } from 'iconoir-react';
import DeviceAuth from './DeviceAuth';

const SCAN_INTERVAL_MS = 5000;

function DeviceTab() {
  const { t } = useTranslation();
  const [devices, setDevices] = useState<DiscoveredDevice[]>([]);
  const [scanning, setScanning] = useState(false);

  const scan = useCallback(async () => {
    setScanning((current) => {
      if (current) return current;
      (async () => {
        try {
          const result = await App.ScanDevices();
          setDevices(result || []);
        } catch (e) {
          console.error(e);
        }
        setScanning(false);
      })();
      return true;
    });
  }, []);

  useEffect(() => {
    scan();
    const timer = window.setInterval(scan, SCAN_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [scan]);

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
        <h1 className="text-2xl font-semibold">{t('tabs.device')}</h1>
        <p className="text-sm text-muted-foreground">{t('device.subtitle')}</p>
      </div>

      <DeviceAuth />

      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold">{t('device.nearbyDevices')}</h2>
          <Refresh
            className={`h-4 w-4 text-muted-foreground ${scanning ? 'animate-spin' : ''}`}
          />
        </div>

        <div
          className="rounded-lg border bg-card"
          onClick={scan}
        >
          {devices.length === 0 && (
            <div className="h-12" />
          )}
          {devices.map((d) => (
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
              <Button
                size="sm"
                onClick={(e) => {
                  e.stopPropagation();
                  connect(d);
                }}
              >
                {t('device.connect')}
              </Button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default DeviceTab;
