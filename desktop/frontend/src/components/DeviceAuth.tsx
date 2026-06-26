import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { AuthorizedDevice } from '../../bindings/desktop/internal/store/models';
import type { PendingDevice } from '../../bindings/desktop/internal/gateway/models';

function DeviceAuth() {
  const [devices, setDevices] = useState<AuthorizedDevice[]>([]);
  const [pending, setPending] = useState<PendingDevice[]>([]);
  const [editing, setEditing] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');

  const refresh = () => {
    App.ListAuthorizedDevices().then(v => setDevices(v || [])).catch(() => {});
    App.ListPendingDevices().then(v => setPending(v || [])).catch(() => {});
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
    App.ListAuthorizedDevices().then(v => setDevices(v || [])).catch(() => {});
  };

  const cancelEdit = () => {
    setEditing(null);
    setEditValue('');
  };

  const authorize = async (id: string) => {
    await App.AuthorizeDevice(id);
    App.ListPendingDevices().then(v => setPending(v || [])).catch(() => {});
    App.ListAuthorizedDevices().then(v => setDevices(v || [])).catch(() => {});
  };

  const displayName = (d: AuthorizedDevice) => d.alias || d.name || d.device_id.slice(-8);

  return (
    <section className="card">
      <h3>授权设备</h3>
      {pending.map(p => (
        <div className="pending-alert" key={p.device_id}>
          新设备请求连接: <code>{p.name || p.device_id.slice(-8)}</code>
          <button onClick={() => authorize(p.device_id)}>授权</button>
        </div>
      ))}
      {devices.filter(d => !d.revoked).map(d => (
        <div key={d.device_id} className="device-row">
          {editing === d.device_id ? (
            <>
              <input
                value={editValue}
                onChange={e => setEditValue(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') saveAlias(d.device_id);
                  if (e.key === 'Escape') cancelEdit();
                }}
                autoFocus
              />
              <button onClick={() => saveAlias(d.device_id)}>保存</button>
              <button onClick={cancelEdit}>取消</button>
            </>
          ) : (
            <>
              <span onClick={() => startEdit(d)} style={{ cursor: 'pointer' }}>
                {displayName(d)}
              </span>
              <span className="time">{d.authorized_at}</span>
              <span className="time">{d.last_seen_ip ? `Last seen ${d.last_seen_ip}` : 'Never seen'}</span>
              <button onClick={() => { App.RevokeDevice(d.device_id); App.ListAuthorizedDevices().then(v => setDevices(v || [])).catch(() => {}); }}>撤销</button>
            </>
          )}
        </div>
      ))}
    </section>
  );
}

export default DeviceAuth;
