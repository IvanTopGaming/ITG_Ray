import { useStore } from "@/store";
import { Row, SectionShell } from "./primitives";

const fallback = { version: "dev", gitRev: "", buildDate: "" };

export function SectionAbout() {
  const a = useStore((s) => s.settings?.about) ?? fallback;
  return (
    <SectionShell id="about" title="About">
      <Row label="Version">
        <span className="text-sm text-text-primary font-mono">{a.version || "dev"}</span>
      </Row>
      {a.gitRev ? (
        <Row label="Git revision">
          <span className="text-sm text-text-muted font-mono">{a.gitRev}</span>
        </Row>
      ) : null}
      {a.buildDate ? (
        <Row label="Build date">
          <span className="text-sm text-text-muted font-mono">{a.buildDate}</span>
        </Row>
      ) : null}
      <Row label="Project">
        <a
          className="text-sm text-accent-primary hover:underline"
          href="https://github.com/itg-team/itg-ray"
          target="_blank"
          rel="noreferrer"
        >
          github.com/itg-team/itg-ray
        </a>
      </Row>
    </SectionShell>
  );
}
