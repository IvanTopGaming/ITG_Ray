import { useStore } from "@/store";

export function ServersPage() {
  const servers = useStore((s) => s.servers);
  return (
    <div>
      <h1 className="text-2xl mb-2">Servers ({servers.length})</h1>
      <pre className="text-xs">{JSON.stringify(servers, null, 2)}</pre>
    </div>
  );
}
