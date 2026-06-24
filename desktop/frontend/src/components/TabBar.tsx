type Tab = 'device' | 'agent' | 'history';

interface Props {
  active: Tab;
  onChange: (tab: Tab) => void;
}

const TABS: { key: Tab; label: string }[] = [
  { key: 'device', label: '设备' },
  { key: 'agent', label: 'Agent' },
  { key: 'history', label: '历史' },
];

function TabBar({ active, onChange }: Props) {
  return (
    <nav className="tab-bar">
      {TABS.map(tab => (
        <button
          key={tab.key}
          className={`tab-btn ${active === tab.key ? 'active' : ''}`}
          onClick={() => onChange(tab.key)}
        >
          {tab.label}
        </button>
      ))}
    </nav>
  );
}

export default TabBar;
