import { useEffect } from "react";
import type { NavigateFunction } from "react-router-dom";
import {
  dashConnect,
  dashDisconnect,
  dashSetRoutingMode,
  dashSwitchMode,
  getDashState,
  loadLastServerId,
  useDash,
  type Mode,
  type RoutingMode,
} from "@/lib/dashStore";
import { useSubs, type SubsActions } from "@/lib/subsStore";

export type TrayCommand =
  | { kind: "connect" }
  | { kind: "disconnect" }
  | { kind: "selectServer"; id: string }
  | { kind: "setMode"; mode: Mode }
  | { kind: "setRoutingMode"; mode: RoutingMode }
  | { kind: "syncAll" }
  | { kind: "cancelSync" }
  | { kind: "navigate"; path: string };

export interface TrayCommandDeps {
  navigate: (path: string) => void;
  subs: Pick<SubsActions, "syncAll" | "cancelAll">;
}

function connectTarget(): string | null {
  const dash = getDashState();
  if (dash.currentServer?.id) return dash.currentServer.id;
  const last = loadLastServerId();
  if (last && dash.allServers.some((s) => s.id === last)) return last;
  return dash.allServers.find((s) => s.favorite)?.id ?? dash.allServers[0]?.id ?? null;
}

export async function runTrayCommand(cmd: TrayCommand, deps: TrayCommandDeps): Promise<void> {
  switch (cmd.kind) {
    case "connect": {
      const target = connectTarget();
      if (target) await dashConnect(target);
      return;
    }
    case "disconnect":
      await dashDisconnect();
      return;
    case "selectServer": {
      const dash = getDashState();
      if (dash.status === "connected" && dash.currentServer?.id === cmd.id) return;
      await dashConnect(cmd.id);
      return;
    }
    case "setMode":
      await dashSwitchMode(cmd.mode);
      return;
    case "setRoutingMode":
      await dashSetRoutingMode(cmd.mode);
      return;
    case "syncAll":
      await deps.subs.syncAll();
      return;
    case "cancelSync":
      await deps.subs.cancelAll();
      return;
    case "navigate":
      deps.navigate(cmd.path);
      return;
  }
}

function isTrayCommand(v: unknown): v is TrayCommand {
  return !!v && typeof (v as { kind?: unknown }).kind === "string";
}

export function useTrayBridge(navigate: NavigateFunction, language: string): void {
  const dash = useDash();
  const { state: subsState, actions } = useSubs();
  const syncing = subsState.inFlight.syncing.size > 0;

  useEffect(() => {
    const off = window.itg.on("tray.command", (payload: unknown) => {
      if (!isTrayCommand(payload)) return;
      void runTrayCommand(payload, { navigate, subs: actions }).catch(() => {});
    });
    return off;
  }, [navigate, actions]);

  useEffect(() => {
    void window.itg.tray?.takePendingNavigate?.().then((path) => {
      if (typeof path === "string" && path) navigate(path);
    });
  }, [navigate]);

  useEffect(() => {
    void window.itg.tray?.report?.({
      mode: dash.mode,
      syncing,
      language,
      lastServerId: loadLastServerId(),
    });
  }, [dash.mode, dash.status, syncing, language]);
}
