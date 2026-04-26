import { useStore } from "@/store";

export function DashboardPage() {
  const s = useStore((st) => ({
    status: st.status,
    helper: st.helperState,
    version: st.version,
  }));
  return (
    <div>
      <h1 className="text-2xl mb-2">Dashboard</h1>
      <pre className="text-xs">{JSON.stringify(s, null, 2)}</pre>
    </div>
  );
}
