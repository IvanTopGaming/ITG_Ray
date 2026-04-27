import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusPill } from './StatusPill';

describe('StatusPill', () => {
  it('renders the running label', () => {
    render(<StatusPill status="running" />);
    expect(screen.getByText(/running/i)).toBeInTheDocument();
  });

  it('renders stopped, error, pending labels', () => {
    const { rerender } = render(<StatusPill status="stopped" />);
    expect(screen.getByText(/stopped/i)).toBeInTheDocument();
    rerender(<StatusPill status="error" />);
    expect(screen.getByText(/error/i)).toBeInTheDocument();
    rerender(<StatusPill status="pending" />);
    expect(screen.getByText(/pending/i)).toBeInTheDocument();
  });

  it('accepts custom label override', () => {
    render(<StatusPill status="running" label="Active" />);
    expect(screen.getByText('Active')).toBeInTheDocument();
  });
});
