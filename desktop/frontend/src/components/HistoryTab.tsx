import { useEffect, useMemo, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { Session, Message } from '../../bindings/desktop/internal/store/models';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { Search, ChatBubble, Calendar, User, BrainResearch } from 'iconoir-react';

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
          <h1 className="text-2xl font-semibold">历史</h1>
          <p className="text-sm text-muted-foreground">查看与设备的对话记录</p>
        </div>
        <div className="relative w-64">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="搜索 Agent..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      <ScrollArea className="h-[calc(100vh-180px)] pr-2">
        <div className="space-y-6">
          {Object.entries(grouped).length === 0 && (
            <Card>
              <CardContent className="py-12 text-center text-sm text-muted-foreground">
                暂无历史记录
              </CardContent>
            </Card>
          )}
          {Object.entries(grouped).map(([date, group]) => (
            <div key={date}>
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-muted-foreground">
                <Calendar className="h-4 w-4" />
                {date}
              </div>
              <div className="space-y-3">
                {group.map((s) => (
                  <Card
                    key={s.id}
                    className="cursor-pointer transition-colors hover:bg-accent"
                    onClick={() => expandSession(s.id)}
                  >
                    <CardHeader className="pb-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <ChatBubble className="h-4 w-4 text-muted-foreground" />
                          <CardTitle className="text-sm font-medium">
                            {s.created_at.slice(11, 16)}
                          </CardTitle>
                        </div>
                        <Badge variant="secondary">
                          {s.agent_id}
                        </Badge>
                      </div>
                    </CardHeader>
                    {expandedSession === s.id && messages.length > 0 && (
                      <CardContent className="pt-0">
                        <Separator className="mb-3" />
                        <div className="space-y-3">
                          {messages.map((m) => (
                            <div key={m.id} className="flex gap-3">
                              <div className="mt-0.5">
                                {m.role === 'user' ? (
                                  <User className="h-4 w-4 text-primary" />
                                ) : (
                                  <BrainResearch className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                                )}
                              </div>
                              <div className="flex-1">
                                <p className="text-sm">
                                  {m.summary || m.content}
                                </p>
                                {m.summary && m.summary !== m.content && (
                                  <p className="mt-1 text-xs text-muted-foreground">{m.content}</p>
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
