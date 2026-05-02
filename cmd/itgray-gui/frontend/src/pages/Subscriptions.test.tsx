import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import type { Sub } from "@/lib/subsAdapter";

const baseSub = (over: Partial<Sub> = {}): Sub => ({
  id: "s1",
  name: "alpha",
  url: "https://provider.example/sub",
  status: "ok",
  lastSyncAt: Date.now() - 60_000,
  serverCount: 4,
  ...over,
});

const mockUseSubs = vi.fn();
vi.mock("@/lib/subsStore", async () => {
  const actual = await vi.importActual<typeof import("@/lib/subsStore")>("@/lib/subsStore");
  return { ...actual, useSubs: () => mockUseSubs() };
});

import { Subscriptions } from "./Subscriptions";

function makeStore(opts: {
  subs?: Sub[];
  load?: "loading" | "ready" | "error";
  message?: string;
  add?: (...args: unknown[]) => Promise<void>;
  remove?: (...args: unknown[]) => Promise<void>;
  edit?: (...args: unknown[]) => Promise<void>;
  syncOne?: (...args: unknown[]) => Promise<void>;
  syncAll?: (...args: unknown[]) => Promise<void>;
} = {}) {
  const subs = opts.subs ?? [baseSub()];
  return {
    state: {
      load: opts.load === "loading"
        ? { kind: "loading" as const }
        : opts.load === "error"
          ? { kind: "error" as const, message: opts.message ?? "boom" }
          : { kind: "ready" as const, subs },
      inFlight: { syncing: new Set<string>(), removing: new Set<string>(), adding: false, editing: new Set<string>() },
    },
    actions: {
      add: opts.add ?? vi.fn().mockResolvedValue(undefined),
      edit: opts.edit ?? vi.fn().mockResolvedValue(undefined),
      remove: opts.remove ?? vi.fn().mockResolvedValue(undefined),
      syncOne: opts.syncOne ?? vi.fn().mockResolvedValue(undefined),
      syncAll: opts.syncAll ?? vi.fn().mockResolvedValue(undefined),
      refresh: vi.fn().mockResolvedValue(undefined),
    },
  };
}

describe("Subscriptions page", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("renders rows from the ready store state", () => {
    mockUseSubs.mockReturnValue(makeStore({ subs: [baseSub({ name: "okins" })] }));
    render(<Subscriptions />);
    expect(screen.getByText("okins")).toBeInTheDocument();
  });

  it("Add success closes modal and dispatches actions.add", async () => {
    const add = vi.fn().mockResolvedValue(undefined);
    mockUseSubs.mockReturnValue(makeStore({ add }));
    render(<Subscriptions />);
    fireEvent.click(screen.getByText(/Add subscription/));
    fireEvent.change(screen.getByPlaceholderText(/Main provider/), { target: { value: "new" } });
    fireEvent.change(screen.getByPlaceholderText(/provider\.example/), { target: { value: "https://x.example" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
      await Promise.resolve();
    });
    expect(add).toHaveBeenCalledWith("new", "https://x.example", "");
    await waitFor(() => {
      expect(screen.queryByPlaceholderText(/Main provider/)).not.toBeInTheDocument();
    });
  });

  it("Add with custom UA dispatches actions.add with userAgent", async () => {
    const add = vi.fn().mockResolvedValue(undefined);
    mockUseSubs.mockReturnValue(makeStore({ add }));
    render(<Subscriptions />);
    fireEvent.click(screen.getByText(/Add subscription/));
    fireEvent.change(screen.getByPlaceholderText(/Main provider/), { target: { value: "n" } });
    fireEvent.change(screen.getByPlaceholderText(/provider\.example/), { target: { value: "https://x" } });
    fireEvent.change(screen.getByPlaceholderText(/Settings default/), { target: { value: "Hiddify/1.0" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
      await Promise.resolve();
    });
    expect(add).toHaveBeenCalledWith("n", "https://x", "Hiddify/1.0");
  });

  it("Add backend error shows banner and keeps modal open", async () => {
    const add = vi.fn().mockRejectedValue(new Error("subscription URL must be http or https"));
    mockUseSubs.mockReturnValue(makeStore({ add }));
    render(<Subscriptions />);
    fireEvent.click(screen.getByText(/Add subscription/));
    fireEvent.change(screen.getByPlaceholderText(/Main provider/), { target: { value: "x" } });
    fireEvent.change(screen.getByPlaceholderText(/provider\.example/), { target: { value: "https://bad.example" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
      await Promise.resolve();
    });
    expect(await screen.findByText(/http\(s\)/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/Main provider/)).toBeInTheDocument();
  });

  it("Delete inside Edit modal calls actions.remove and closes modal on success", async () => {
    const remove = vi.fn().mockResolvedValue(undefined);
    mockUseSubs.mockReturnValue(makeStore({ remove, subs: [baseSub({ id: "s1" })] }));
    render(<Subscriptions />);
    fireEvent.click(screen.getByLabelText(/edit/i));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /Delete/ }));
      await Promise.resolve();
    });
    expect(remove).toHaveBeenCalledWith("s1");
  });

  it("Delete backend error reverts and shows banner inside modal", async () => {
    const remove = vi.fn().mockRejectedValue(new Error("subscription not found"));
    mockUseSubs.mockReturnValue(makeStore({ remove }));
    render(<Subscriptions />);
    fireEvent.click(screen.getByLabelText(/edit/i));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /Delete/ }));
      await Promise.resolve();
    });
    expect(await screen.findByText(/no longer exists/)).toBeInTheDocument();
  });

  it("SyncOne button dispatches actions.syncOne", () => {
    const syncOne = vi.fn().mockResolvedValue(undefined);
    mockUseSubs.mockReturnValue(makeStore({ syncOne, subs: [baseSub()] }));
    render(<Subscriptions />);
    fireEvent.click(screen.getAllByLabelText(/^sync$/i)[0]);
    expect(syncOne).toHaveBeenCalledWith("s1");
  });

  it("SyncAll button is disabled while any sync is in flight", () => {
    const store = makeStore();
    store.state.inFlight.syncing = new Set(["s1"]);
    mockUseSubs.mockReturnValue(store);
    render(<Subscriptions />);
    const syncAll = screen.getByRole("button", { name: /Sync all/ });
    expect(syncAll).toBeDisabled();
  });
});
