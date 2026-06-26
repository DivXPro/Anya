import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { Agent } from '../../bindings/desktop/internal/store/models';
import { Badge } from '@/components/ui/badge';
import { Check } from 'iconoir-react';

const agentLogo: Record<string, string> = {
  'claude-code': '/claude-logo.svg',
  opencode: '/opencode.png',
  codex: '/codex.png',
  cursor: '/cursor.png',
  droid: '/droid.svg',
  hermes: '/hermes.png',
  kimi: '/kimi.png',
  mimo: '/mimo.png',
  omp: '/omp.svg',
  pi: '/pi.svg',
  qwen: '/qwen.png',
};

function AgentLogo({ id, name }: { id: string; name: string }) {
  const src = agentLogo[id];
  if (!src) {
    return (
      <div className="flex h-9 w-9 items-center justify-center text-xs font-medium text-muted-foreground">
        {name.slice(0, 2)}
      </div>
    );
  }
  return (
    <div className="flex h-9 w-9 items-center justify-center p-1">
      <img
        src={src}
        alt={name}
        className="h-full w-full object-contain"
      />
    </div>
  );
}

function AgentTab() {
  const { t } = useTranslation();
  const [agents, setAgents] = useState<Agent[]>([]);

  useEffect(() => {
    App.ListAgents().then((v) => setAgents(v || [])).catch(() => {});
  }, []);

  const refreshAgents = async () => {
    try {
      const v = await App.ListAgents();
      setAgents(v || []);
    } catch (e) {
      console.error(e);
    }
  };

  const selectAgent = async (agent: Agent) => {
    if (agent.selected) return;
    await App.SelectAgent(agent.id);
    await refreshAgents();
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{t('tabs.agent')}</h1>
        <p className="text-sm text-muted-foreground">{t('agent.subtitle')}</p>
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
              agent.selected ? 'bg-accent/50' : ''
            }`}
          >
            <div className="flex items-center gap-3">
              <div
                className={`flex h-5 w-5 items-center justify-center rounded-full border-2 ${
                  agent.selected
                    ? 'border-primary bg-primary'
                    : 'border-muted-foreground'
                }`}
              >
                {agent.selected && <div className="h-2 w-2 rounded-full bg-primary-foreground" />}
              </div>
              <AgentLogo id={agent.id} name={agent.name} />
              <div>
                <p className="text-sm font-medium">{agent.name}</p>
                <p className="text-xs text-muted-foreground">{agent.version || agent.id}</p>
              </div>
            </div>
            {agent.enabled ? (
              agent.selected ? (
                <Badge
                  variant="secondary"
                  className="gap-1 bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                >
                  <Check className="h-3 w-3" />
                  {t('agent.enabled')}
                </Badge>
              ) : (
                <Badge variant="secondary">{t('agent.available')}</Badge>
              )
            ) : (
              <Badge variant="outline">{t('agent.notInstalled')}</Badge>
            )}
          </button>
        ))}
      </div>
    </div>
  );
}

export default AgentTab;
