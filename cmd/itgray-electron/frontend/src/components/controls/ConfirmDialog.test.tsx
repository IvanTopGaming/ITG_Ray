import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConfirmDialog } from './ConfirmDialog';

describe('ConfirmDialog', () => {
  it('does not render when open=false', () => {
    render(
      <ConfirmDialog
        open={false}
        onClose={() => {}}
        title="Reinstall?"
        description="d"
        onConfirm={() => {}}
      />,
    );
    expect(screen.queryByText('Reinstall?')).not.toBeInTheDocument();
  });

  it('renders title, description, and both buttons when open', () => {
    render(
      <ConfirmDialog
        open
        onClose={() => {}}
        title="Reinstall helper service?"
        description="This will request elevated privileges."
        confirmLabel="Reinstall"
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByText('Reinstall helper service?')).toBeInTheDocument();
    expect(screen.getByText('This will request elevated privileges.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reinstall' })).toBeInTheDocument();
  });

  it('calls onConfirm and onClose when confirm is clicked', async () => {
    const onConfirm = vi.fn();
    const onClose = vi.fn();
    render(
      <ConfirmDialog open onClose={onClose} title="t" description="d" confirmLabel="OK" onConfirm={onConfirm} />,
    );
    await userEvent.click(screen.getByRole('button', { name: 'OK' }));
    expect(onConfirm).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('calls onClose on Escape key', async () => {
    const onClose = vi.fn();
    render(<ConfirmDialog open onClose={onClose} title="t" description="d" onConfirm={() => {}} />);
    await userEvent.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalled();
  });
});
