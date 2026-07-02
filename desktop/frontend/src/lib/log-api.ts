import { Call } from '@wailsio/runtime';

export interface LogInfo {
  path: string;
  size: number;
  modified_at: string;
}

export function GetLogInfo(): Promise<LogInfo> {
  return Call.ByName('main.App.GetLogInfo');
}

export function ReadLogTail(maxBytes: number): Promise<string> {
  return Call.ByName('main.App.ReadLogTail', maxBytes);
}
