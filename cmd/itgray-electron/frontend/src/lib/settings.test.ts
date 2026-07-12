import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';

// Mocks must be set up BEFORE importing the SUT.
const getMock = vi.fn();
const updateMock = vi.fn();
const eventsOnMock = vi.fn();
const eventsOffMock = vi.fn();

vi.mock('@/lib/itg/SettingsService', () => ({
  Get: (...args: unknown[]) => getMock(...args),
  Update: (...args: unknown[]) => updateMock(...args),
}));
vi.mock('@/lib/itg/runtime', () => ({
  EventsOn: (...args: unknown[]) => eventsOnMock(...args),
  EventsOff: (...args: unknown[]) => eventsOffMock(...args),
}));

import {
  useSettings,
  useReconnectNeeded,
  flushSettings,
  __resetForTests,
  DEFAULTS,
  snapshotFromConnectedPayload,
  seedConnectSnapshotFromSnapshot,
  clearConnectSnapshot,
  getConnectSnapshot,
  markActiveServerEdited,
  clearActiveServerEdited,
  dismissNetworkDiff,
  setDesiredServer,
  clearDesiredServer,
  getDesiredServer,
  setCurrentRulesSignature,
  setRulesDismissed,
} from './settings';

beforeEach(() => {
  vi.useFakeTimers();
  // Pretend we are in a Wails-injected window.
  (window as any).go = {};
  getMock.mockReset();
  updateMock.mockReset();
  eventsOnMock.mockReset();
  eventsOffMock.mockReset();
  __resetForTests();
});

afterEach(() => {
  vi.useRealTimers();
  delete (window as any).go;
});

describe('useSettings (mocked SettingsService)', () => {
  it('returns DEFAULTS synchronously, then merges Get() result', async () => {
    getMock.mockResolvedValue({
      general: { language: 'ru' },
      network: { defaultMode: 'sysproxy' },
      notifications: {},
      debug: {},
    });

    const { result } = renderHook(() => useSettings());

    expect(result.current[0]).toEqual(DEFAULTS);

    await act(async () => {
      await Promise.resolve(); // flush microtasks
      await Promise.resolve();
    });
    expect(result.current[0].language).toBe('ru');
    expect(result.current[0].defaultMode).toBe('sysproxy');
    expect(getMock).toHaveBeenCalledTimes(1);
  });

  it('update() merges optimistically and fires Update RPC after 200ms', async () => {
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => {
      result.current[1]({ autostart: true });
    });
    expect(result.current[0].autostart).toBe(true);
    expect(updateMock).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(199);
    });
    expect(updateMock).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(1);
      await Promise.resolve();
    });
    expect(updateMock).toHaveBeenCalledWith('general', { autostart: true });
  });

  it('coalesces multiple updates within debounce window into one section RPC', async () => {
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ socksPort: 1080 }));
    act(() => { vi.advanceTimersByTime(50); });
    act(() => result.current[1]({ socksPort: 1081 }));
    act(() => { vi.advanceTimersByTime(70); });
    act(() => result.current[1]({ allowLan: true }));

    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
    });

    expect(updateMock).toHaveBeenCalledTimes(1);
    expect(updateMock).toHaveBeenCalledWith('network', { socksPort: 1081, allowLan: true });
  });

  it('multi-section patch fires one RPC per section in parallel', async () => {
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ language: 'en', socksPort: 1080, onConnected: false }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(updateMock).toHaveBeenCalledTimes(3);
    const calls = updateMock.mock.calls.map(([section, patch]) => [section, patch]).sort(
      (a, b) => String(a[0]).localeCompare(String(b[0])),
    );
    expect(calls).toEqual([
      ['general', { language: 'en' }],
      ['network', { socksPort: 1080 }],
      ['notifications', { onConnected: false }],
    ]);
  });

  it('flushSettings() forces immediate RPC', async () => {
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ autostart: true }));
    expect(updateMock).not.toHaveBeenCalled();

    await act(async () => {
      flushSettings();
      await Promise.resolve();
    });
    expect(updateMock).toHaveBeenCalledWith('general', { autostart: true });
  });

  it('Update rejection is logged; post-flush Get rolls state back to disk truth', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    // Disk explicitly has autostart=false. ConfigStore.toView always
    // emits the full struct, so the post-flush Get sees it and the
    // adapter includes it in the merge.
    getMock.mockResolvedValue({
      general: { autostart: false },
      network: {},
      notifications: {},
      debug: {},
    });
    updateMock.mockRejectedValue(new Error('disk full'));

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ autostart: true }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });

    // Disk never received the write (Update rejected). Post-flush Get
    // re-syncs from disk: autostart returns to its disk value (false).
    // This implicitly rolls back optimistic state on failed writes.
    expect(result.current[0].autostart).toBe(false);
    expect(warnSpy).toHaveBeenCalledWith(
      'SettingsService.Update failed',
      expect.objectContaining({ message: 'disk full' }),
    );
    warnSpy.mockRestore();
  });

  it('post-flush Get picks up cross-process disk drift (multi-writer)', async () => {
    // Bootstrap: disk has language=en, autostart=false.
    getMock.mockResolvedValueOnce({ general: {}, network: {}, notifications: {}, debug: {} });
    // Post-flush Get: another writer (CLI) changed language=ru on disk
    // during our flush window.
    getMock.mockResolvedValueOnce({
      general: { language: 'ru' },
      network: {},
      notifications: {},
      debug: {},
    });
    updateMock.mockResolvedValue({});

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ autostart: true }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(updateMock).toHaveBeenCalledTimes(1);
    expect(getMock).toHaveBeenCalledTimes(2); // bootstrap + post-flush
    expect(result.current[0].language).toBe('ru'); // disk drift picked up
    expect(result.current[0].autostart).toBe(true); // our write persisted
  });

  it('flushNow leaves the in-flight counter at zero on empty section map', async () => {
    // Regression: an empty section map (e.g. patch with only undefined
    // keys) used to leave the inFlightUpdate flag stuck at true,
    // permanently muting backend EventSettings refresh.
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    // Force a pending patch with no mappable backend keys by passing
    // only undefined values. update() still records pendingPatch but
    // frontendToBackend returns an empty Map.
    act(() => result.current[1]({ autostart: undefined as unknown as boolean }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
    });

    // Now an EventsOn echo should still trigger a fresh Get, proving
    // the in-flight gate didn't get stuck.
    const handler = eventsOnMock.mock.calls[0][1] as () => void;
    getMock.mockResolvedValueOnce({
      general: { language: 'ru' },
      network: {},
      notifications: {},
      debug: {},
    });
    await act(async () => {
      handler();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(result.current[0].language).toBe('ru');
  });

  it('retries Get on remount after initial bootstrap failure', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    getMock.mockRejectedValueOnce(new Error('rpc not ready'));

    const first = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    expect(warnSpy).toHaveBeenCalledWith('SettingsService.Get failed', expect.any(Error));
    expect(getMock).toHaveBeenCalledTimes(1);
    first.unmount();

    // Bootstrap was a one-shot module-level guard; without retry-on-fail
    // it would never fire again. Verify a successful remount picks up
    // disk truth.
    getMock.mockResolvedValueOnce({
      general: { language: 'ru' },
      network: {},
      notifications: {},
      debug: {},
    });
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    expect(getMock).toHaveBeenCalledTimes(2);
    expect(result.current[0].language).toBe('ru');
    warnSpy.mockRestore();
  });

  it('EventsOn("settings:changed") triggers Get + state merge', async () => {
    getMock.mockResolvedValueOnce({ general: {}, network: {}, notifications: {}, debug: {} });
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    expect(eventsOnMock).toHaveBeenCalledWith('settings:changed', expect.any(Function));
    const handler = eventsOnMock.mock.calls[0][1] as () => void;

    getMock.mockResolvedValueOnce({
      general: { language: 'ru' },
      network: {},
      notifications: {},
      debug: {},
    });
    await act(async () => {
      handler();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(result.current[0].language).toBe('ru');
  });
});

describe('reconnect snapshot', () => {
  beforeEach(() => {
    // useReconnectNeeded (rendered by the setDesiredServer cases below)
    // subscribes to the store, whose first listener triggers
    // loadFromBackend() -> Get(). Without a resolved mock the .then()
    // chain throws on undefined.
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    clearConnectSnapshot();
  });

  it('snapshot is null at boot', () => {
    expect(getConnectSnapshot()).toBeNull();
  });

  it('connected event populates snapshot from payload', () => {
    snapshotFromConnectedPayload({
      serverId: 's1',
      mode: 'sysproxy',
      network: {
        tunCidr: '198.18.0.1/15',
        tunMtu: 1500,
        socksPort: 1090,
        httpPort: 8889,
        allowLan: false,
        ipv6Mode: 'prefer-v4',
        dns: { mode: 'auto', servers: [] },
      },
    });
    const snap = getConnectSnapshot();
    expect(snap).not.toBeNull();
    expect(snap?.serverId).toBe('s1');
    expect(snap?.mode).toBe('sysproxy');
    expect(snap?.network.socksPort).toBe(1090);
    expect(snap?.network.httpPort).toBe(8889);
  });

  it('disconnected event clears snapshot via clearConnectSnapshot', () => {
    snapshotFromConnectedPayload({
      serverId: 's1',
      mode: 'tun',
      network: {
        tunCidr: '198.18.0.1/15',
        tunMtu: 1500,
        socksPort: 1080,
        httpPort: 8888,
        allowLan: false,
        ipv6Mode: 'prefer-v4',
        dns: { mode: 'auto' },
      },
    });
    expect(getConnectSnapshot()).not.toBeNull();
    clearConnectSnapshot();
    expect(getConnectSnapshot()).toBeNull();
  });

  it('snapshotFromConnectedPayload(undefined network) is a no-op', () => {
    snapshotFromConnectedPayload({ serverId: 's1', mode: 'tun' });
    expect(getConnectSnapshot()).toBeNull();
  });

  const connectedPull = {
    status: 'connected',
    currentServer: { id: 's7' },
    mode: 'tun',
    settings: {
      network: {
        tunCidr: '198.18.0.1/15',
        tunMtu: 1500,
        socksPort: 1080,
        httpPort: 8888,
        allowLan: true,
        ipv6Mode: 'prefer-v4',
        dns: { mode: 'auto', servers: [] },
      },
    },
  };

  it('seedConnectSnapshotFromSnapshot rebuilds the snapshot from a connected pull (adopt/reopen path)', () => {
    seedConnectSnapshotFromSnapshot(connectedPull);
    const snap = getConnectSnapshot();
    expect(snap).not.toBeNull();
    expect(snap?.serverId).toBe('s7');
    expect(snap?.mode).toBe('tun');
    expect(snap?.network.allowLan).toBe(true);
    expect(snap?.network.socksPort).toBe(1080);
  });

  it('seedConnectSnapshotFromSnapshot is a no-op when not connected', () => {
    seedConnectSnapshotFromSnapshot({ ...connectedPull, status: 'idle' });
    expect(getConnectSnapshot()).toBeNull();
  });

  it('seedConnectSnapshotFromSnapshot never clobbers a live event-sourced snapshot', () => {
    snapshotFromConnectedPayload({
      serverId: 'live',
      mode: 'sysproxy',
      network: {
        tunCidr: '198.18.0.1/15',
        tunMtu: 1500,
        socksPort: 1090,
        httpPort: 8889,
        allowLan: false,
        ipv6Mode: 'prefer-v4',
        dns: { mode: 'auto' },
      },
    });
    seedConnectSnapshotFromSnapshot(connectedPull);
    expect(getConnectSnapshot()?.serverId).toBe('live');
  });

  const serverDimNetwork = {
    tunCidr: DEFAULTS.tunCidr,
    tunMtu: DEFAULTS.tunMtu,
    socksPort: DEFAULTS.socksPort,
    httpPort: DEFAULTS.httpPort,
    allowLan: DEFAULTS.allowLan,
    ipv6Mode: DEFAULTS.ipv6Mode,
    dns: { mode: 'auto' },
  };

  it('setDesiredServer arms the toast when it differs from the connected server', () => {
    snapshotFromConnectedPayload({ serverId: 'A', mode: 'tun', network: serverDimNetwork });
    setDesiredServer('B');
    expect(getDesiredServer()).toBe('B');
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(true);
  });

  it('setDesiredServer(connected id) clears the pending pick (revert)', () => {
    snapshotFromConnectedPayload({ serverId: 'A', mode: 'tun', network: serverDimNetwork });
    setDesiredServer('B');
    setDesiredServer('A');
    expect(getDesiredServer()).toBeNull();
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(false);
  });

  it('clearDesiredServer removes the pending pick', () => {
    snapshotFromConnectedPayload({ serverId: 'A', mode: 'tun', network: serverDimNetwork });
    setDesiredServer('B');
    clearDesiredServer();
    expect(getDesiredServer()).toBeNull();
  });

  it('rules diff arms toast after connect when signature changes', () => {
    setCurrentRulesSignature('sig-1');
    snapshotFromConnectedPayload({ serverId: 'A', mode: 'tun', network: serverDimNetwork });
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(false);
    setCurrentRulesSignature('sig-2');
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(true);
  });

  it('reverting rules to the connected signature hides the toast', () => {
    setCurrentRulesSignature('sig-1');
    snapshotFromConnectedPayload({ serverId: 'A', mode: 'tun', network: serverDimNetwork });
    setCurrentRulesSignature('sig-2');
    setCurrentRulesSignature('sig-1');
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(false);
  });

  it('does not arm on the first rules-signature push after a snapshot with the empty (unbooted) signature; a later real change still arms', () => {
    snapshotFromConnectedPayload({ serverId: 'A', mode: 'tun', network: serverDimNetwork });
    expect(getConnectSnapshot()?.rulesSignature).toBe('');
    setCurrentRulesSignature('{"defaultAction":"proxy","groups":[]}');
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(false);
    setCurrentRulesSignature('{"defaultAction":"direct","groups":[]}');
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(true);
  });

  it('setRulesDismissed hides a rules diff until the next change re-arms', () => {
    setCurrentRulesSignature('sig-1');
    snapshotFromConnectedPayload({ serverId: 'A', mode: 'tun', network: serverDimNetwork });
    setCurrentRulesSignature('sig-2');
    setRulesDismissed();
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(false);
    setCurrentRulesSignature('sig-3');
    expect(renderHook(() => useReconnectNeeded()).result.current).toBe(true);
  });
});

describe('reconnectNeeded union & auto-clear', () => {
  beforeEach(() => {
    __resetForTests();
    // Bootstrap-Get default — useReconnectNeeded subscribes to the same
    // store, so the first listener triggers loadFromBackend(). Without a
    // resolved Get mock the .then() chain throws on undefined.
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
  });

  it('reflects activeServerEdited even without a snapshot', async () => {
    const { result } = renderHook(() => useReconnectNeeded());
    await act(async () => { await Promise.resolve(); });
    expect(result.current).toBe(false);
    act(() => {
      markActiveServerEdited();
    });
    expect(result.current).toBe(true);
  });

  it('markActiveServerEdited is idempotent (second call is no-op)', async () => {
    const { result } = renderHook(() => useReconnectNeeded());
    await act(async () => { await Promise.resolve(); });
    act(() => {
      markActiveServerEdited();
    });
    expect(result.current).toBe(true);
    // A second call must not throw or flip the state — it's a no-op
    // because activeServerEdited is already true.
    act(() => {
      markActiveServerEdited();
    });
    expect(result.current).toBe(true);
  });

  it('dismissNetworkDiff does not affect activeServerEdited signal', async () => {
    const { result } = renderHook(() => useReconnectNeeded());
    await act(async () => { await Promise.resolve(); });
    act(() => {
      markActiveServerEdited();
      dismissNetworkDiff();
    });
    // Active-edit signal still active even after dismissNetworkDiff.
    expect(result.current).toBe(true);
    act(() => {
      clearActiveServerEdited();
    });
    expect(result.current).toBe(false);
  });

  it('dismissNetworkDiff suppresses the network-diff signal until next edit', async () => {
    const settings = renderHook(() => useSettings());
    const reconnect = renderHook(() => useReconnectNeeded());
    await act(async () => {
      await Promise.resolve();
    });

    // Snapshot reflects the current state — diff should be false.
    act(() => {
      snapshotFromConnectedPayload({
        serverId: 's1',
        mode: 'tun',
        network: {
          tunCidr: DEFAULTS.tunCidr,
          tunMtu: DEFAULTS.tunMtu,
          socksPort: DEFAULTS.socksPort,
          httpPort: DEFAULTS.httpPort,
          allowLan: DEFAULTS.allowLan,
          ipv6Mode: DEFAULTS.ipv6Mode,
          dns: { mode: 'auto' },
        },
      });
    });
    expect(reconnect.result.current).toBe(false);

    // Edit a network field — diff is true, toast is armed.
    act(() => {
      settings.result.current[1]({ tunMtu: 1400 });
    });
    expect(reconnect.result.current).toBe(true);

    // Dismiss — toast is suppressed even though the diff still exists.
    act(() => {
      dismissNetworkDiff();
    });
    expect(reconnect.result.current).toBe(false);

    // Another edit — dismiss flag clears, toast re-arms.
    act(() => {
      settings.result.current[1]({ tunMtu: 1300 });
    });
    expect(reconnect.result.current).toBe(true);
  });

  it('snapshotFromConnectedPayload re-clears networkDiffDismissed', async () => {
    const { result } = renderHook(() => useReconnectNeeded());
    await act(async () => { await Promise.resolve(); });
    // Seed a snapshot, dismiss is set, then a fresh connect rebuilds the
    // snapshot — dismiss must reset so future edits arm the toast.
    act(() => {
      snapshotFromConnectedPayload({
        serverId: 's1',
        mode: 'tun',
        network: {
          tunCidr: DEFAULTS.tunCidr,
          tunMtu: DEFAULTS.tunMtu,
          socksPort: DEFAULTS.socksPort,
          httpPort: DEFAULTS.httpPort,
          allowLan: DEFAULTS.allowLan,
          ipv6Mode: DEFAULTS.ipv6Mode,
          dns: { mode: 'auto' },
        },
      });
      dismissNetworkDiff();
    });
    expect(result.current).toBe(false);
    // New connect snapshot rebuilds — dismiss should reset implicitly.
    act(() => {
      snapshotFromConnectedPayload({
        serverId: 's2',
        mode: 'tun',
        network: {
          tunCidr: '10.0.0.1/15', // different snapshot — current state diverges
          tunMtu: DEFAULTS.tunMtu,
          socksPort: DEFAULTS.socksPort,
          httpPort: DEFAULTS.httpPort,
          allowLan: DEFAULTS.allowLan,
          ipv6Mode: DEFAULTS.ipv6Mode,
          dns: { mode: 'auto' },
        },
      });
    });
    // currentState.tunCidr is DEFAULTS.tunCidr ('198.18.0.1/15') which
    // != snapshot '10.0.0.1/15', so the diff flips true and the toast
    // is no longer suppressed.
    expect(result.current).toBe(true);
  });

  it('__resetForTests zeroes activeServerEdited', async () => {
    const { result, rerender } = renderHook(() => useReconnectNeeded());
    await act(async () => { await Promise.resolve(); });
    act(() => {
      markActiveServerEdited();
    });
    expect(result.current).toBe(true);
    act(() => {
      __resetForTests();
    });
    rerender();
    expect(result.current).toBe(false);
  });

});
