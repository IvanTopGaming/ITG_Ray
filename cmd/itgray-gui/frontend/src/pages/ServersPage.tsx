import { useStore } from "@/store";
import { ServerTable } from "@/components/server-table/ServerTable";

export function ServersPage() {
  const servers = useStore((s) => s.servers);
  return (
    <div className="h-full">
      <ServerTable servers={servers} />
    </div>
  );
}
