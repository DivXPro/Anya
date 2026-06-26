import { useEffect, useMemo, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { Session, Message } from '../../bindings/desktop/internal/store/models';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { Search, MessageSquare, Calendar, User, Bot } from 'lucide-react';

function HistoryTab() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [expandedSession, setExpandedSession] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [search, setSearch] = useState('');

  useEffect(() => {
    App.ListSessions(50, 0).then((v) => setSessions(v || [])).catch(() => {});
  }, []);

  const expandSession = async (sessionId: string) => {
    if (expandedSession === sessionId) {
      setExpandedSession(null);
      setMessages([]);
      return;
    }
    setExpandedSession(sessionId);
    try {
      const result = await App.GetSessionMessages(sessionId);
      setMessages(result || []);
    } catch (e) {
      console.error(e);
    }
  };

  const grouped = useMemo(() => {
    const groups: Record<string, Session[]> = {};
    for (const s of sessions) {
      if (search && !s.agent_id.toLowerCase().includes(search.toLowerCase())) {
        continue;
      }
      const date = s.created_at.split('T')[0];
      (groups[date] ||= []).push(s);
    }
    return groups;
  }, [sessions, search]);

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">历史</h1>
          <p className="text-sm text-white/50">查看与设备的对话记录</p>
        </div>
        <div className="relative w-64">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-white/40" />
          <Input
            placeholder="搜索 Agent..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="border-white/10 bg-white/5 pl-9 text-white placeholder:text-white/30"
          />
        </div>
      </div>

      <ScrollArea className="h-[calc(100vh-180px)] pr-2">
        <div className="space-y-6">
          {Object.entries(grouped).length === 0 && (
            <Card className="border-white/10 bg-[#2e2e2e]">
              <CardContent className="py-12 text-center text-sm text-white/40">
                暂无历史记录
              </CardContent>
            </Card>
          )}
          {Object.entries(grouped).map(([date, group]) => (
            <div key={date}>
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-white/60">
                <Calendar className="h-4 w-4" />
                {date}
              </div>
              <div className="space-y-3">
                {group.map((s) => (
                  <Card
                    key={s.id}
                    className="cursor-pointer border-white/10 bg-[#2e2e2e] transition-colors hover:bg-[#333333]"
                    onClick={() => expandSession(s.id)}
                  >
                    <CardHeader className="pb-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <MessageSquare className="h-4 w-4 text-white/40" />
                          <CardTitle className="text-sm font-medium text-white">
                            {s.created_at.slice(11, 16)}
                          </CardTitle>
                        </div>
                        <Badge variant="secondary" className="bg-white/10 text-white/70">
                          {s.agent_id}
                        </Badge>
                      </div>
                    </CardHeader>
                    {expandedSession === s.id && messages.length > 0 && (
                      <CardContent className="pt-0">
                        <Separator className="mb-3 bg-white/10" />
                        <div className="space-y-3">
                          {messages.map((m) => (
                            <div key={m.id} className="flex gap-3">
                              <div className="mt-0.5">
                                {m.role === 'user' ? (
                                  <User className="h-4 w-4 text-blue-400" />
                                ) : (
                                  <Bot className="h-4 w-4 text-emerald-400" />
                                )}
                              </div>
                              <div className="flex-1">
                                <p className="text-sm text-white/90">
                                  {m.summary || m.content}
                                </p>
                                {m.summary && m.summary !== m.content && (
                                  <p className="mt-1 text-xs text-white/40">{m.content}</p>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      </CardContent>
                    )}
                  </Card>
                ))}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
    </div>
  );
}

export default HistoryTab;
