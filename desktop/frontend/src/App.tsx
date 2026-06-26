import { useState } from 'react';
import { Sidebar } from '@/components/Sidebar';
import DeviceTab from '@/components/DeviceTab';
import AgentTab from '@/components/AgentTab';
import HistoryTab from '@/components/HistoryTab';

export type Tab = 'device' | 'agent' | 'history';

function App() {
  const [activeTab, setActiveTab] = useState<Tab>('device');

  return (
    <div className="flex h-screen w-screen overflow-hidden rounded-[20px] bg-[#2a2a2a] text-foreground">
      <Sidebar active={activeTab} onChange={setActiveTab} />
      <main className="relative flex-1 overflow-hidden bg-[#222222]">
        <div className="absolute inset-0 overflow-y-auto p-6">
          {activeTab === 'device' && <DeviceTab />}
          {activeTab === 'agent' && <AgentTab />}
          {activeTab === 'history' && <HistoryTab />}
        </div>
      </main>
    </div>
  );
}

export default App;
