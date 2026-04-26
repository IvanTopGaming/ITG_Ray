import { useStore } from "@/store";

export function SubsPage() {
  const subs = useStore((s) => s.subs);
  return (
    <div>
      <h1 className="text-2xl mb-2">Subscriptions ({subs.length})</h1>
      <pre className="text-xs">{JSON.stringify(subs, null, 2)}</pre>
    </div>
  );
}
