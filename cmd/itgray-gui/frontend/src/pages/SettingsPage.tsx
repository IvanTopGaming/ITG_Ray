import { useEffect, useState } from "react";
import { SectionGeneral } from "@/components/settings/SectionGeneral";
import { SectionNetwork } from "@/components/settings/SectionNetwork";
import { SectionSubs } from "@/components/settings/SectionSubs";
import { SectionNotifications } from "@/components/settings/SectionNotifications";
import { SectionDebug } from "@/components/settings/SectionDebug";
import { SectionAbout } from "@/components/settings/SectionAbout";
import { SectionSecurity } from "@/components/settings/SectionSecurity";

// Section nav anchors; order matches the visible section ordering and the
// id attribute on each SectionShell. Keeping the source of truth in one
// place avoids drift between the nav and the content.
const sections = [
  { id: "general", label: "General" },
  { id: "network", label: "Network" },
  { id: "subs", label: "Subscriptions" },
  { id: "notifications", label: "Notifications" },
  { id: "debug", label: "Debug" },
  { id: "about", label: "About" },
  { id: "security", label: "Security" },
] as const;

export function SettingsPage() {
  const [active, setActive] = useState<string>(sections[0].id);

  // Sync the active nav item to scroll position via IntersectionObserver.
  // Each section header crossing the top 30% of the viewport activates
  // that nav entry. Falls back to the first section on mount.
  useEffect(() => {
    const els = sections
      .map((s) => document.getElementById(s.id))
      .filter((e): e is HTMLElement => e !== null);
    if (els.length === 0) return;
    const obs = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible.length > 0) setActive(visible[0].target.id);
      },
      { rootMargin: "-20% 0px -60% 0px", threshold: 0 },
    );
    els.forEach((el) => obs.observe(el));
    return () => obs.disconnect();
  }, []);

  return (
    <div className="flex gap-6 max-w-5xl mx-auto">
      <nav className="hidden md:block w-44 flex-shrink-0 sticky top-2 self-start">
        <ul className="space-y-1">
          {sections.map((s) => (
            <li key={s.id}>
              <a
                href={`#${s.id}`}
                className={`block px-3 py-1.5 rounded-md text-sm transition-colors ${
                  active === s.id
                    ? "bg-accent-primary/15 text-text-primary"
                    : "text-text-secondary hover:bg-white/5 hover:text-text-primary"
                }`}
                onClick={() => setActive(s.id)}
              >
                {s.label}
              </a>
            </li>
          ))}
        </ul>
      </nav>
      <div className="flex-1 min-w-0 space-y-4">
        <h1 className="text-2xl font-semibold">Settings</h1>
        <SectionGeneral />
        <SectionNetwork />
        <SectionSubs />
        <SectionNotifications />
        <SectionDebug />
        <SectionAbout />
        <SectionSecurity />
      </div>
    </div>
  );
}
