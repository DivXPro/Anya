import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { Agent } from '../../bindings/desktop/internal/store/models';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { Bot, Terminal } from 'lucide-react';

function AgentTab() {
  const [agents, setAgents] = useState<Agent[]>([]);

  useEffect(() => {
    App.ListAgents().then((v) => setAgents(v || [])).catch(() => {});
  }, []);

  const toggleAgent = async (agent: Agent, enabled: boolean) => {
    const updated = { ...agent, enabled };
    await App.UpdateAgent(updated);
    setAgents((prev) =>
      prev.map((a) => (a.id === agent.id ? { ...a, enabled } : a))
    );
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Agent</h1>
        <p className="text-sm text-muted-foreground">管理可用的 AI Agent 适配器</p>
      </div>

      <div className="grid gap-4">
        {agents.map((agent) => (
          <Card key={agent.id}>
            <CardHeader className="pb-3">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
                    <Bot className="h-5 w-5" />
                  </div>
                  <div>
                    <CardTitle className="text-base">{agent.name}</CardTitle>
                    <CardDescription>{agent.id}</CardDescription>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Switch
                    checked={agent.enabled}
                    onCheckedChange={(checked) => toggleAgent(agent, checked)}
                  />
                  <Badge
                    variant="secondary"
                    className={
                      agent.enabled
                        ? 'bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400'
                        : ''
                    }
                  >
                    {agent.enabled ? '已启用' : '已禁用'}
                  </Badge>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-2 rounded-md bg-muted p-3">
                <Terminal className="h-4 w-4 text-muted-foreground" />
                <code className="text-xs">{agent.command}</code>
              </div>
            </CardContent>
          </Card>
        ))}
        {agents.length === 0 && (
          <Card>
            <CardContent className="py-8 text-center text-sm text-muted-foreground">
              暂无 Agent 配置
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}

export default AgentTab;
