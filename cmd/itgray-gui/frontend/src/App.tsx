import { useEffect, useState } from "react";

export default function App() {
  const [version, setVersion] = useState<string>("…");
  useEffect(() => {
    // Wails-generated bindings live under window.go after `wails build`.
    // For C.T1 the binding has not been generated yet — placeholder string.
    const w = window as unknown as { go?: { main?: { App?: { GetVersion?: () => Promise<string> } } } };
    w.go?.main?.App?.GetVersion?.().then(setVersion).catch(() => setVersion("(binding unavailable)"));
  }, []);
  return (
    <div className="min-h-screen flex items-center justify-center bg-surface-base text-text-primary">
      <div className="text-center">
        <h1 className="text-3xl font-semibold mb-2">ITG Ray</h1>
        <p className="text-text-secondary">version {version}</p>
      </div>
    </div>
  );
}
