import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import { Reveal } from './Reveal';

describe('Reveal', () => {
  it('renders children when show=true', () => {
    render(<Reveal show={true}><div>visible content</div></Reveal>);
    expect(screen.getByText('visible content')).toBeInTheDocument();
  });

  it('does not render children when show=false', () => {
    render(<Reveal show={false}><div>hidden content</div></Reveal>);
    expect(screen.queryByText('hidden content')).not.toBeInTheDocument();
  });

  it('renders different children when show flips', () => {
    const { rerender } = render(<Reveal show={false}><div>content</div></Reveal>);
    expect(screen.queryByText('content')).not.toBeInTheDocument();
    rerender(<Reveal show={true}><div>content</div></Reveal>);
    expect(screen.getByText('content')).toBeInTheDocument();
  });
});
