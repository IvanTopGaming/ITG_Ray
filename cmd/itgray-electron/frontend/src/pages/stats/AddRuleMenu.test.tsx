import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { AddRuleMenu, placeNewRule } from "./AddRuleMenu";
import type { StatsConn } from "./format";

const groups = [
  { id: "safety", name: "Safety", locked: true, enabled: true, rules: [{ id: "lan", name: "LAN", enabled: true, action: "direct", conditions: {} }] },
  { id: "off", name: "Off", locked: false, enabled: false, rules: [] },
  { id: "mine", name: "Mine", locked: false, enabled: true, rules: [{ id: "top", name: "Top", enabled: true, action: "direct", conditions: {} }, { id: "video", name: "Video", enabled: true, action: "proxy", conditions: {} }] },
] as any[];

const domainConn: StatsConn = { id: "1", app: "firefox", host: "a.video.example.com", suffix: "example.com", destIp: "", route: "proxy", ruleId: "video", ruleGroupId: "mine", ruleGroupLocked: false, downBps: 0, upBps: 0 };
const ipConn: StatsConn = { id: "2", app: "", host: "149.154.167.51", destIp: "149.154.167.51", route: "proxy", ruleId: "final", downBps: 0, upBps: 0 };

describe("AddRuleMenu", () => {
  it("defaults to the registrable-domain suffix and ANDs the app when also picked", async () => {
    const onCreate = vi.fn();
    render(<AddRuleMenu conn={domainConn} groups={groups} onCreate={onCreate} />);
    await userEvent.click(screen.getByRole("button", { name: /\+ rule/i }));
    expect(screen.getByRole("checkbox", { name: /example\.com and subdomains/i })).toBeChecked();
    await userEvent.click(screen.getByRole("checkbox", { name: /app: firefox/i }));
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(onCreate).toHaveBeenCalledWith({
      groupId: "mine",
      beforeId: "video",
      prefill: { name: "example.com", conditions: { processes: ["firefox"], domains: [{ kind: "suffix", value: "example.com" }] } },
    });
  });

  it("offers only the IP when there is no domain", async () => {
    const onCreate = vi.fn();
    render(<AddRuleMenu conn={ipConn} groups={groups} onCreate={onCreate} />);
    await userEvent.click(screen.getByRole("button", { name: /\+ rule/i }));
    const boxes = screen.getAllByRole("checkbox");
    expect(boxes).toHaveLength(1);
    expect(boxes[0]).toBeChecked();
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(onCreate.mock.calls[0][0].prefill.conditions).toEqual({ ip_cidrs: ["149.154.167.51/32"] });
  });

  it("disables create when nothing is ticked", async () => {
    render(<AddRuleMenu conn={domainConn} groups={groups} onCreate={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /\+ rule/i }));
    await userEvent.click(screen.getByRole("checkbox", { name: /example\.com and subdomains/i }));
    expect(screen.getByRole("button", { name: /create rule/i })).toBeDisabled();
  });
});

describe("placeNewRule", () => {
  it("goes right above the matching rule in an unlocked group", () => {
    expect(placeNewRule(domainConn, groups)).toEqual({ groupId: "mine", beforeId: "video" });
  });

  it("falls back to the top of the first unlocked enabled group for final or locked rules", () => {
    expect(placeNewRule(ipConn, groups)).toEqual({ groupId: "mine", beforeId: "top" });
    expect(placeNewRule({ ...domainConn, ruleId: "lan", ruleGroupId: "safety", ruleGroupLocked: true }, groups)).toEqual({ groupId: "mine", beforeId: "top" });
  });
});
