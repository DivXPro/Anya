import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { Message, Agent, Session } from '../../bindings/desktop/internal/store/models';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Search, Calendar, User, BrainResearch } from 'iconoir-react';
import { AgentLogo } from './AgentTab';

function HistoryTab() {
  const { t } = useTranslation();
  const [messages, setMessages] = useState<Message[]>([]);
  const [search, setSearch] = useState('');
  const [agents, setAgents] = useState<Agent[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);

  const loadMessages = async () => {
    try {
      const result = search.trim()
        ? await App.SearchMessages(search.trim(), 100)
        : await App.ListMessages(100, 0);
      setMessages(result || []);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    loadMessages();

    App.ListAgents()
      .then((v) => setAgents(v || []))
      .catch(console.error);

    App.ListSessions(1000, 0)
      .then((v) => setSessions(v || []))
      .catch(console.error);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(loadMessages, 300);
    return () => window.clearTimeout(timer);
  }, [search]);

  const grouped = useMemo(() => {
    const groups: Record<string, Message[]> = {};
    for (const m of messages) {
      const date = m.created_at.split('T')[0];
      (groups[date] ||= []).push(m);
    }
    return groups;
  }, [messages]);

  const agentById = useMemo(() => {
    const map: Record<string, Agent> = {};
    for (const a of agents) map[a.id] = a;
    return map;
  }, [agents]);

  const sessionAgentIdById = useMemo(() => {
    const map: Record<string, string> = {};
    for (const s of sessions) map[s.id] = s.agent_id;
    return map;
  }, [sessions]);

  const displayText = (m: Message) => {
    return m.content || t('history.noContent');
  };

  const renderBubble = (m: Message) => {
    const isUser = m.role === 'user';
    const agentId = sessionAgentIdById[m.session_id];
    const agent = agentId ? agentById[agentId] : undefined;
    const fullTime = new Date(m.created_at).toLocaleString();

    return (
      <div key={m.id} className={`flex gap-3 ${isUser ? 'flex-row-reverse' : ''}`}>
        <div className="flex flex-shrink-0 flex-col items-center gap-1">
          {isUser ? (
            <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary text-primary-foreground">
              <User className="h-5 w-5" />
            </div>
          ) : (
            <div className="flex h-9 w-9 items-center justify-center overflow-hidden rounded-full bg-muted">
              {agent ? (
                <AgentLogo id={agent.id} name={agent.name} />
              ) : (
                <BrainResearch className="h-5 w-5 text-muted-foreground" />
              )}
            </div>
          )}
        </div>

        <div className={`flex max-w-[80%] flex-col ${isUser ? 'items-end' : 'items-start'}`}>
          <div
            title={fullTime}
            className={`rounded-2xl px-4 py-2 text-sm shadow-sm ${
              isUser
                ? 'rounded-br-sm bg-primary text-primary-foreground'
                : 'rounded-bl-sm bg-muted/80 text-foreground'
            }`}
          >
            <p className="whitespace-pre-wrap">{displayText(m)}</p>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-4">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">{t('tabs.history')}</h1>
          <p className="text-sm text-muted-foreground">{t('history.subtitle')}</p>
        </div>
        <div className="relative w-64">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={t('history.search')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      <div className="rounded-xl border bg-card/50 p-3">
        <ScrollArea className="h-[calc(100vh-156px)] pr-2">
          <div className="space-y-5">
            {Object.entries(grouped).length === 0 && (
              <div className="py-12 text-center text-sm text-muted-foreground">
                {t('history.empty')}
              </div>
            )}
            {Object.entries(grouped).map(([date, group]) => (
              <div key={date}>
                <div className="mb-3 flex items-center justify-center gap-2 text-xs font-medium text-muted-foreground">
                  <Calendar className="h-3.5 w-3.5" />
                  {date}
                </div>
                <div className="space-y-4">
                  {group.map((m) => renderBubble(m))}
                </div>
              </div>
            ))}
          </div>
        </ScrollArea>
      </div>
    </div>
  );
}

export default HistoryTab;
