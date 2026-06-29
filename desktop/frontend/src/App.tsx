import { useState, useEffect, useRef } from 'react';
import { Sidebar } from '@/components/Sidebar';
import DeviceTab from '@/components/DeviceTab';
import AgentTab from '@/components/AgentTab';
import HistoryTab from '@/components/HistoryTab';
import SettingsTab from '@/components/SettingsTab';
import { useThemeInit } from '@/hooks/useThemeInit';
import { Events } from '@wailsio/runtime';

export type Tab = 'device' | 'agent' | 'history' | 'settings';

function App() {
  useThemeInit();
  const [activeTab, setActiveTab] = useState<Tab>('device');
  const workingDirectoryRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const off = Events.On('navigate-to-working-directory', () => {
      setActiveTab('settings');
      // Wait for the next paint so SettingsTab is mounted and the DOM updated
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          workingDirectoryRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
        });
      });
    });
    return () => off();
  }, []);

  return (
    <div className="flex h-full w-full overflow-hidden bg-background/80 text-foreground backdrop-blur-xl">
      <Sidebar active={activeTab} onChange={setActiveTab} />
      <main className="relative flex-1 overflow-hidden bg-background/60">
        <div className="absolute inset-0 overflow-y-auto p-6">
          {activeTab === 'device' && <DeviceTab />}
          {activeTab === 'agent' && <AgentTab />}
          {activeTab === 'history' && <HistoryTab />}
          {activeTab === 'settings' && <SettingsTab workingDirectoryRef={workingDirectoryRef} />}
        </div>
      </main>
    </div>
  );
}

export default App;
