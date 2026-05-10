import { describe, it, expect, vi } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConfirmButton } from './ConfirmButton';

describe('ConfirmButton', () => {
  it('does not fire onConfirm on first click', async () => {
    const onConfirm = vi.fn();
    render(<ConfirmButton onConfirm={onConfirm}>Restart</ConfirmButton>);
    await userEvent.click(screen.getByRole('button'));
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('shows confirm prompt after first click', async () => {
    render(<ConfirmButton onConfirm={() => {}}>Restart</ConfirmButton>);
    await userEvent.click(screen.getByRole('button'));
    expect(screen.getByRole('button')).toHaveTextContent(/click to confirm/i);
  });

  it('fires onConfirm on second click within window', async () => {
    const onConfirm = vi.fn();
    render(<ConfirmButton onConfirm={onConfirm}>Restart</ConfirmButton>);
    await userEvent.click(screen.getByRole('button'));
    await userEvent.click(screen.getByRole('button'));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('reverts to idle after timeout', async () => {
    vi.useFakeTimers();
    try {
      render(<ConfirmButton onConfirm={() => {}} timeoutMs={3000}>Restart</ConfirmButton>);
      // Manually fire a click via the DOM (no user-event so we don't hit
      // the user-event + fake-timers asyncWrapper deadlock).
      act(() => {
        screen.getByRole('button').click();
      });
      expect(screen.getByRole('button')).toHaveTextContent(/click to confirm/i);
      act(() => {
        vi.advanceTimersByTime(3100);
      });
      expect(screen.getByRole('button')).toHaveTextContent('Restart');
    } finally {
      vi.useRealTimers();
    }
  });
});
