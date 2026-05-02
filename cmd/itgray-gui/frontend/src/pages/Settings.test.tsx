import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useEffect, useState } from 'react';

// ──────────────────────────────────────────────────────────────────────
//  Numeric-input draft pattern integration test
// ──────────────────────────────────────────────────────────────────────
//
// The Settings page previously bound numeric <input>s directly to the
// store value: every keystroke pushed `Number(e.target.value)` through
// update(), the 200ms debounce hit the backend's range validator, the
// out-of-range intermediate (e.g. "150" while typing "1500" → "1400")
// was silently rejected, and the EventSettings echo snapped the input
// back to the last persisted value. That made the field uneditable.
//
// The fix decouples user typing from store flushing via a local draft
// string + an effect that mirrors the canonical store value back into
// the draft on outside changes. The store only sees commits when the
// draft parses to a validator-clean number.
//
// This test renders a stand-in component that uses the identical
// pattern (same shape as the inputs in Settings.tsx) and asserts:
//   1. Every keystroke is reflected in the visible input value.
//   2. update() is called only on commits to validator-clean values.
//   3. An external change to the canonical value mirrors back into
//      the draft (proves the useEffect sync path works).
//   4. Blur with an invalid draft reverts the draft to the canonical
//      value (proves user always sees what backend persisted).
//
// We deliberately don't render the full Settings page — it imports
// Wails bindings, framer-motion, and a ScrollSpy that needs observer
// polyfills in jsdom. The draft logic is the regression surface; an
// isolated test of that logic captures the bug directly.

function isMtuValid(value: number): boolean {
  return Number.isFinite(value) && value >= 576 && value <= 9000;
}

type MtuFieldProps = {
  storeValue: number;
  onCommit: (n: number) => void;
};

function MtuField({ storeValue, onCommit }: MtuFieldProps) {
  const [draft, setDraft] = useState(String(storeValue));
  useEffect(() => {
    setDraft(String(storeValue));
  }, [storeValue]);
  return (
    <input
      data-testid="mtu"
      type="text"
      inputMode="numeric"
      value={draft}
      onChange={(e) => {
        setDraft(e.target.value);
        const n = Number(e.target.value);
        if (Number.isFinite(n) && isMtuValid(n)) {
          onCommit(n);
        }
      }}
      onBlur={() => {
        const n = Number(draft);
        if (!Number.isFinite(n) || !isMtuValid(n)) {
          setDraft(String(storeValue));
        }
      }}
    />
  );
}

describe('numeric draft pattern (MTU input)', () => {
  it('lets the user clear and retype freely without snap-back', async () => {
    const user = userEvent.setup();
    const onCommit = vi.fn();
    render(<MtuField storeValue={1500} onCommit={onCommit} />);

    const input = screen.getByTestId('mtu') as HTMLInputElement;
    expect(input.value).toBe('1500');

    // The initial 1500 is the canonical value already; no commit
    // happens until the user changes it to a different valid one.
    await user.click(input);

    // Backspace four times: "1500" → "150" → "15" → "1" → ""
    await user.keyboard('{End}{Backspace}');
    expect(input.value).toBe('150');
    await user.keyboard('{Backspace}');
    expect(input.value).toBe('15');
    await user.keyboard('{Backspace}');
    expect(input.value).toBe('1');
    await user.keyboard('{Backspace}');
    expect(input.value).toBe('');

    // None of the intermediates passed isMtuValid (150, 15, 1, ""),
    // so no commit fired. The 1500 commit did not fire either —
    // the input started at 1500, so re-committing it would be a noop
    // anyway, but more importantly the change handler only commits
    // values that PARSE AND VALIDATE.
    expect(onCommit).not.toHaveBeenCalled();

    // Now type "1400" — first three keystrokes ("1", "14", "140") fail
    // isMtuValid; the fourth ("1400") passes and commits exactly once.
    await user.keyboard('1');
    expect(input.value).toBe('1');
    await user.keyboard('4');
    expect(input.value).toBe('14');
    await user.keyboard('0');
    expect(input.value).toBe('140');
    expect(onCommit).not.toHaveBeenCalled();
    await user.keyboard('0');
    expect(input.value).toBe('1400');
    expect(onCommit).toHaveBeenCalledTimes(1);
    expect(onCommit).toHaveBeenLastCalledWith(1400);
  });

  it('mirrors external store-value changes back into the draft', async () => {
    const onCommit = vi.fn();
    const { rerender } = render(<MtuField storeValue={1500} onCommit={onCommit} />);
    const input = screen.getByTestId('mtu') as HTMLInputElement;
    expect(input.value).toBe('1500');

    // Backend pushed a new value (e.g. CLI edit, post-flush refetch).
    rerender(<MtuField storeValue={1400} onCommit={onCommit} />);
    expect(input.value).toBe('1400');
  });

  it('reverts to the canonical value on blur if the draft is invalid', async () => {
    const user = userEvent.setup();
    const onCommit = vi.fn();
    render(<MtuField storeValue={1500} onCommit={onCommit} />);
    const input = screen.getByTestId('mtu') as HTMLInputElement;

    await user.click(input);
    await user.tripleClick(input);
    await user.keyboard('{Backspace}');
    expect(input.value).toBe('');
    expect(onCommit).not.toHaveBeenCalled();

    // Blur with empty draft — should revert to canonical "1500".
    await user.tab();
    expect(input.value).toBe('1500');
    expect(onCommit).not.toHaveBeenCalled();
  });
});
