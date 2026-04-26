// ModeToggle is a 3-button segmented control selecting the connect mode.
// Auto: chainctl picks TUN, falls back to sysproxy if TunCreate fails.
// TUN: full-system routing via WinTUN. Sysproxy: per-app HKCU proxy only.
const modes = ["auto", "tun", "sysproxy"] as const;

export function ModeToggle({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <div className="inline-flex bg-white/[0.05] border border-white/10 rounded-full p-0.5 text-xs">
      {modes.map((m) => (
        <button
          key={m}
          onClick={() => onChange(m)}
          className={`px-3 h-7 rounded-full ${
            value === m
              ? "bg-gradient-to-br from-indigo-500 to-pink-500 text-white"
              : "text-text-secondary"
          }`}
        >
          {m.toUpperCase()}
        </button>
      ))}
    </div>
  );
}
