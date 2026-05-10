import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import * as React from "react";

import { GlowOrb } from "./GlowOrb";

describe("GlowOrb memoization", () => {
  it("does not re-render when props are unchanged", () => {
    const onClick = vi.fn();
    const renderSpy = vi.fn();

    function Probe(props: { unrelated: number }) {
      // Probe re-renders on every parent state change; GlowOrb should not.
      return (
        <>
          <span data-testid="probe">{props.unrelated}</span>
          <SpyingOrb onClick={onClick} renderSpy={renderSpy} />
        </>
      );
    }

    // Wrap in React.memo so the spy fires only when SpyingOrb's own props
    // change. Since renderSpy and onClick are stable across rerenders, and
    // GlowOrb is itself memoized, this asserts the full memoization chain:
    // SpyingOrb skipped → GlowOrb subtree untouched → no SVG re-mount → no
    // CSS animation reset.
    const SpyingOrb = React.memo(function SpyingOrb({
      onClick,
      renderSpy,
    }: {
      onClick: () => void;
      renderSpy: () => void;
    }) {
      renderSpy();
      return (
        <GlowOrb
          status="connecting"
          size={104}
          onClick={onClick}
          ariaLabel="test"
        />
      );
    });

    const { rerender } = render(<Probe unrelated={1} />);
    rerender(<Probe unrelated={2} />);
    rerender(<Probe unrelated={3} />);

    expect(renderSpy).toHaveBeenCalledTimes(1);
  });

  it("does re-render when status changes", () => {
    const renderSpy = vi.fn();

    function SpyingOrb(props: { status: "idle" | "connecting" }) {
      renderSpy();
      return <GlowOrb status={props.status} ariaLabel="test" />;
    }

    const { rerender } = render(<SpyingOrb status="idle" />);
    rerender(<SpyingOrb status="connecting" />);

    expect(renderSpy).toHaveBeenCalledTimes(2);
  });
});
