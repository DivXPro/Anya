import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { DiscoveredDevice } from '../../bindings/desktop/internal/discovery/models';
import type { AuthorizedDevice } from '../../bindings/desktop/internal/store/models';
import type { PendingDevice } from '../../bindings/desktop/internal/gateway/models';
import { Button } from '@/components/ui/button';
import { RiRefreshLine, RiSignalTowerLine } from '@remixicon/react';
import DeviceAuth from './DeviceAuth';

const SCAN_INTERVAL_MS = 15000;
const MATCHED_POLL_INTERVAL_MS = 2000;

function DeviceTab() {
  const { t } = useTranslation();
  const [devices, setDevices] = useState<DiscoveredDevice[]>([]);
  const [scanning, setScanning] = useState(false);
  const [matchedIds, setMatchedIds] = useState<Set<string>>(new Set());

  const refreshMatched = useCallback(async () => {
    try {
      const [authorized, pending] = await Promise.all([
        App.ListAuthorizedDevices(),
        App.ListPendingDevices(),
      ]);
      const ids = new Set<string>();
      (authorized || [])
        .filter((d: AuthorizedDevice) => !d.revoked)
        .forEach((d: AuthorizedDevice) => ids.add(d.device_id));
      (pending || []).forEach((d: PendingDevice) => ids.add(d.device_id));
      setMatchedIds(ids);
    } catch (e) {
      console.error(e);
    }
  }, []);

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
    const scanTimer = window.setInterval(scan, SCAN_INTERVAL_MS);
    return () => window.clearInterval(scanTimer);
  }, [scan]);

  useEffect(() => {
    refreshMatched();
    const matchedTimer = window.setInterval(refreshMatched, MATCHED_POLL_INTERVAL_MS);
    return () => window.clearInterval(matchedTimer);
  }, [refreshMatched]);

  const filteredDevices = useMemo(
    () => devices.filter((d) => !matchedIds.has(d.DeviceID)),
    [devices, matchedIds]
  );

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
      </div>

      <DeviceAuth />

      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold">{t('device.nearbyDevices')}</h2>
          {scanning ? (
            <RiRefreshLine className="h-4 w-4 animate-spin text-primary" />
          ) : (
            <RiRefreshLine className="h-4 w-4 text-muted-foreground" />
          )}
        </div>

        <div className="rounded-lg border bg-card" onClick={scan}>
          {filteredDevices.length === 0 && !scanning && (
            <div className="py-3 text-center text-sm text-muted-foreground">
              {t('device.noNearbyDevices')}
            </div>
          )}
          {scanning && filteredDevices.length === 0 && (
            <div className="py-3 text-center text-sm text-muted-foreground">
              {t('device.scanning')}
            </div>
          )}
          {filteredDevices.map((d) => (
            <div
              key={d.DeviceID}
              className="flex items-center justify-between border-b p-3 last:border-b-0"
            >
              <div className="flex items-center gap-3">
                <RiSignalTowerLine className="h-4 w-4 text-primary" />
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
