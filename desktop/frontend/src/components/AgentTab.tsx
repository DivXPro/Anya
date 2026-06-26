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
        <h1 className="text-2xl font-semibold text-white">Agent</h1>
        <p className="text-sm text-white/50">管理可用的 AI Agent 适配器</p>
      </div>

      <div className="grid gap-4">
        {agents.map((agent) => (
          <Card key={agent.id} className="border-white/10 bg-[#2e2e2e]">
            <CardHeader className="pb-3">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-white/10">
                    <Bot className="h-5 w-5 text-white" />
                  </div>
                  <div>
                    <CardTitle className="text-base text-white">{agent.name}</CardTitle>
                    <CardDescription className="text-white/50">{agent.id}</CardDescription>
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
                        ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-400'
                        : 'border-white/10 bg-white/5 text-white/40'
                    }
                  >
                    {agent.enabled ? '已启用' : '已禁用'}
                  </Badge>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-2 rounded-md bg-black/20 p-3">
                <Terminal className="h-4 w-4 text-white/40" />
                <code className="text-xs text-white/70">{agent.command}</code>
              </div>
            </CardContent>
          </Card>
        ))}
        {agents.length === 0 && (
          <Card className="border-white/10 bg-[#2e2e2e]">
            <CardContent className="py-8 text-center text-sm text-white/40">
              暂无 Agent 配置
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}

export default AgentTab;
