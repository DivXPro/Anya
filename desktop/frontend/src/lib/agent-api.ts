import { Call } from '@wailsio/runtime';

export const EventInstallStarted = 'agent:install:started';
export const EventInstallFinished = 'agent:install:finished';
export const EventInstallFailed = 'agent:install:failed';

function byName<T = void>(method: string, ...args: unknown[]) {
  return Call.ByName(method, ...args) as Promise<T>;
}

export function DetectAgents(): Promise<void> {
  return byName('main.App.DetectAgents');
}

export function InstallAgent(agentID: string): Promise<void> {
  return byName('main.App.InstallAgent', agentID);
}

export function IsAgentInstalling(agentID: string): Promise<boolean> {
  return byName('main.App.IsAgentInstalling', agentID);
}

export function GetPackageManager(): Promise<string> {
  return byName('main.App.GetPackageManager');
}

export function GetAgentInstallCommand(agentID: string): Promise<string> {
  return byName('main.App.GetAgentInstallCommand', agentID);
}

export function CheckAgentUpdates(): Promise<Record<string, string>> {
  return byName('main.App.CheckAgentUpdates');
}
