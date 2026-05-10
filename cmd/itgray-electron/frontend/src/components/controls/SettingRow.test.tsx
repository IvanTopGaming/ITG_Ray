import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SettingRow } from './SettingRow';

describe('SettingRow', () => {
  it('renders label, hint, and child control', () => {
    render(
      <SettingRow label="My setting" hint="A description">
        <button>Action</button>
      </SettingRow>,
    );
    expect(screen.getByText('My setting')).toBeInTheDocument();
    expect(screen.getByText('A description')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Action' })).toBeInTheDocument();
  });

  it('omits hint element when hint prop is undefined', () => {
    render(
      <SettingRow label="Bare">
        <button>X</button>
      </SettingRow>,
    );
    expect(screen.queryByText(/description/i)).not.toBeInTheDocument();
  });

  it('renders in stacked mode when stacked prop is true', () => {
    const { container } = render(
      <SettingRow label="Stacked" stacked>
        <input />
      </SettingRow>,
    );
    // Stacked mode uses flex-col on the wrapper. Verify by class presence.
    expect(container.firstChild).toHaveClass('flex-col');
  });
});
