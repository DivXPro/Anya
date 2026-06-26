import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App } from '../../bindings/desktop';
import type { Message } from '../../bindings/desktop/internal/store/models';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Search, Calendar, User, BrainResearch } from 'iconoir-react';

function HistoryTab() {
  const { t } = useTranslation();
  const [messages, setMessages] = useState<Message[]>([]);
  const [search, setSearch] = useState('');

  useEffect(() => {
    loadMessages();
  }, []);

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

  // Reload when search changes (debounced by the user typing).
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

  const displayText = (m: Message) => {
    if (m.summary && m.summary.trim()) return m.summary;
    return m.content || t('history.noContent');
  };

  return (
    <div className="space-y-6">
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

      <ScrollArea className="h-[calc(100vh-180px)] pr-2">
        <div className="space-y-6">
          {Object.entries(grouped).length === 0 && (
            <Card>
              <CardContent className="py-12 text-center text-sm text-muted-foreground">
                {t('history.empty')}
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
                {group.map((m) => (
                  <Card key={m.id}>
                    <CardContent className="py-3">
                      <div className="flex gap-3">
                        <div className="mt-0.5">
                          {m.role === 'user' ? (
                            <User className="h-4 w-4 text-primary" />
                          ) : (
                            <BrainResearch className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                          )}
                        </div>
                        <div className="flex-1 space-y-1">
                          <div className="flex items-center justify-between">
                            <span className="text-xs font-medium text-muted-foreground">
                              {m.role === 'user' ? t('history.roleUser') : t('history.roleAssistant')}
                            </span>
                            <span className="text-xs text-muted-foreground">
                              {m.created_at.slice(11, 16)}
                            </span>
                          </div>
                          <p className="text-sm">{displayText(m)}</p>
                          {m.summary && m.summary !== m.content && (
                            <p className="text-xs text-muted-foreground">{m.content}</p>
                          )}
                        </div>
                      </div>
                    </CardContent>
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
