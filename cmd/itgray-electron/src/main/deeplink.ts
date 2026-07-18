export function extractDeeplink(argv: string[]): string | null {
  return argv.find((a) => a.startsWith("itgray://")) ?? null;
}
