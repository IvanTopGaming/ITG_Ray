import { NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  Server,
  Globe,
  Settings as SettingsIcon,
  Route,
  ScrollText,
} from "lucide-react";
import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import clsx from "clsx";
import { useEffect, useState } from "react";
import { useDash, effectiveStatus, type ChainStatus } from "@/lib/dashStore";
import { Get as GetSettings } from "@/lib/itg/SettingsService";
import type { hub } from "@/lib/itg/models";

interface NavItem {
  to: string;
  labelKey: string;
  icon: typeof LayoutDashboard;
}

const main: NavItem[] = [
  { to: "/", labelKey: "nav.dashboard", icon: LayoutDashboard },
  { to: "/servers", labelKey: "nav.servers", icon: Server },
  { to: "/subscriptions", labelKey: "nav.subscriptions", icon: Globe },
  { to: "/routing", labelKey: "nav.routing", icon: Route },
  { to: "/logs", labelKey: "nav.logs", icon: ScrollText },
];

const settingsItem: NavItem = {
  to: "/settings",
  labelKey: "nav.settings",
  icon: SettingsIcon,
};

const statusMeta: Record<
  ChainStatus | "error",
  { labelKey: string; dot: string }
> = {
  connected: {
    labelKey: "status.connected",
    dot: "bg-success shadow-[0_0_6px_rgba(0,230,118,0.7)]",
  },
  connecting: {
    labelKey: "status.connecting",
    dot: "bg-warn shadow-[0_0_6px_rgba(255,177,60,0.7)]",
  },
  disconnecting: {
    labelKey: "status.disconnecting",
    dot: "bg-warn shadow-[0_0_6px_rgba(255,177,60,0.7)]",
  },
  idle: { labelKey: "status.idle", dot: "bg-white/30" },
  error: {
    labelKey: "status.error",
    dot: "bg-danger shadow-[0_0_6px_rgba(255,94,94,0.7)]",
  },
};

export function Sidebar() {
  const { t } = useTranslation();
  return (
    <aside className="glass-dim flex h-full w-[200px] flex-col gap-1 px-3 py-4">
      <div className="flex items-center gap-2.5 px-2 pb-3">
        <div className="h-7 w-7 rounded-lg bg-orb-accent shadow-[0_0_14px_rgba(120,200,255,0.55)]" />
        <span className="text-[13px] font-semibold tracking-tight">ITG Ray</span>
      </div>

      <SectionLabel>{t("nav.main")}</SectionLabel>
      {main.map((it) => (
        <Item key={it.to} {...it} />
      ))}

      <div className="mt-auto flex flex-col gap-1 pt-3">
        <Item {...settingsItem} />
        <StatusFooter />
      </div>
    </aside>
  );
}

function StatusFooter() {
  const { t } = useTranslation();
  const status = effectiveStatus(useDash());
  const meta = statusMeta[status];
  const [version, setVersion] = useState<string | null>(null);
  useEffect(() => {
    GetSettings()
      .then((view) => setVersion((view as hub.SettingsView).about?.version ?? null))
      .catch(() => {});
  }, []);
  return (
    <div className="mt-1 flex justify-between border-t border-white/5 px-3 pt-2.5 font-mono text-[10px] text-white/40">
      <span className="flex items-center gap-1.5">
        <span className={clsx("h-1.5 w-1.5 rounded-full", meta.dot)} />
        {t(meta.labelKey)}
      </span>
      {version && <span>v{version}</span>}
    </div>
  );
}

function SectionLabel({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={clsx(
        "px-2 pb-1 pt-2 text-[9px] font-medium uppercase tracking-[0.18em] text-white/30",
        className,
      )}
    >
      {children}
    </div>
  );
}

function Item({ to, labelKey, icon: Icon }: NavItem) {
  const { t } = useTranslation();
  const label = t(labelKey);
  return (
    <NavLink
      to={to}
      end={to === "/"}
      className={({ isActive }) =>
        clsx(
          "relative flex items-center gap-2.5 rounded-lg px-3 py-2 text-[12px] transition-colors duration-standard ease-snap",
          isActive
            ? "font-medium text-white"
            : "text-white/65 hover:bg-white/[0.06] hover:text-white",
        )
      }
    >
      {({ isActive }) => (
        <>
          {isActive && (
            <motion.div
              layoutId="sidebar-active-pill"
              className="absolute inset-0 rounded-lg border border-white/10 bg-white/[0.12]"
              transition={{ type: "spring", stiffness: 400, damping: 32 }}
            />
          )}
          <Icon className="relative z-10 h-3.5 w-3.5" />
          <span className="relative z-10">{label}</span>
        </>
      )}
    </NavLink>
  );
}
