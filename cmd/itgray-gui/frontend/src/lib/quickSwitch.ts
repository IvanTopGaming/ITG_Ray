import type { hub } from "../../wailsjs/go/models";

type ServerView = hub.ServerView;

/**
 * pickQuickSwitch returns the top-N candidates for the Dashboard
 * QuickSwitch row: favorites first (sorted by latency ascending), then
 * non-favorites (also sorted by latency ascending). Servers with
 * latencyMs === 0 ("never probed") sort to the end of their group.
 */
export function pickQuickSwitch(all: ServerView[], n: number): ServerView[] {
  const byLatencyAsc = (a: ServerView, b: ServerView) =>
    (a.latencyMs || Number.POSITIVE_INFINITY) -
    (b.latencyMs || Number.POSITIVE_INFINITY);
  const favs = all.filter((s) => s.favorite).slice().sort(byLatencyAsc);
  const rest = all.filter((s) => !s.favorite).slice().sort(byLatencyAsc);
  return [...favs, ...rest].slice(0, n);
}
