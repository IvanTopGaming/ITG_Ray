import { useEffect, useState } from "react";
import {
  Connect as wailsConnect,
  Disconnect as wailsDisconnect,
} from "../../../wailsjs/go/bindings/RunService";
import { useStore } from "@/store";
import { ConnectButton } from "./ConnectButton";
import { ServerPicker } from "./ServerPicker";
import { ModeToggle } from "./ModeToggle";

// Wails generates TS signatures with a leading context.Context arg that
// the runtime fills in transparently. Cast to clean shapes so call sites
// stay readable. Mirrors api/client.ts and ServerActions.tsx.
const Connect = wailsConnect as unknown as (serverID: string, mode: string) => Promise<void>;
const Disconnect = wailsDisconnect as unknown as () => Promise<void>;

// Hero is the centrepiece of the Dashboard: a gradient card with the big
// Connect button surrounded by the server picker and the mode toggle. All
// state flows through the zustand store so vpn:status events from the
// chain controller drive the UI without extra plumbing.
export function Hero() {
  const status = useStore((s) => s.status);
  const servers = useStore((s) => s.servers);
  const cs = useStore((s) => s.currentServer);
  const [serverId, setServerId] = useState<string>(cs?.id ?? "");
  const [mode, setMode] = useState<string>("auto");

  // When the snapshot's currentServer changes (boot, reconcile-after-crash),
  // mirror it into the local picker selection so the dropdown reflects the
  // actually-active server rather than whatever was selected before.
  useEffect(() => {
    if (cs?.id) setServerId(cs.id);
  }, [cs]);

  const onClick = () => {
    if (status === "connected") {
      void Disconnect();
      return;
    }
    if (!serverId) return;
    void Connect(serverId, mode);
  };

  return (
    <div className="rounded-2xl border border-white/10 bg-gradient-to-br from-indigo-500/10 to-pink-500/5 p-6 flex flex-col items-center gap-4 h-[260px]">
      <ConnectButton status={status} onClick={onClick} />
      <div className="flex gap-2 items-center">
        <ServerPicker servers={servers} value={serverId} onChange={setServerId} />
        <ModeToggle value={mode} onChange={setMode} />
      </div>
    </div>
  );
}
