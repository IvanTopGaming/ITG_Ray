import { NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  Server,
  Globe,
  Settings as SettingsIcon,
  Route,
  ScrollText,
  ShieldOff,
} from "lucide-react";
import { motion } from "framer-motion";
import clsx from "clsx";

interface NavItem {
  to: string;
  label: string;
  icon: typeof LayoutDashboard;
  disabled?: boolean;
}

const main: NavItem[] = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/servers", label: "Servers", icon: Server },
  { to: "/subscriptions", label: "Subscriptions", icon: Globe },
  { to: "/routing", label: "Routing", icon: Route },
];

const soon: NavItem[] = [
  { to: "/logs", label: "Logs", icon: ScrollText, disabled: true },
  { to: "/kill-switch", label: "Kill-switch", icon: ShieldOff, disabled: true },
];

const settingsItem: NavItem = {
  to: "/settings",
  label: "Settings",
  icon: SettingsIcon,
};

export function Sidebar() {
  return (
    <aside className="glass-dim flex h-full w-[200px] flex-col gap-1 px-3 py-4">
      <div className="flex items-center gap-2.5 px-2 pb-3">
        <div className="h-7 w-7 rounded-lg bg-orb-accent shadow-[0_0_14px_rgba(120,200,255,0.55)]" />
        <span className="text-[13px] font-semibold tracking-tight">ITG Ray</span>
      </div>

      <SectionLabel>Main</SectionLabel>
      {main.map((it) => (
        <Item key={it.to} {...it} />
      ))}

      <SectionLabel className="mt-2 opacity-70">Soon</SectionLabel>
      {soon.map((it) => (
        <Item key={it.to} {...it} />
      ))}

      <div className="mt-auto flex flex-col gap-1 pt-3">
        <Item {...settingsItem} />
        <div className="mt-1 flex justify-between border-t border-white/5 px-3 pt-2.5 font-mono text-[10px] text-white/40">
          <span className="flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-success shadow-[0_0_6px_rgba(0,230,118,0.7)]" />
            running
          </span>
          <span>v0.0.0</span>
        </div>
      </div>
    </aside>
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

function Item({ to, label, icon: Icon, disabled }: NavItem) {
  if (disabled) {
    return (
      <div
        aria-disabled="true"
        className="flex cursor-not-allowed items-center gap-2.5 rounded-lg px-3 py-2 text-[12px] text-white/30"
      >
        <Icon className="h-3.5 w-3.5" />
        {label}
      </div>
    );
  }
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
