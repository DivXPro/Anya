import { useEffect, useState } from 'react';
import { Events } from '@wailsio/runtime';
import {
  AvailableUpdate,
  DownloadAndApplyUpdate,
  EventUpdateAvailable,
  EventUpdateProgress,
  EventUpdateApplying,
  EventUpdateError,
  type UpdateInfo,
} from '@/lib/update-api';

/**
 * Shared self-update state, driven by the backend update events plus a one-time
 * cached query on mount. The backend's daily background check can emit
 * `update:available` before the window (and React) mounts, so we also pull the
 * cached availability via AvailableUpdate() so a late subscriber still sees it.
 */
export function useAppUpdate() {
  const [available, setAvailable] = useState<UpdateInfo | null>(null);
  const [percent, setPercent] = useState<number | null>(null);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    AvailableUpdate()
      .then((info) => {
        if (!cancelled && info) setAvailable(info);
      })
      .catch(() => {});

    const offAvailable = Events.On(EventUpdateAvailable, (e) => {
      const info = e.data as UpdateInfo | null;
      if (info) setAvailable(info);
    });
    const offProgress = Events.On(EventUpdateProgress, (e) => {
      const d = e.data as { percent: number };
      setPercent(d?.percent ?? 0);
      setError('');
    });
    const offApplying = Events.On(EventUpdateApplying, () => {
      setApplying(true);
      setPercent(100);
    });
    const offError = Events.On(EventUpdateError, (e) => {
      const d = e.data as { message: string };
      setError(d?.message ?? 'error');
      setPercent(null);
      setApplying(false);
    });

    return () => {
      cancelled = true;
      offAvailable();
      offProgress();
      offApplying();
      offError();
    };
  }, []);

  const startUpdate = async () => {
    setError('');
    setPercent(0);
    try {
      await DownloadAndApplyUpdate();
    } catch (err) {
      setError(String(err));
      setPercent(null);
    }
  };

  // True while a download/apply is in flight.
  const inProgress = percent !== null || applying;

  return { available, percent, applying, error, inProgress, startUpdate };
}
