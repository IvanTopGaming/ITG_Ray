import { NavLink } from "react-router-dom";
import { Home, Globe, Link2, Settings } from "lucide-react";
import { useTranslation } from "react-i18next";

export function Sidebar() {
  const { t } = useTranslation();
  const items = [
    { to: "/dashboard", icon: Home, label: t("sidebar.dashboard") },
    { to: "/servers", icon: Globe, label: t("sidebar.servers") },
    { to: "/subs", icon: Link2, label: t("sidebar.subs") },
    { to: "/settings", icon: Settings, label: t("sidebar.settings") },
  ];
  return (
    <nav className="w-[220px] shrink-0 border-r border-white/10 p-3 flex flex-col gap-1 bg-white/[0.025]">
      {items.map((it) => (
        <NavLink
          key={it.to}
          to={it.to}
          className={({ isActive }) =>
            `flex items-center gap-3 px-3 h-9 rounded-md text-sm transition ${
              isActive
                ? "bg-gradient-to-br from-indigo-500 to-pink-500 text-white"
                : "text-text-secondary hover:bg-white/5"
            }`
          }
        >
          <it.icon size={16} strokeWidth={1.5} />
          {it.label}
        </NavLink>
      ))}
    </nav>
  );
}
