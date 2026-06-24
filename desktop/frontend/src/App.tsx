import { useState } from 'react';
import TabBar from './components/TabBar';
import DeviceTab from './components/DeviceTab';
import AgentTab from './components/AgentTab';
import HistoryTab from './components/HistoryTab';

type Tab = 'device' | 'agent' | 'history';

function App() {
  const [activeTab, setActiveTab] = useState<Tab>('device');

  return (
    <div className="app">
      <TabBar active={activeTab} onChange={setActiveTab} />
      <main className="tab-content">
        {activeTab === 'device' && <DeviceTab />}
        {activeTab === 'agent' && <AgentTab />}
        {activeTab === 'history' && <HistoryTab />}
      </main>
    </div>
  );
}

export default App;
