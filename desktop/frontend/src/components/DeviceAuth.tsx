import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { AuthorizedDevice } from '../../bindings/desktop/internal/store/models';
import type { PendingDevice } from '../../bindings/desktop/internal/gateway/models';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { ShieldCheck, ShieldX, Pencil, Check, X, Clock, Wifi } from 'lucide-react';

function DeviceAuth() {
  const [devices, setDevices] = useState<AuthorizedDevice[]>([]);
  const [pending, setPending] = useState<PendingDevice[]>([]);
  const [editing, setEditing] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');

  const refresh = () => {
    App.ListAuthorizedDevices().then((v) => setDevices(v || [])).catch(() => {});
    App.ListPendingDevices().then((v) => setPending(v || [])).catch(() => {});
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
    App.ListAuthorizedDevices().then((v) => setDevices(v || [])).catch(() => {});
  };

  const cancelEdit = () => {
    setEditing(null);
    setEditValue('');
  };

  const authorize = async (id: string) => {
    await App.AuthorizeDevice(id);
    refresh();
  };

  const revoke = async (id: string) => {
    await App.RevokeDevice(id);
    App.ListAuthorizedDevices().then((v) => setDevices(v || [])).catch(() => {});
  };

  const displayName = (d: AuthorizedDevice) => d.alias || d.name || d.device_id.slice(-8);

  const activeDevices = devices.filter((d) => !d.revoked);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>授权设备</CardTitle>
            <CardDescription>管理已配对设备和别名</CardDescription>
          </div>
          {activeDevices.length > 0 && (
            <Badge variant="secondary">
              {activeDevices.length} 台
            </Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {pending.map((p) => (
          <div
            key={p.device_id}
            className="flex items-center justify-between rounded-lg border border-amber-500/30 bg-amber-500/10 p-3"
          >
            <div className="flex items-center gap-3">
              <ShieldCheck className="h-4 w-4 text-amber-600 dark:text-amber-400" />
              <span className="text-sm">
                新设备请求连接: <span className="font-medium">{p.name || p.device_id.slice(-8)}</span>
              </span>
            </div>
            <Button size="sm" onClick={() => authorize(p.device_id)}>
              授权
            </Button>
          </div>
        ))}

        <ScrollArea className="max-h-[260px] pr-2">
          <div className="space-y-2">
            {activeDevices.length === 0 && pending.length === 0 && (
              <p className="text-sm text-muted-foreground">暂无授权设备</p>
            )}
            {activeDevices.map((d) => (
              <div
                key={d.device_id}
                className="flex items-center justify-between rounded-lg border bg-card p-3"
              >
                {editing === d.device_id ? (
                  <div className="flex flex-1 items-center gap-2">
                    <Input
                      value={editValue}
                      onChange={(e) => setEditValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') saveAlias(d.device_id);
                        if (e.key === 'Escape') cancelEdit();
                      }}
                      autoFocus
                    />
                    <Button size="icon" variant="ghost" className="h-8 w-8" onClick={() => saveAlias(d.device_id)}>
                      <Check className="h-4 w-4" />
                    </Button>
                    <Button size="icon" variant="ghost" className="h-8 w-8" onClick={cancelEdit}>
                      <X className="h-4 w-4" />
                    </Button>
                  </div>
                ) : (
                  <>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <Wifi className="h-4 w-4 text-primary" />
                        <button
                          onClick={() => startEdit(d)}
                          className="flex items-center gap-1 text-sm font-medium hover:text-muted-foreground"
                        >
                          {displayName(d)}
                          <Pencil className="h-3 w-3 text-muted-foreground" />
                        </button>
                      </div>
                      <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
                        <span className="flex items-center gap-1">
                          <Clock className="h-3 w-3" />
                          {d.authorized_at}
                        </span>
                        {d.last_seen_ip && <span>Last seen {d.last_seen_ip}</span>}
                      </div>
                    </div>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="h-8 w-8 text-muted-foreground hover:text-destructive"
                      onClick={() => revoke(d.device_id)}
                    >
                      <ShieldX className="h-4 w-4" />
                    </Button>
                  </>
                )}
              </div>
            ))}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  );
}

export default DeviceAuth;
