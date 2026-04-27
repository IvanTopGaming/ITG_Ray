import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ScrollSpy } from './ScrollSpy';

describe('ScrollSpy', () => {
  const sections = [
    { id: 'general', label: 'General' },
    { id: 'connection', label: 'Connection' },
    { id: 'logs', label: 'Logs' },
  ];

  it('renders all chips', () => {
    render(<ScrollSpy sections={sections} active="general" onSelect={() => {}} />);
    expect(screen.getByRole('button', { name: 'General' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Connection' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Logs' })).toBeInTheDocument();
  });

  it('marks the active chip with aria-current="true"', () => {
    render(<ScrollSpy sections={sections} active="connection" onSelect={() => {}} />);
    expect(screen.getByRole('button', { name: 'Connection' })).toHaveAttribute('aria-current', 'true');
    expect(screen.getByRole('button', { name: 'General' })).toHaveAttribute('aria-current', 'false');
  });

  it('calls onSelect with chip id when clicked', async () => {
    const onSelect = vi.fn();
    render(<ScrollSpy sections={sections} active="general" onSelect={onSelect} />);
    await userEvent.click(screen.getByRole('button', { name: 'Logs' }));
    expect(onSelect).toHaveBeenCalledWith('logs');
  });
});
