import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import path from "node:path";

export function loadLastServer(file: string): string | null {
  try {
    const data = JSON.parse(readFileSync(file, "utf8")) as { lastServerId?: unknown };
    return typeof data.lastServerId === "string" && data.lastServerId ? data.lastServerId : null;
  } catch {
    return null;
  }
}

export function saveLastServer(file: string, id: string): void {
  if (!id || loadLastServer(file) === id) return;
  try {
    mkdirSync(path.dirname(file), { recursive: true });
    writeFileSync(file, JSON.stringify({ lastServerId: id }), "utf8");
  } catch (err) {
    console.warn("tray: could not persist last server", err);
  }
}
