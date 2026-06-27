import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { AuthorizedDevice } from '../../bindings/desktop/internal/store/models';
import type { PendingDevice } from '../../bindings/desktop/internal/gateway/models';
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
  ShieldCheck,
  Trash,
  EditPencil,
  Wifi,
} from 'iconoir-react';

function DeviceAuth() {
  const { t } = useTranslation();
  const [devices, setDevices] = useState<AuthorizedDevice[]>([]);
  const [pending, setPending] = useState<PendingDevice[]>([]);
  const [connectedIds, setConnectedIds] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');
  const [deleting, setDeleting] = useState<AuthorizedDevice | null>(null);

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

  const activeDevices = devices.filter((d) => !d.revoked);
  const totalRows = pending.length + activeDevices.length;
  const editingDevice = activeDevices.find((d) => d.device_id === editing) || null;

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
                <ShieldCheck className="h-4 w-4 text-amber-600 dark:text-amber-400" />
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
            return (
              <div
                key={d.device_id}
                className="flex items-center justify-between border-b p-3 last:border-b-0"
              >
                <div className="min-w-0 flex-1">
                  <div className="group flex items-center gap-2">
                    <Wifi className="h-4 w-4 text-primary" />
                    <button
                      onClick={() => startEdit(d)}
                      className="flex items-center gap-1.5 text-sm font-medium hover:text-muted-foreground"
                    >
                      {displayName(d)}
                      <EditPencil className="h-3 w-3 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                    </button>
                  </div>
                  <div className="mt-1 flex items-center gap-1.5 text-xs text-muted-foreground">
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
                  </div>
                </div>
                <Button
                  size="icon"
                  variant="ghost"
                  className="h-8 w-8 text-muted-foreground hover:text-destructive"
                  onClick={() => confirmDelete(d)}
                >
                  <Trash className="h-4 w-4" />
                </Button>
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
