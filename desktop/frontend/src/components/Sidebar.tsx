import { cn } from '@/lib/utils';
import type { Tab } from '@/App';
import { SmartphoneDevice, BrainResearch, ClockRotateRight } from 'iconoir-react';

interface SidebarProps {
  active: Tab;
  onChange: (tab: Tab) => void;
}

const ITEMS: { key: Tab; label: string; icon: typeof SmartphoneDevice }[] = [
  { key: 'device', label: '设备', icon: SmartphoneDevice },
  { key: 'agent', label: 'Agent', icon: BrainResearch },
  { key: 'history', label: '历史', icon: ClockRotateRight },
];

export function Sidebar({ active, onChange }: SidebarProps) {
  return (
    <aside className="flex w-[220px] flex-col justify-between border-r border-border/50 bg-secondary/80 p-4 backdrop-blur-md">
      <div>
        <div className="mb-8 flex items-center gap-3 px-2">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <span className="text-lg font-bold">E</span>
          </div>
          <span className="text-lg font-semibold tracking-tight">Elf</span>
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
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </button>
            );
          })}
        </nav>
      </div>
      <div className="px-2 text-xs text-muted-foreground">
        Hardware Agent
      </div>
    </aside>
  );
}
