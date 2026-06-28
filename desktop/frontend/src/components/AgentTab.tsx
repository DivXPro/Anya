import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Events } from '@wailsio/runtime';
import { App } from '../../bindings/desktop';
import type { Agent } from '../../bindings/desktop/internal/store/models';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { RiCheckLine, RiLoader4Line } from '@remixicon/react';
import {
  DetectAgents,
  InstallAgent,
  IsAgentInstalling,
  GetPackageManager,
  GetAgentInstallCommand,
  EventInstallStarted,
  EventInstallFinished,
  EventInstallFailed,
} from '@/lib/agent-api';

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

export function AgentLogo({ id, name }: { id: string; name: string }) {
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

interface InstallEventData {
  agentID: string;
  version?: string;
  error?: string;
}

function AgentTab() {
  const { t } = useTranslation();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [installingIds, setInstallingIds] = useState<Set<string>>(new Set());
  const [missingPmAgent, setMissingPmAgent] = useState<Agent | null>(null);
  const [manualCommand, setManualCommand] = useState<string>('');

  const refreshAgents = async () => {
    try {
      const v = await App.ListAgents();
      setAgents(v || []);
    } catch (e) {
      console.error(e);
    }
  };

  const refreshInstalling = async (list: Agent[]) => {
    const next = new Set<string>();
    await Promise.all(
      list.map(async (ag) => {
        try {
          if (await IsAgentInstalling(ag.id)) {
            next.add(ag.id);
          }
        } catch {
          // ignore
        }
      })
    );
    setInstallingIds(next);
  };

  useEffect(() => {
    let mounted = true;

    (async () => {
      try {
        await DetectAgents();
      } catch {
        // ignore; list will still return agents
      }
      if (!mounted) return;
      const list = await App.ListAgents();
      if (!mounted) return;
      setAgents(list || []);
      await refreshInstalling(list || []);
    })();

    return () => {
      mounted = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const offStarted = Events.On(EventInstallStarted, (event) => {
      const data = event.data as InstallEventData;
      if (data?.agentID) {
        setInstallingIds((prev) => new Set(prev).add(data.agentID));
      }
    });

    const offFinished = Events.On(EventInstallFinished, () => {
      setInstallingIds((prev) => {
        const next = new Set(prev);
        // Refresh will update precise state; clear all for simplicity.
        next.clear();
        return next;
      });
      refreshAgents();
    });

    const offFailed = Events.On(EventInstallFailed, () => {
      setInstallingIds((prev) => {
        const next = new Set(prev);
        next.clear();
        return next;
      });
      refreshAgents();
    });

    return () => {
      offStarted();
      offFinished();
      offFailed();
    };
  }, []);

  const selectAgent = async (agent: Agent) => {
    if (agent.selected || !agent.installed) return;
    try {
      await App.SelectAgent(agent.id);
      await refreshAgents();
    } catch (e) {
      console.error(e);
    }
  };

  const handleInstall = async (agent: Agent) => {
    try {
      const pm = await GetPackageManager();
      if (!pm) {
        const cmd = await GetAgentInstallCommand(agent.id);
        setManualCommand(cmd || '');
        setMissingPmAgent(agent);
        return;
      }
      setInstallingIds((prev) => new Set(prev).add(agent.id));
      await InstallAgent(agent.id);
    } catch (e) {
      console.error(e);
      setInstallingIds((prev) => {
        const next = new Set(prev);
        next.delete(agent.id);
        return next;
      });
    }
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
        {agents.map((agent) => {
          const installing = installingIds.has(agent.id);
          const selectable = agent.installed && !installing;
          return (
            <div
              key={agent.id}
              onClick={() => selectable && selectAgent(agent)}
              className={`flex w-full items-center justify-between border-b p-3 text-left transition-colors last:border-b-0 ${
                selectable ? 'hover:bg-accent cursor-pointer' : 'cursor-default'
              } ${agent.selected ? 'bg-accent/50' : ''}`}
            >
              <div className="flex items-center gap-3">
                <div
                  className={`flex h-5 w-5 items-center justify-center rounded-full border-2 ${
                    agent.selected && agent.installed
                      ? 'border-primary bg-primary'
                      : 'border-muted-foreground'
                  }`}
                >
                  {agent.selected && agent.installed && (
                    <div className="h-2 w-2 rounded-full bg-primary-foreground" />
                  )}
                </div>
                <AgentLogo id={agent.id} name={agent.name} />
                <div>
                  <p className="text-sm font-medium">{agent.name}</p>
                  <p className="text-xs text-muted-foreground">{agent.version || agent.id}</p>
                </div>
              </div>

              {agent.installed ? (
                agent.selected ? (
                  <Badge
                    variant="secondary"
                    className="gap-1 bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                  >
                    <RiCheckLine className="h-3 w-3" />
                    {t('agent.active')}
                  </Badge>
                ) : (
                  <Badge variant="secondary">{t('agent.installed')}</Badge>
                )
              ) : installing ? (
                <Badge variant="outline" className="gap-1">
                  <RiLoader4Line className="h-3 w-3 animate-spin" />
                  {t('agent.installing')}
                </Badge>
              ) : (
                <Button
                  size="sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleInstall(agent);
                  }}
                >
                  {t('agent.install')}
                </Button>
              )}
            </div>
          );
        })}
      </div>

      <Dialog open={missingPmAgent !== null} onOpenChange={(open) => !open && setMissingPmAgent(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('agent.nodeRequiredTitle')}</DialogTitle>
            <DialogDescription>
              {t('agent.nodeRequiredDesc', { name: missingPmAgent?.name || '' })}
            </DialogDescription>
          </DialogHeader>
          <div className="rounded-md bg-muted p-3">
            <code className="block break-all text-sm font-mono">
              {manualCommand || t('agent.installCommandPlaceholder')}
            </code>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default AgentTab;
