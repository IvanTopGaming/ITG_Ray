import { useStore } from "@/store";

export function SettingsPage() {
  const settings = useStore((s) => s.settings);
  return (
    <div>
      <h1 className="text-2xl mb-2">Settings</h1>
      <pre className="text-xs">{JSON.stringify(settings, null, 2)}</pre>
    </div>
  );
}
