import { cn } from '@/lib/utils';
import { useTranslation } from 'react-i18next';
import type { Tab } from '@/App';
import RemoteControlLineIcon from 'remixicon-react/RemoteControlLineIcon';
import { RiRobot2Line } from '@remixicon/react';
import HistoryLineIcon from 'remixicon-react/HistoryLineIcon';
import SettingsLineIcon from 'remixicon-react/SettingsLineIcon';

const ITEMS: { key: Tab; labelKey: string; icon: typeof RemoteControlLineIcon | typeof RiRobot2Line }[] = [
  { key: 'device', labelKey: 'tabs.device', icon: RemoteControlLineIcon },
  { key: 'agent', labelKey: 'tabs.agent', icon: RiRobot2Line },
  { key: 'history', labelKey: 'tabs.history', icon: HistoryLineIcon },
  { key: 'settings', labelKey: 'tabs.settings', icon: SettingsLineIcon },
];

export function Sidebar({ active, onChange }: { active: Tab; onChange: (tab: Tab) => void }) {
  const { t } = useTranslation();

  return (
    <aside className="flex w-[220px] flex-col justify-between border-r border-border/50 bg-secondary/80 px-4 pb-4 pt-10 backdrop-blur-md">
      <div>
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
                {t(item.labelKey)}
              </button>
            );
          })}
        </nav>
      </div>
      <div className="px-2 text-xs text-muted-foreground">
        {t('app.tagline')}
      </div>
    </aside>
  );
}
