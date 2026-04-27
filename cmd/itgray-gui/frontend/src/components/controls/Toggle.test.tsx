import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Toggle } from './Toggle';

describe('Toggle', () => {
  it('renders with role="switch" reflecting value', () => {
    render(<Toggle value={true} onChange={() => {}} />);
    const sw = screen.getByRole('switch');
    expect(sw).toHaveAttribute('aria-checked', 'true');
  });

  it('calls onChange with toggled value when clicked', async () => {
    const onChange = vi.fn();
    render(<Toggle value={false} onChange={onChange} />);
    await userEvent.click(screen.getByRole('switch'));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('does not call onChange when disabled', async () => {
    const onChange = vi.fn();
    render(<Toggle value={false} onChange={onChange} disabled />);
    await userEvent.click(screen.getByRole('switch'));
    expect(onChange).not.toHaveBeenCalled();
  });
});
