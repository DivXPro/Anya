import { cn } from '@/lib/utils';
import type { Tab } from '@/App';
import { Smartphone, Bot, History } from 'lucide-react';

interface SidebarProps {
  active: Tab;
  onChange: (tab: Tab) => void;
}

const ITEMS: { key: Tab; label: string; icon: typeof Smartphone }[] = [
  { key: 'device', label: '设备', icon: Smartphone },
  { key: 'agent', label: 'Agent', icon: Bot },
  { key: 'history', label: '历史', icon: History },
];

export function Sidebar({ active, onChange }: SidebarProps) {
  return (
    <aside className="flex w-[220px] flex-col justify-between bg-[#3a3a3a] p-4">
      <div>
        <div className="mb-8 flex items-center gap-3 px-2">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-white/10">
            <span className="text-lg font-bold text-white">E</span>
          </div>
          <span className="text-lg font-semibold tracking-tight text-white">Elf</span>
        </div>
        <nav className="space-y-1">
          {ITEMS.map((item) => {
            const Icon = item.icon;
            const isActive = active === item.key;
            return (
              <button
                key={item.key}
                onClick={() => onChange(item.key)}
                className={cn(
                  'flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-white/10 text-white'
                    : 'text-white/60 hover:bg-white/5 hover:text-white'
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </button>
            );
          })}
        </nav>
      </div>
      <div className="px-2 text-xs text-white/40">
        Hardware Agent
      </div>
    </aside>
  );
}
