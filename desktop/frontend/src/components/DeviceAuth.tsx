import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { AuthorizedDevice } from '../../bindings/desktop/internal/store/models';
import type { PendingDevice } from '../../bindings/desktop/internal/gateway/models';

function DeviceAuth() {
  const [devices, setDevices] = useState<AuthorizedDevice[]>([]);
  const [pending, setPending] = useState<PendingDevice[]>([]);

  useEffect(() => {
    App.ListAuthorizedDevices().then(v => setDevices(v || [])).catch(() => {});
    App.ListPendingDevices().then(v => setPending(v || [])).catch(() => {});
    const timer = window.setInterval(() => {
      App.ListPendingDevices().then(v => setPending(v || [])).catch(() => {});
    }, 2000);
    return () => window.clearInterval(timer);
  }, []);

  const authorize = async (id: string) => {
    await App.AuthorizeDevice(id);
    App.ListPendingDevices().then(v => setPending(v || [])).catch(() => {});
    App.ListAuthorizedDevices().then(v => setDevices(v || [])).catch(() => {});
  };

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
          <span>{d.name || d.device_id.slice(-8)}</span>
          <span className="time">{d.authorized_at}</span>
          <span className="time">{d.last_seen_ip ? `Last seen ${d.last_seen_ip}` : 'Never seen'}</span>
          <button onClick={() => { App.RevokeDevice(d.device_id); App.ListAuthorizedDevices().then(v => setDevices(v || [])).catch(() => {}); }}>撤销</button>
        </div>
      ))}
    </section>
  );
}

export default DeviceAuth;
