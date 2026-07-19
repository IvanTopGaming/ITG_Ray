import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';

const { dashReconnectMock, revertBaselineMock } = vi.hoisted(() => ({
  dashReconnectMock: vi.fn(),
  revertBaselineMock: vi.fn(),
}));

vi.mock('@/lib/rulesStore', () => ({
  rulesRevertToBaseline: () => revertBaselineMock(),
}));

vi.mock('@/lib/dashStore', () => ({
  dashReconnect: (id: string, mode: string) => dashReconnectMock(id, mode),
  useDash: () => ({ status: 'idle', lastError: null }),
  effectiveStatus: () => 'idle',
  getDashState: () => ({ currentServer: { id: 'live' }, mode: 'tun' }),
}));

vi.mock('@/lib/itg/SettingsService', () => ({
  Get: vi.fn().mockResolvedValue({
    general: {},
    network: {},
    notifications: {},
    debug: {},
  }),
  Update: vi.fn().mockResolvedValue({}),
}));

vi.mock('@/lib/itg/runtime', () => ({
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
  markActiveServerEdited,
  setCurrentRulesSignature,
  setDesiredServer,
  getDesiredServer,
  __resetForTests,
} from '@/lib/settings';

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
    dashReconnectMock.mockReset();
    dashReconnectMock.mockResolvedValue(undefined);
    revertBaselineMock.mockReset();
    revertBaselineMock.mockResolvedValue(false);
    // AppShell's deeplink effects reach for the preload bridge on mount;
    // jsdom has no preload script, so stand one in.
    (window as any).itg = {
      on: vi.fn(() => vi.fn()),
      app: { getPendingDeeplink: vi.fn().mockResolvedValue(null) },
    };
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

  it('clicking Reconnect routes through dashReconnect with snapshot args', async () => {
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
    expect(dashReconnectMock).toHaveBeenCalledWith('s1', 'tun');
  });

  it('Reconnect rejection is swallowed (dashStore surfaces error via lastError)', async () => {
    dashReconnectMock.mockRejectedValueOnce(new Error('helper down'));
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
    // Click must not throw uncaught — AppShell's try/catch swallows after
    // dashStore has already populated lastError.
    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /reconnect/i }));
    });
    expect(dashReconnectMock).toHaveBeenCalledOnce();
  });

  it('renders dismiss button on the toast (key differentiator from old pill)', async () => {
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
    expect(screen.getByRole('button', { name: /dismiss/i })).toBeInTheDocument();
  });

  it('shows toast when active server has been edited (no snapshot needed)', async () => {
    markActiveServerEdited();
    await renderShell();
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('dismiss clears active-edit signal and hides toast', async () => {
    markActiveServerEdited();
    const user = userEvent.setup();
    await renderShell();
    expect(screen.getByRole('status')).toBeInTheDocument();
    await act(async () => {
      await user.click(screen.getByRole('button', { name: /dismiss/i }));
    });
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
  });

  it('dismiss hides a toast armed by a rules edit (signature diff)', async () => {
    setCurrentRulesSignature('sig-1');
    snapshotFromConnectedPayload({
      serverId: 'A', mode: 'tun',
      network: { tunCidr: '198.18.0.1/15', tunMtu: 1500, socksPort: 1080, httpPort: 8888, allowLan: false, ipv6Mode: 'prefer-v4', dns: { mode: 'auto' } },
    });
    setCurrentRulesSignature('sig-2');
    const user = userEvent.setup();
    await renderShell();
    expect(screen.getByRole('status')).toBeInTheDocument();
    await act(async () => { await user.click(screen.getByRole('button', { name: /dismiss/i })); });
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
  });

  it('dismiss rolls back the rule edits it declines to apply', async () => {
    setCurrentRulesSignature('sig-1');
    snapshotFromConnectedPayload({
      serverId: 'A', mode: 'tun',
      network: { tunCidr: '198.18.0.1/15', tunMtu: 1500, socksPort: 1080, httpPort: 8888, allowLan: false, ipv6Mode: 'prefer-v4', dns: { mode: 'auto' } },
    });
    setCurrentRulesSignature('sig-2');
    const user = userEvent.setup();
    await renderShell();
    await act(async () => { await user.click(screen.getByRole('button', { name: /dismiss/i })); });
    expect(revertBaselineMock).toHaveBeenCalledTimes(1);
  });

  it('Reconnect keeps the rule edits it applies', async () => {
    setCurrentRulesSignature('sig-1');
    snapshotFromConnectedPayload({
      serverId: 'A', mode: 'tun',
      network: { tunCidr: '198.18.0.1/15', tunMtu: 1500, socksPort: 1080, httpPort: 8888, allowLan: false, ipv6Mode: 'prefer-v4', dns: { mode: 'auto' } },
    });
    setCurrentRulesSignature('sig-2');
    const user = userEvent.setup();
    await renderShell();
    await act(async () => { await user.click(screen.getByRole('button', { name: /reconnect/i })); });
    expect(revertBaselineMock).not.toHaveBeenCalled();
  });

  it('Reconnect hides toast armed by an active-server edit', async () => {
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
    markActiveServerEdited();
    const user = userEvent.setup();
    await renderShell();
    expect(screen.getByRole('status')).toBeInTheDocument();
    await act(async () => {
      await user.click(screen.getByRole('button', { name: /reconnect/i }));
    });
    expect(dashReconnectMock).toHaveBeenCalledWith('s1', 'tun');
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
  });

  it('Reconnect targets the pending desired server over the snapshot', async () => {
    snapshotFromConnectedPayload({
      serverId: 'A', mode: 'tun',
      network: { tunCidr: '198.18.0.1/15', tunMtu: 1500, socksPort: 1080, httpPort: 8888, allowLan: false, ipv6Mode: 'prefer-v4', dns: { mode: 'auto' } },
    });
    setDesiredServer('B');
    const user = userEvent.setup();
    await renderShell();
    await act(async () => {
      await user.click(screen.getByRole('button', { name: /reconnect/i }));
    });
    expect(dashReconnectMock).toHaveBeenCalledWith('B', 'tun');
  });

  it('Dismiss reverts a pending server pick and hides the toast', async () => {
    snapshotFromConnectedPayload({
      serverId: 'A', mode: 'tun',
      network: { tunCidr: '198.18.0.1/15', tunMtu: 1500, socksPort: 1080, httpPort: 8888, allowLan: false, ipv6Mode: 'prefer-v4', dns: { mode: 'auto' } },
    });
    setDesiredServer('B');
    const user = userEvent.setup();
    await renderShell();
    expect(screen.getByRole('status')).toBeInTheDocument();
    await act(async () => {
      await user.click(screen.getByRole('button', { name: /dismiss/i }));
    });
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
    expect(getDesiredServer()).toBeNull();
  });
});
