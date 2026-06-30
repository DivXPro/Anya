import { Call } from '@wailsio/runtime';

export const EventUpdateAvailable = 'update:available';
export const EventUpdateProgress = 'update:progress';
export const EventUpdateApplying = 'update:applying';
export const EventUpdateError = 'update:error';

export interface UpdateInfo {
  version: string;
  notes: string;
  assetName: string;
  assetURL: string;
  size: number;
  checksumsURL: string;
  signatureURL: string;
}

export function CurrentVersion(): Promise<string> {
  return Call.ByName('main.App.CurrentVersion');
}

export function CheckForUpdate(): Promise<UpdateInfo | null> {
  return Call.ByName('main.App.CheckForUpdate');
}

export function AvailableUpdate(): Promise<UpdateInfo | null> {
  return Call.ByName('main.App.AvailableUpdate');
}

export function DownloadAndApplyUpdate(): Promise<void> {
  return Call.ByName('main.App.DownloadAndApplyUpdate');
}
