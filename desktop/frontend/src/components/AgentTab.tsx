import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Events } from '@wailsio/runtime';
import { Dialogs } from '@wailsio/runtime';
import { App } from '../../bindings/desktop';
import type { Agent } from '../../bindings/desktop/internal/store/models';
import { Badge, badgeVariants } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { RiCheckLine, RiLoader4Line, RiFolderLine, RiFolderOpenLine } from '@remixicon/react';
import {
  DetectAgents,
  InstallAgent,
  CancelAgentInstall,
  IsAgentInstalling,
  GetAgentInstallCommand,
  CheckAgentUpdates,
  EventInstallStarted,
  EventInstallFinished,
  EventInstallFailed,
  EventInstallProgress,
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
  line?: string;
}

interface AgentTabProps {
  workingDirectoryRef: React.RefObject<HTMLDivElement>;
}

function AgentTab({ workingDirectoryRef }: AgentTabProps) {
  const { t } = useTranslation();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [installingIds, setInstallingIds] = useState<Set<string>>(new Set());
  const [installProgress, setInstallProgress] = useState<Record<string, string>>({});
  const [agentUpdates, setAgentUpdates] = useState<Record<string, string>>({});
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [doneFlash, setDoneFlash] = useState<Record<string, 'updated' | 'installed'>>({});
  const [installError, setInstallError] = useState<{ name: string; message: string } | null>(null);
  const [missingPmAgent, setMissingPmAgent] = useState<Agent | null>(null);
  const [manualCommand, setManualCommand] = useState<string>('');
  const [settings, setSettings] = useState<Record<string, string>>({});

  // Tracks installs triggered from the "Update" button (vs a fresh install), so
  // the success flash can say "Updated" instead of "Installed". A ref because
  // the install event listeners are registered once and would capture stale
  // state otherwise.
  const updateClickedRef = useRef<Set<string>>(new Set());
  // Tracks installs the user explicitly canceled, so the resulting failure event
  // is treated as a quiet cancellation rather than surfacing an error dialog.
  const canceledRef = useRef<Set<string>>(new Set());
  // Latest agents list, for lookups inside the once-registered event listeners.
  const agentsRef = useRef<Agent[]>([]);
  useEffect(() => {
    agentsRef.current = agents;
  }, [agents]);

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

  // Query the registry for newer versions of installed agents. Best-effort:
  // failures leave the map empty so no spurious "Update" buttons appear. While
  // running, checkingUpdates drives a spinner on installed rows.
  const refreshUpdates = async () => {
    setCheckingUpdates(true);
    try {
      const map = await CheckAgentUpdates();
      setAgentUpdates(map || {});
    } catch {
      setAgentUpdates({});
    } finally {
      setCheckingUpdates(false);
    }
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
      if (!mounted) return;
      refreshUpdates();
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
        // Reset any leftover progress line from a previous run.
        setInstallProgress((prev) => {
          if (!(data.agentID in prev)) return prev;
          const next = { ...prev };
          delete next[data.agentID];
          return next;
        });
      }
    });

    const offProgress = Events.On(EventInstallProgress, (event) => {
      const data = event.data as InstallEventData;
      if (data?.agentID && data.line) {
        setInstallProgress((prev) => ({ ...prev, [data.agentID]: data.line as string }));
      }
    });

    const clearTransient = (id: string) => {
      setInstallingIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
      setInstallProgress((prev) => {
        if (!(id in prev)) return prev;
        const next = { ...prev };
        delete next[id];
        return next;
      });
    };

    const offFinished = Events.On(EventInstallFinished, (event) => {
      const data = event.data as InstallEventData;
      const id = data?.agentID;
      if (id) {
        clearTransient(id);
        canceledRef.current.delete(id);
        // The update completed — drop any stale "update available" flag right
        // away so the button reverts even before the re-check returns.
        setAgentUpdates((prev) => {
          if (!(id in prev)) return prev;
          const next = { ...prev };
          delete next[id];
          return next;
        });
        const kind = updateClickedRef.current.has(id) ? 'updated' : 'installed';
        updateClickedRef.current.delete(id);
        setDoneFlash((prev) => ({ ...prev, [id]: kind }));
        setTimeout(() => {
          setDoneFlash((prev) => {
            const next = { ...prev };
            delete next[id];
            return next;
          });
        }, 2500);
      }
      refreshAgents();
      // An install may have been an upgrade — re-check so the button reverts.
      refreshUpdates();
    });

    const offFailed = Events.On(EventInstallFailed, (event) => {
      const data = event.data as InstallEventData;
      const id = data?.agentID;
      if (id) {
        clearTransient(id);
        updateClickedRef.current.delete(id);
        // A user-initiated cancel arrives as a failure event; swallow it quietly
        // instead of surfacing an error dialog.
        if (canceledRef.current.has(id)) {
          canceledRef.current.delete(id);
        } else {
          const name = agentsRef.current.find((a) => a.id === id)?.name || id;
          setInstallError({ name, message: data?.error || 'unknown error' });
        }
      }
      refreshAgents();
      refreshUpdates();
    });

    return () => {
      offStarted();
      offProgress();
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

  const handleInstall = async (agent: Agent, isUpdate = false) => {
    if (isUpdate) updateClickedRef.current.add(agent.id);
    setInstallingIds((prev) => new Set(prev).add(agent.id));
    try {
      await InstallAgent(agent.id);
    } catch (e) {
      console.error(e);
      updateClickedRef.current.delete(agent.id);
      setInstallingIds((prev) => {
        const next = new Set(prev);
        next.delete(agent.id);
        return next;
      });
      try {
        const cmd = await GetAgentInstallCommand(agent.id);
        setManualCommand(cmd || '');
      } catch {
        setManualCommand('');
      }
      setMissingPmAgent(agent);
    }
  };

  const handleCancel = async (id: string) => {
    // Mark first so the resulting failure event is treated as a quiet cancel.
    canceledRef.current.add(id);
    try {
      await CancelAgentInstall(id);
    } catch (e) {
      console.error(e);
      canceledRef.current.delete(id);
    }
  };

  useEffect(() => {
    App.GetSettings()
      .then((v) => {
        if (v) {
          const s: Record<string, string> = {};
          for (const [k, val] of Object.entries(v)) {
            s[k] = val ?? '';
          }
          setSettings(s);
        }
      })
      .catch(() => {});
  }, []);

  const updateSetting = async (key: string, value: string) => {
    try {
      await App.SetSetting(key, value);
      setSettings((prev) => ({ ...prev, [key]: value }));
    } catch (err) {
      console.error('failed to set setting', err);
      alert(t('agent.workingDirectory.saveFailed') || 'Failed to save setting');
    }
  };

  const handlePickWorkingDirectory = async () => {
    try {
      const result = await Dialogs.OpenFile({
        CanChooseDirectories: true,
        CanChooseFiles: false,
        Title: t('agent.workingDirectory.dialogTitle'),
        ...(settings.agent_cwd ? { Directory: settings.agent_cwd } : {}),
      });
      if (result) {
        const path = Array.isArray(result) ? result[0] : result;
        if (path) {
          await updateSetting('agent_cwd', path);
        }
      }
    } catch (err) {
      console.error('failed to pick working directory', err);
    }
  };

  const handleResetWorkingDirectory = async () => {
    await updateSetting('agent_cwd', '');
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{t('tabs.agent')}</h1>
      </div>

      <div ref={workingDirectoryRef} className="space-y-3">
        <h2 className="text-base font-semibold">{t('agent.workingDirectory.title')}</h2>
        <div className="rounded-lg border bg-card">
          <div className="flex items-center gap-2 border-b p-3">
            <RiFolderLine className="h-4 w-4 text-muted-foreground shrink-0" />
            <input
              type="text"
              value={settings.agent_cwd || ''}
              readOnly
              placeholder={t('agent.workingDirectory.placeholder')}
              className="h-10 flex-1 rounded-md border px-3 text-sm bg-background"
            />
            <Button onClick={handlePickWorkingDirectory}>
              <RiFolderOpenLine />
              {t('agent.workingDirectory.browse')}
            </Button>
            {settings.agent_cwd && (
              <Button variant="outline" onClick={handleResetWorkingDirectory}>
                {t('agent.workingDirectory.reset')}
              </Button>
            )}
          </div>
          <p className="p-3 text-xs text-muted-foreground">
            {t('agent.workingDirectory.description')}
          </p>
        </div>
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

              {installing ? (
                <div className="flex items-center gap-2">
                  <span
                    className="flex items-center gap-1 max-w-[180px] text-xs text-muted-foreground"
                    title={installProgress[agent.id] || t('agent.installing')}
                  >
                    <RiLoader4Line className="h-3 w-3 shrink-0 animate-spin" />
                    <span className="truncate">
                      {installProgress[agent.id] || t('agent.installing')}
                    </span>
                  </span>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleCancel(agent.id);
                    }}
                    className={cn(badgeVariants({ variant: 'outline' }), 'cursor-pointer')}
                  >
                    {t('agent.cancel')}
                  </button>
                </div>
              ) : doneFlash[agent.id] ? (
                <Badge
                  variant="secondary"
                  className="gap-1 bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                >
                  <RiCheckLine className="h-3 w-3" />
                  {doneFlash[agent.id] === 'updated' ? t('agent.updated') : t('agent.installed')}
                </Badge>
              ) : agent.installed && checkingUpdates ? (
                <span
                  className="flex items-center text-muted-foreground"
                  title={t('agent.checkingUpdate')}
                >
                  <RiLoader4Line className="h-4 w-4 animate-spin" />
                </span>
              ) : agent.installed ? (
                agentUpdates[agent.id] ? (
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleInstall(agent, true);
                    }}
                    className={cn(badgeVariants({ variant: 'default' }), 'cursor-pointer')}
                  >
                    {t('agent.update')}
                  </button>
                ) : agent.selected ? (
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
              ) : (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleInstall(agent);
                  }}
                  className={cn(badgeVariants({ variant: 'default' }), 'cursor-pointer')}
                >
                  {t('agent.install')}
                </button>
              )}
            </div>
          );
        })}
      </div>

      <Dialog open={missingPmAgent !== null} onOpenChange={(open) => !open && setMissingPmAgent(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('agent.installManuallyTitle')}</DialogTitle>
            <DialogDescription>
              {t('agent.installManuallyDesc', { name: missingPmAgent?.name || '' })}
            </DialogDescription>
          </DialogHeader>
          <div className="rounded-md bg-muted p-3">
            <code className="block break-all text-sm font-mono">
              {manualCommand || t('agent.installCommandPlaceholder')}
            </code>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={installError !== null} onOpenChange={(open) => !open && setInstallError(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('agent.installFailedTitle')}</DialogTitle>
            <DialogDescription>
              {t('agent.installFailedDesc', { name: installError?.name || '' })}
            </DialogDescription>
          </DialogHeader>
          <div className="rounded-md bg-muted p-3">
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all text-xs font-mono">
              {installError?.message}
            </pre>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default AgentTab;
