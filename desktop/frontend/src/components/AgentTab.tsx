import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { Agent } from '../../bindings/desktop/internal/store/models';

function AgentTab() {
  const [agents, setAgents] = useState<Agent[]>([]);

  useEffect(() => {
    App.ListAgents().then(v => setAgents(v || [])).catch(() => {});
  }, []);

  return (
    <div className="agent-tab">
      {agents.map(agent => (
        <section key={agent.id} className="card agent-card">
          <div className="agent-header">
            <span className={`status-dot ${agent.enabled ? 'online' : 'offline'}`} />
            <h3>{agent.name}</h3>
            <label className="switch">
              <input type="checkbox" checked={agent.enabled}
                onChange={e => App.UpdateAgent({ ...agent, enabled: e.target.checked })} />
              启用
            </label>
          </div>
          <div className="agent-info">
            <label>命令: <code>{agent.command}</code></label>
          </div>
        </section>
      ))}
    </div>
  );
}

export default AgentTab;
