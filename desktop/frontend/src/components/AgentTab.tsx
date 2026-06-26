import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { Agent } from '../../bindings/desktop/internal/store/models';
import { Badge } from '@/components/ui/badge';
import { BrainResearch, Check } from 'iconoir-react';

function AgentTab() {
  const [agents, setAgents] = useState<Agent[]>([]);

  useEffect(() => {
    App.ListAgents().then((v) => setAgents(v || [])).catch(() => {});
  }, []);

  const selectAgent = async (agent: Agent) => {
    if (agent.enabled) return;
    await App.SelectAgent(agent.id);
    setAgents((prev) =>
      prev.map((a) => ({ ...a, enabled: a.id === agent.id }))
    );
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Agent</h1>
        <p className="text-sm text-muted-foreground">选择当前启用的 AI Agent</p>
      </div>

      <div className="rounded-lg border bg-card">
        {agents.length === 0 && (
          <div className="h-12" />
        )}
        {agents.map((agent) => (
          <button
            key={agent.id}
            onClick={() => selectAgent(agent)}
            className={`flex w-full items-center justify-between border-b p-3 text-left transition-colors last:border-b-0 hover:bg-accent ${
              agent.enabled ? 'bg-accent/50' : ''
            }`}
          >
            <div className="flex items-center gap-3">
              <div
                className={`flex h-5 w-5 items-center justify-center rounded-full border-2 ${
                  agent.enabled
                    ? 'border-primary bg-primary'
                    : 'border-muted-foreground'
                }`}
              >
                {agent.enabled && <div className="h-2 w-2 rounded-full bg-primary-foreground" />}
              </div>
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
                <BrainResearch className="h-5 w-5" />
              </div>
              <div>
                <p className="text-sm font-medium">{agent.name}</p>
                <p className="text-xs text-muted-foreground">{agent.id}</p>
              </div>
            </div>
            {agent.enabled ? (
              <Badge
                variant="secondary"
                className="gap-1 bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
              >
                <Check className="h-3 w-3" />
                已启用
              </Badge>
            ) : (
              <Badge variant="secondary">可选</Badge>
            )}
          </button>
        ))}
      </div>
    </div>
  );
}

export default AgentTab;
