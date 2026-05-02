import {
  CA,
  CH,
  DE,
  ES,
  FI,
  FR,
  GB,
  JP,
  NL,
  SE,
  SG,
  US,
} from "country-flag-icons/react/3x2";

// Convert a unicode regional-indicator emoji flag (e.g. "🇳🇱") to its
// 2-letter ISO 3166-1 alpha-2 country code (e.g. "NL"). Each regional
// indicator codepoint is offset by 0x1F1E6 from 'A'.
function emojiFlagToISO(emoji: string): string | null {
  // Each flag is two regional indicator chars; each is a surrogate pair.
  const codepoints = Array.from(emoji).map((c) => c.codePointAt(0) ?? 0);
  if (codepoints.length !== 2) return null;
  const [hi, lo] = codepoints;
  if (hi < 0x1f1e6 || hi > 0x1f1ff || lo < 0x1f1e6 || lo > 0x1f1ff) return null;
  const a = String.fromCharCode(hi - 0x1f1e6 + "A".charCodeAt(0));
  const b = String.fromCharCode(lo - 0x1f1e6 + "A".charCodeAt(0));
  return a + b;
}

// Reuse the library's exported component type so prop shapes line up with its
// HTMLSVGElement attribute set (HTMLAttributes & SVGAttributes union).
type FlagComponent = typeof NL;

// Hand-rolled lookup table. country-flag-icons exports ~270 components from a
// single index that bundlers can't tree-shake reliably (the re-export indirects
// through `../../modules/react/3x2/index.js`, so `import *` pulls everything,
// adding ~240 KB raw to the bundle). Named imports DO tree-shake, which means
// we must enumerate the countries we actually use. Mock data currently uses
// 12 of these — add new ones here as new servers appear.
const FLAGS: Record<string, FlagComponent> = {
  CA,
  CH,
  DE,
  ES,
  FI,
  FR,
  GB,
  JP,
  NL,
  SE,
  SG,
  US,
};

export type CountryFlagProps = {
  /** Either an emoji flag (e.g. "🇳🇱") OR an ISO 3166-1 alpha-2 code (e.g. "NL"). */
  code: string;
  /** Optional className for sizing/positioning. Default sizing is up to the caller. */
  className?: string;
  /** Accessible label override. Default: the ISO code. */
  title?: string;
};

/**
 * CountryFlag renders an SVG flag for the given country. Accepts either
 * a unicode regional-indicator emoji ("🇳🇱") or a literal ISO code ("NL").
 *
 * Falls back to rendering the raw input as text when:
 *   - input doesn't parse as either an emoji flag or 2-letter code,
 *   - or the country isn't in the FLAGS lookup table above.
 *
 * On Windows where the OS has no emoji-flag font, this is the only
 * way the user sees an actual flag — the bare emoji glyph degrades
 * to "NL" / "DE" / etc. otherwise. The emoji-text fallback is also
 * what renders the "all servers" pseudo-row's globe (🌐), which is
 * a generic emoji that Windows DOES render.
 */
export function CountryFlag({ code, className, title }: CountryFlagProps) {
  const iso =
    code.length === 2 && /^[A-Z]{2}$/i.test(code)
      ? code.toUpperCase()
      : emojiFlagToISO(code);
  if (!iso) {
    return <span className={className}>{code}</span>;
  }
  const Comp = FLAGS[iso];
  if (!Comp) {
    return <span className={className}>{iso}</span>;
  }
  return <Comp title={title ?? iso} className={className} />;
}
