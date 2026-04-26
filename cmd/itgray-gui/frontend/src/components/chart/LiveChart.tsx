import { useEffect, useRef, useState } from "react";
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip } from "recharts";
import { useStore } from "@/store";
import { SpeedBuffer, Sample } from "./buffer";

// CAPACITY is 5 minutes at 1 Hz (the chain backend emits vpn:speed once per
// second). Driving the buffer off speeds.at — refreshed by every applySpeed —
// keeps the chart in lockstep with the wire feed.
const CAPACITY = 300;

export function LiveChart() {
  const [samples, setSamples] = useState<Sample[]>([]);
  const bufRef = useRef(new SpeedBuffer(CAPACITY));
  const speeds = useStore((s) => s.speeds);

  useEffect(() => {
    bufRef.current.push({ upBps: speeds.upBps, downBps: speeds.downBps, t: Date.now() });
    setSamples([...bufRef.current.values()]);
  }, [speeds.at]);

  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.035] p-4 flex-1 min-h-0">
      <div className="text-[10px] uppercase tracking-wider text-text-muted mb-2">
        ↑/↓ bytes/s · 5 min
      </div>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={samples}>
          <defs>
            <linearGradient id="upG" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#6366f1" stopOpacity={0.45} />
              <stop offset="100%" stopColor="#6366f1" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="dnG" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#ec4899" stopOpacity={0.45} />
              <stop offset="100%" stopColor="#ec4899" stopOpacity={0} />
            </linearGradient>
          </defs>
          <XAxis dataKey="t" hide />
          <YAxis hide />
          <Tooltip
            contentStyle={{
              background: "rgba(10,13,23,0.9)",
              border: "1px solid rgba(255,255,255,0.1)",
              borderRadius: 8,
            }}
            labelFormatter={(v) => new Date(v).toLocaleTimeString()}
          />
          <Area type="monotone" dataKey="downBps" stroke="#ec4899" fill="url(#dnG)" />
          <Area type="monotone" dataKey="upBps" stroke="#6366f1" fill="url(#upG)" />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
