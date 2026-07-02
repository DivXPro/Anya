import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { App } from "../../bindings/desktop";import type {
  Message,
  Agent,
  Session,
} from "../../bindings/desktop/internal/store/models";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { RiSearchLine, RiUserLine, RiRobot2Line } from "@remixicon/react";
import { AgentLogo } from "./AgentTab";

const PAGE_SIZE = 25;

function HistoryTab() {
  const { t } = useTranslation();
  const [messages, setMessages] = useState<Message[]>([]);
  const [search, setSearch] = useState("");
  const [agents, setAgents] = useState<Agent[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(true);
  const [loading, setLoading] = useState(false);
  const loadingRef = useRef(false);

  const loadMessages = async (reset = false) => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);

    try {
      const nextOffset = reset ? 0 : offset;

      let result: Message[] = [];
      if (search.trim()) {
        result = (await App.SearchMessages(search.trim(), 100)) || [];
        setHasMore(false);
      } else {
        result = (await App.ListMessages(PAGE_SIZE, nextOffset)) || [];
        setHasMore(result.length === PAGE_SIZE);
      }

      setMessages((prev) => {
        if (reset) return result;
        const existing = new Set(prev.map((m) => m.id));
        const merged = [...prev, ...result.filter((m) => !existing.has(m.id))];
        return merged;
      });
      setOffset(nextOffset + result.length);
    } catch (e) {
      console.error(e);
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  };

  useEffect(() => {
    loadMessages(true);

    App.ListAgents()
      .then((v) => setAgents(v || []))
      .catch(console.error);

    App.ListSessions(1000, 0)
      .then((v) => setSessions(v || []))
      .catch(console.error);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => loadMessages(true), 300);
    return () => window.clearTimeout(timer);
  }, [search]);

  const handleScroll: React.UIEventHandler<HTMLDivElement> = (e) => {
    const target = e.currentTarget;
    const nearBottom =
      target.scrollTop + target.clientHeight >= target.scrollHeight - 80;
    if (nearBottom && hasMore && !loadingRef.current && !search.trim()) {
      loadMessages(false);
    }
  };

  const grouped = useMemo(() => {
    const groups: Record<string, Message[]> = {};
    for (const m of messages) {
      const date = m.created_at.split("T")[0];
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
    return m.content || t("history.noContent");
  };

  const renderBubble = (m: Message) => {
    const isUser = m.role === "user";
    const agentId = sessionAgentIdById[m.session_id ?? ""];
    const agent = agentId ? agentById[agentId] : undefined;
    const fullTime = new Date(m.created_at).toLocaleString();

    return (
      <div
        key={m.id}
        className={`flex gap-3 ${isUser ? "flex-row-reverse" : ""}`}
      >
        <div className="flex flex-shrink-0 flex-col items-center gap-1">
          {isUser ? (
            <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary text-primary-foreground">
              <RiUserLine className="h-5 w-5" />
            </div>
          ) : (
            <div className="flex h-9 w-9 items-center justify-center overflow-hidden rounded-full bg-muted">
              {agent ? (
                <AgentLogo id={agent.id} name={agent.name} />
              ) : (
                <RiRobot2Line className="h-5 w-5 text-muted-foreground" />
              )}
            </div>
          )}
        </div>

        <div
          className={`flex max-w-[80%] flex-col ${isUser ? "items-end" : "items-start"}`}
        >
          <span className="mb-1 text-[10px] text-muted-foreground">
            {fullTime}
          </span>
          <div
            className={`rounded-2xl px-4 py-2 text-sm shadow-sm ${
              isUser
                ? "rounded-br-sm bg-primary text-primary-foreground"
                : "rounded-bl-sm bg-muted/80 text-foreground"
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
          <h1 className="text-2xl font-semibold">{t("tabs.history")}</h1>
        </div>
        <div className="relative w-64">
          <RiSearchLine className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={t("history.search")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      <div className="rounded-xl border bg-card/50 p-3">
        <ScrollArea
          onScroll={handleScroll}
          className="h-[calc(100vh-156px)] pr-2"
        >
          <div className="space-y-5">
            {Object.entries(grouped).length === 0 && !loading && (
              <div className="py-12 text-center text-sm text-muted-foreground">
                {t("history.empty")}
              </div>
            )}
            {Object.entries(grouped).map(([date, group]) => (
              <div key={date}>
                <div className="space-y-4">
                  {group.map((m) => renderBubble(m))}
                </div>
              </div>
            ))}
            {loading && (
              <div className="py-3 text-center text-xs text-muted-foreground">
                {t("history.loading")}
              </div>
            )}
          </div>
        </ScrollArea>
      </div>
    </div>
  );
}

export default HistoryTab;
