import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';

vi.mock('../../wailsjs/go/bindings/RunService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  Disconnect: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('../../wailsjs/go/bindings/SettingsService', () => ({
  Get: vi.fn().mockResolvedValue({
    general: {},
    network: {},
    notifications: {},
    debug: {},
  }),
  Update: vi.fn().mockResolvedValue({}),
}));

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
  WindowMinimise: vi.fn(),
  WindowToggleMaximise: vi.fn(),
  WindowIsMaximised: vi.fn().mockResolvedValue(false),
  Quit: vi.fn(),
}));

import { AppShell } from './AppShell';
import {
  snapshotFromConnectedPayload,
  clearConnectSnapshot,
  __resetForTests,
} from '@/lib/settings';
import * as RunService from '../../wailsjs/go/bindings/RunService';

const renderShell = async () => {
  const utils = render(
    <MemoryRouter>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<div data-testid="content">page</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
  // Drain the bootstrap GetSettings() promise chain (.then →
  // notifyListeners → useSyncExternalStore re-check → React commit) so
  // assertions run after the post-mount state has settled. The
  // beforeEach filter swallows the residual act() warning React still
  // emits on the React commit-phase tick (see comment above).
  await act(async () => {
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
  });
  return utils;
};

describe('AppShell reconnect pill', () => {
  // Filter the bootstrap-promise act() warning. After mount,
  // useSyncExternalStore subscribes to the settings store and the
  // GetSettings() mock resolves on a microtask whose React commit-phase
  // tick we cannot reliably contain inside act() in jsdom — multiple
  // setTimeout(0) drains and act()-wrapping the render itself were tried
  // without success on the tests that pre-populate lastConnectSnapshot.
  // Filtering only the exact "not wrapped in act" message keeps every
  // other console.error visible.
  beforeEach(() => {
    __resetForTests();
    clearConnectSnapshot();
    vi.clearAllMocks();
    const originalConsoleError = console.error;
    vi.spyOn(console, 'error').mockImplementation((...args: unknown[]) => {
      const msg = args[0];
      if (typeof msg === 'string' && msg.includes('not wrapped in act')) return;
      originalConsoleError(...args);
    });
  });
  afterEach(() => {
    clearConnectSnapshot();
    __resetForTests();
    (console.error as unknown as { mockRestore?: () => void }).mockRestore?.();
  });

  it('hides pill when no snapshot exists', async () => {
    await renderShell();
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('shows pill when snapshot exists and live state diverges', async () => {
    snapshotFromConnectedPayload({
      serverId: 's1',
      mode: 'sysproxy',
      network: {
        tunCidr: '198.18.0.1/15',
        tunMtu: 9999, // differs from default 1500 → diff selector returns true
        socksPort: 1080,
        httpPort: 8888,
        allowLan: false,
        ipv6Mode: 'prefer-v4',
        dns: { mode: 'auto' },
      },
    });
    await renderShell();
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /reconnect/i })).toBeInTheDocument();
  });

  it('clicking Reconnect calls Disconnect then Connect with snapshot args', async () => {
    snapshotFromConnectedPayload({
      serverId: 's1',
      mode: 'tun',
      network: {
        tunCidr: '198.18.0.1/15',
        tunMtu: 9999,
        socksPort: 1080,
        httpPort: 8888,
        allowLan: false,
        ipv6Mode: 'prefer-v4',
        dns: { mode: 'auto' },
      },
    });
    await renderShell();
    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /reconnect/i }));
    });
    expect(RunService.Disconnect).toHaveBeenCalledOnce();
    expect(RunService.Connect).toHaveBeenCalledWith('s1', 'tun');
  });
});
