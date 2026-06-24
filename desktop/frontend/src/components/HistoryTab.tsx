import { useEffect, useState } from 'react';
import { App } from '../../bindings/desktop';
import type { Session, Message } from '../../bindings/desktop/internal/store/models';

function HistoryTab() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [expandedSession, setExpandedSession] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);

  useEffect(() => {
    App.ListSessions(50, 0).then(v => setSessions(v || [])).catch(() => {});
  }, []);

  const expandSession = async (sessionId: string) => {
    setExpandedSession(sessionId);
    try {
      const result = await App.GetSessionMessages(sessionId);
      setMessages(result || []);
    } catch (e) {
      console.error(e);
    }
  };

  const groupByDate = (sessions: Session[]) => {
    const groups: Record<string, Session[]> = {};
    for (const s of sessions) {
      const date = s.created_at.split('T')[0];
      (groups[date] ||= []).push(s);
    }
    return groups;
  };

  return (
    <div className="history-tab">
      <input type="search" placeholder="搜索..." className="search-box" />
      {Object.entries(groupByDate(sessions)).map(([date, group]) => (
        <section key={date}>
          <h4>{date}</h4>
          {group.map(s => (
            <div key={s.id} className="session-item" onClick={() => expandSession(s.id)}>
              <span className="time">{s.created_at.slice(11, 16)}</span>
              <span className="agent-name">{s.agent_id}</span>
              {expandedSession === s.id && (
                <div className="messages">
                  {messages.map(m => (
                    <p key={m.id} className={`msg ${m.role}`}>{m.summary || m.content}</p>
                  ))}
                </div>
              )}
            </div>
          ))}
        </section>
      ))}
    </div>
  );
}

export default HistoryTab;
