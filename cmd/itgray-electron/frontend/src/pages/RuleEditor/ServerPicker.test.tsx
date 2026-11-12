import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { ServerPicker } from "./ServerPicker";

const servers = [
  { id: "a", name: "Netherlands 2", country: "NL", sourceId: "sub", origin: "P" },
  { id: "b", name: "Finland", country: "FI", sourceId: "sub", origin: "P" },
] as any[];

describe("ServerPicker", () => {
  it("shows the current-server choice by default and lists servers on open", async () => {
    render(<ServerPicker value={null} servers={servers} onChange={() => {}} />);
    const trigger = screen.getByRole("button", { name: /current server/i });
    await userEvent.click(trigger);
    expect(screen.getByRole("option", { name: /netherlands 2/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /finland/i })).toBeInTheDocument();
  });

  it("filters by search text", async () => {
    render(<ServerPicker value={null} servers={servers} onChange={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: /current server/i }));
    await userEvent.type(screen.getByPlaceholderText(/search servers/i), "fin");
    expect(screen.queryByRole("option", { name: /netherlands 2/i })).toBeNull();
    expect(screen.getByRole("option", { name: /finland/i })).toBeInTheDocument();
  });

  it("picks a server and the current-server entry", async () => {
    const onChange = vi.fn();
    const { rerender } = render(<ServerPicker value={null} servers={servers} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /current server/i }));
    await userEvent.click(screen.getByRole("option", { name: /finland/i }));
    expect(onChange).toHaveBeenLastCalledWith(servers[1]);
    rerender(<ServerPicker value="b" servers={servers} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /finland/i }));
    await userEvent.click(screen.getByRole("option", { name: /current server/i }));
    expect(onChange).toHaveBeenLastCalledWith(null);
  });

  it("shows the stored name when the bound server is gone", () => {
    render(<ServerPicker value="gone" fallbackName="Old NL" servers={servers} onChange={() => {}} />);
    expect(screen.getByRole("button", { name: /old nl/i })).toBeInTheDocument();
  });
});
