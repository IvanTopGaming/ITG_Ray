import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ReconnectToast } from "./ReconnectToast";

describe("ReconnectToast", () => {
  it("renders nothing when visible=false", () => {
    render(
      <ReconnectToast
        visible={false}
        message="Reconnect to apply."
        onDismiss={() => {}}
      />,
    );
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("shows message and Reconnect button when visible", () => {
    render(
      <ReconnectToast
        visible={true}
        message="Reconnect to apply."
        onReconnect={() => {}}
        onDismiss={() => {}}
      />,
    );
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.getByText("Reconnect to apply.")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /reconnect/i }),
    ).toBeInTheDocument();
  });

  it("hides Reconnect button when no onReconnect handler given", () => {
    render(
      <ReconnectToast
        visible={true}
        message="X"
        onDismiss={() => {}}
      />,
    );
    expect(screen.queryByRole("button", { name: /reconnect/i })).toBeNull();
  });

  it("invokes onReconnect on click", async () => {
    const onReconnect = vi.fn();
    render(
      <ReconnectToast
        visible={true}
        message="X"
        onReconnect={onReconnect}
        onDismiss={() => {}}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /reconnect/i }));
    expect(onReconnect).toHaveBeenCalledTimes(1);
  });

  it("invokes onDismiss on dismiss-X click", async () => {
    const onDismiss = vi.fn();
    render(
      <ReconnectToast
        visible={true}
        message="X"
        onReconnect={() => {}}
        onDismiss={onDismiss}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /dismiss/i }));
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });
});
