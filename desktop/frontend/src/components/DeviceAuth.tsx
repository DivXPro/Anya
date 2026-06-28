import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { AuthorizedDevice } from '../../bindings/desktop/internal/store/models';
import type { PendingDevice } from '../../bindings/desktop/internal/gateway/models';
import type { OTAProgress } from '../../bindings/desktop/internal/firmware/models';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  RiShieldCheckLine,
  RiDeleteBinLine,
  RiEditLine,
  RiRemoteControlFill,
  RiDownloadCloudLine,
  RiCloseLine,
} from '@remixicon/react';

function DeviceAuth() {
  const { t } = useTranslation();
  const [devices, setDevices] = useState<AuthorizedDevice[]>([]);
  const [pending, setPending] = useState<PendingDevice[]>([]);
  const [connectedIds, setConnectedIds] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');
  const [deleting, setDeleting] = useState<AuthorizedDevice | null>(null);
  const [otaProgress, setOtaProgress] = useState<Record<string, OTAProgress>>({});

  const refresh = () => {
    App.ListAuthorizedDevices()
      .then((v) => setDevices(v || []))
      .catch(() => {});
    App.ListPendingDevices()
      .then((v) => setPending(v || []))
      .catch(() => {});
    App.ListConnectedDeviceIDs()
      .then((v) => setConnectedIds(new Set(v || [])))
      .catch(() => {});

    App.ListAuthorizedDevices()
      .then((devices) => {
        const active = (devices || []).filter((d: AuthorizedDevice) => !d.revoked);
        active.forEach((d: AuthorizedDevice) => {
          App.GetOTAProgress(d.device_id)
            .then((p) => {
              setOtaProgress((prev) => ({ ...prev, [d.device_id]: p }));
            })
            .catch(() => {});
        });
      })
      .catch(() => {});
  };

  useEffect(() => {
    refresh();
    const timer = window.setInterval(refresh, 2000);
    return () => window.clearInterval(timer);
  }, []);

  const startEdit = (d: AuthorizedDevice) => {
    setEditing(d.device_id);
    setEditValue(d.alias || d.name || '');
  };

  const saveAlias = async (deviceID: string) => {
    await App.SetDeviceAlias(deviceID, editValue.trim());
    setEditing(null);
    App.ListAuthorizedDevices()
      .then((v) => setDevices(v || []))
      .catch(() => {});
  };

  const cancelEdit = () => {
    setEditing(null);
    setEditValue('');
  };

  const authorize = async (id: string) => {
    await App.AuthorizeDevice(id);
    refresh();
  };

  const confirmDelete = (d: AuthorizedDevice) => {
    setDeleting(d);
  };

  const revoke = async () => {
    if (!deleting) return;
    await App.RevokeDevice(deleting.device_id);
    setDeleting(null);
    App.ListAuthorizedDevices()
      .then((v) => setDevices(v || []))
      .catch(() => {});
  };

  const cancelDelete = () => {
    setDeleting(null);
  };

  const displayName = (d: AuthorizedDevice) =>
    d.alias || d.name || d.device_id.slice(-8);

  const startOta = async (deviceID: string) => {
    try {
      await App.StartOTAUpdate(deviceID);
    } catch (e) {
      console.error(e);
    }
  };

  const cancelOta = async (deviceID: string) => {
    try {
      await App.CancelOTAUpdate(deviceID);
    } catch (e) {
      console.error(e);
    }
  };

  const activeDevices = devices.filter((d) => !d.revoked);
  const totalRows = pending.length + activeDevices.length;
  const editingDevice = activeDevices.find((d) => d.device_id === editing) || null;

  useEffect(() => {
    activeDevices.forEach((d) => {
      const p = otaProgress[d.device_id];
      if (connectedIds.has(d.device_id) && (!p || !p.device_version)) {
        App.CheckDeviceFirmwareVersion(d.device_id).catch(() => {});
      }
    });
  }, [connectedIds, activeDevices, otaProgress]);

  // Clear terminal OTA states after a few seconds so the version info reappears.
  useEffect(() => {
    const timers: number[] = [];
    Object.entries(otaProgress).forEach(([deviceID, p]) => {
      if (!p.running && ['done', 'cancelled', 'error'].includes(p.stage)) {
        timers.push(
          window.setTimeout(() => {
            setOtaProgress((prev) => {
              const next = { ...prev };
              delete next[deviceID];
              return next;
            });
          }, 3000)
        );
      }
    });
    return () => timers.forEach((t) => window.clearTimeout(t));
  }, [otaProgress]);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold">{t('device.myDevices')}</h2>
      </div>

      <ScrollArea className="min-h-[48px] max-h-[260px] rounded-lg border bg-card">
        <div>
          {totalRows === 0 && <div className="h-12" />}

          {pending.map((p) => (
            <div
              key={p.device_id}
              className="flex items-center justify-between border-b p-3 last:border-b-0"
            >
              <div className="flex items-center gap-3">
                <RiShieldCheckLine className="h-4 w-4 text-amber-600 dark:text-amber-400" />
                <span className="text-sm">
                  {t('device.pendingLabel')}
                  <span className="font-medium">
                    {p.name || p.device_id.slice(-8)}
                  </span>
                </span>
              </div>
              <Button size="sm" onClick={() => authorize(p.device_id)}>
                {t('device.authorize')}
              </Button>
            </div>
          ))}

          {activeDevices.map((d) => {
            const isConnected = connectedIds.has(d.device_id);
            const ota = otaProgress[d.device_id];
            const deviceVersion = ota?.device_version || '';
            const otaRunning = !!ota?.running;
            const otaDone = ota?.stage === 'done';
            const otaError = ota?.stage === 'error';
            const otaCancelled = ota?.stage === 'cancelled';
            const otaTerminal = otaDone || otaError || otaCancelled;

            const statusText = () => {
              if (otaRunning) return `${t('device.otaUpdating')} ${ota.percent || 0}%`;
              if (otaDone) return t('device.otaSuccess');
              if (otaError) return t('device.otaFailed');
              if (otaCancelled) return t('device.otaCancel');
              if (deviceVersion) return t('device.otaVersionCurrent', { version: deviceVersion });
              return '';
            };

            return (
              <div
                key={d.device_id}
                className="flex items-center justify-between border-b p-3 last:border-b-0"
              >
                <div className="flex min-w-0 flex-1 items-center gap-3">
                  <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-muted">
                    <RiRemoteControlFill className="h-6 w-6 text-primary" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="group flex items-center gap-1.5">
                      <button
                        onClick={() => startEdit(d)}
                        className="text-sm font-medium hover:text-muted-foreground"
                      >
                        {displayName(d)}
                      </button>
                      <RiEditLine className="h-3 w-3 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                    </div>
                    <div className="mt-0.5 flex items-center gap-1.5 text-xs text-muted-foreground">
                      {isConnected ? (
                        <>
                          <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                          <span className="text-emerald-700 dark:text-emerald-400">
                            {t('device.connected')}
                          </span>
                        </>
                      ) : (
                        <span>{t('device.disconnected')}</span>
                      )}
                      {statusText() && (
                        <span
                          className={
                            otaError
                              ? 'text-red-700 dark:text-red-400'
                              : otaDone
                              ? 'text-emerald-700 dark:text-emerald-400'
                              : otaRunning
                              ? 'text-amber-700 dark:text-amber-400'
                              : 'text-muted-foreground'
                          }
                        >
                          · {statusText()}
                        </span>
                      )}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {isConnected && (
                    <Button
                      size="sm"
                      variant={otaRunning ? 'secondary' : 'outline'}
                      disabled={otaTerminal}
                      onClick={() =>
                        otaRunning ? cancelOta(d.device_id) : startOta(d.device_id)
                      }
                    >
                      {otaRunning ? (
                        <>
                          <RiCloseLine className="mr-1 h-3.5 w-3.5" />
                          {t('device.otaCancel')}
                        </>
                      ) : (
                        <>
                          <RiDownloadCloudLine className="mr-1 h-3.5 w-3.5" />
                          {t('device.otaUpdate')}
                        </>
                      )}
                    </Button>
                  )}
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-8 w-8 text-muted-foreground hover:text-destructive"
                    onClick={() => confirmDelete(d)}
                  >
                    <RiDeleteBinLine className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      </ScrollArea>

      <Dialog open={!!editing} onOpenChange={(open) => { if (!open) cancelEdit(); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('device.renameTitle')}</DialogTitle>
            <DialogDescription>
              {editingDevice && t('device.renameDescription', { name: displayName(editingDevice) })}
            </DialogDescription>
          </DialogHeader>
          <Input
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && editing) saveAlias(editing);
              if (e.key === 'Escape') cancelEdit();
            }}
            autoFocus
            placeholder={t('device.renamePlaceholder')}
          />
          <DialogFooter>
            <Button variant="outline" onClick={cancelEdit}>
              {t('device.cancel')}
            </Button>
            <Button onClick={() => editing && saveAlias(editing)}>
              {t('device.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleting} onOpenChange={(open) => { if (!open) cancelDelete(); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('device.deleteTitle')}</DialogTitle>
            <DialogDescription>
              {deleting && t('device.deleteDescription', { name: displayName(deleting) })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={cancelDelete}>
              {t('device.cancel')}
            </Button>
            <Button variant="destructive" onClick={revoke}>
              {t('device.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default DeviceAuth;
