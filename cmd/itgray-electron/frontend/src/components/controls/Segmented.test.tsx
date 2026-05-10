import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Segmented } from './Segmented';

const opts = [
  { value: 'a', label: 'Alpha' },
  { value: 'b', label: 'Bravo' },
  { value: 'c', label: 'Charlie' },
] as const;

describe('Segmented', () => {
  it('renders all option labels', () => {
    render(<Segmented value="a" onChange={() => {}} options={opts} />);
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(screen.getByText('Bravo')).toBeInTheDocument();
    expect(screen.getByText('Charlie')).toBeInTheDocument();
  });

  it('marks the active option with aria-pressed=true', () => {
    render(<Segmented value="b" onChange={() => {}} options={opts} />);
    expect(screen.getByRole('button', { name: 'Alpha' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: 'Bravo' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('calls onChange with the clicked value', async () => {
    const onChange = vi.fn();
    render(<Segmented value="a" onChange={onChange} options={opts} />);
    await userEvent.click(screen.getByRole('button', { name: 'Charlie' }));
    expect(onChange).toHaveBeenCalledWith('c');
  });
});
