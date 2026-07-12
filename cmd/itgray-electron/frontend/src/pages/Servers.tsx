import React, { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import {
  AlertTriangle,
  Copy,
  Eye,
  Globe,
  Pencil,
  Plus,
  Search,
  Star,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import { cn } from "@/lib/cn";
import { CountryFlag } from "@/components/controls/CountryFlag";
import { Dropdown } from "@/components/controls/Dropdown";
import {
  clearLastError,
  serverAdd,
  serverEdit,
  serverRemove,
  useServers,
} from "@/lib/serversStore";
import {
  effectiveStatus,
  useDash,
  dashConnect,
  dashProbeOne,
  dashProbeAll,
} from "@/lib/dashStore";
import {
  markActiveServerEdited,
  setDesiredServer,
  useDesiredServer,
} from "@/lib/settings";
import { ToggleFavorite } from "@/lib/itg/ServersService";
import type { hub } from "@/lib/itg/models";

type Sort = "latency" | "name";

type Server = hub.ServerView;

const MANUAL_ORIGIN = "manual";
const MANUAL_GROUP_ID = "__manual__";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { staggerChildren: 0.06, delayChildren: 0.05 },
  },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 10 },
  show: { opacity: 1, y: 0, transition: { duration: 0.35, ease: SNAP_EASE } },
};

type ModalState =
  | { kind: "closed" }
  | { kind: "add" }
  | { kind: "view"; server: Server }
  | { kind: "edit"; server: Server };

export function Servers() {
  const { t } = useTranslation();
  // Use dash.allServers as the rendered source (it carries resolved origin
  // labels from GetSnapshot's originByID map). serversStore.useServers()
  // is consumed only for lastError + the bootstrap side-effect.
  const dash = useDash();
  const { lastError } = useServers();
  const status = effectiveStatus(dash);

  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<Sort>("name");
  const [favoritesFirst, setFavoritesFirst] = useState(true);
  const [modal, setModal] = useState<ModalState>({ kind: "closed" });
  const [submitError, setSubmitError] = useState<string | null>(null);

  const servers = dash.allServers;
  const activeServerId = dash.currentServer?.id ?? null;
  const desiredServerId = useDesiredServer();

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return servers;
    return servers.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.country.toLowerCase().includes(q) ||
        s.address.toLowerCase().includes(q),
    );
  }, [servers, search]);

  const grouped = useMemo(() => {
    const out: Array<{
      id: string;
      label: string;
      hint: string | null;
      origin: "manual" | "subscription";
      rows: Server[];
    }> = [];

    const sortRows = (rows: Server[]) =>
      [...rows].sort((a, b) => {
        if (favoritesFirst && a.favorite !== b.favorite) {
          return a.favorite ? -1 : 1;
        }
        if (sort === "latency") {
          // 0 = unprobed → sort to bottom
          const al = a.latencyMs > 0 ? a.latencyMs : Number.MAX_SAFE_INTEGER;
          const bl = b.latencyMs > 0 ? b.latencyMs : Number.MAX_SAFE_INTEGER;
          if (al !== bl) return al - bl;
          // Stable secondary sort by name when latencies tie (e.g. unprobed
          // batch). Without this, the visible order falls back to insertion
          // order from disk and looks like "by name".
          return a.name.localeCompare(b.name);
        }
        return a.name.localeCompare(b.name);
      });

    const manualRows = filtered.filter((s) => s.origin === MANUAL_ORIGIN);
    if (manualRows.length) {
      out.push({
        id: MANUAL_GROUP_ID,
        label: t("servers.manual"),
        hint: t("servers.serverCount", { count: manualRows.length }),
        origin: "manual",
        rows: sortRows(manualRows),
      });
    }

    // Group subscription servers by origin (= subscription name).
    const subOrigins = new Set<string>();
    for (const s of filtered) {
      if (s.origin && s.origin !== MANUAL_ORIGIN) subOrigins.add(s.origin);
    }
    for (const subName of [...subOrigins].sort()) {
      const rows = filtered.filter((s) => s.origin === subName);
      out.push({
        id: `sub:${subName}`,
        label: subName,
        hint: t("servers.serverCount", { count: rows.length }),
        origin: "subscription",
        rows: sortRows(rows),
      });
    }
    return out;
  }, [filtered, sort, favoritesFirst, t]);

  async function toggleFavorite(id: string) {
    try {
      await ToggleFavorite(id);
      // No local mutation — backend publishes servers:changed and the
      // dash bootstrap refreshes allServers.
    } catch (err: any) {
      setSubmitError(err?.message ?? String(err));
    }
  }

  async function handleAddManual(name: string, uri: string) {
    try {
      await serverAdd(uri, name);
      setModal({ kind: "closed" });
      setSubmitError(null);
    } catch (err: any) {
      // Keep the modal open — the modal owns the error display.
      // Suppress the page-level banner (the store also surfaces lastError
      // for non-modal callers, but here the modal is dominant).
      setSubmitError(err?.message ?? String(err));
      clearLastError();
    }
  }

  async function handleSaveEdit(id: string, name: string, uri: string) {
    try {
      const { vlessChanged } = await serverEdit(id, uri, name);
      setModal({ kind: "closed" });
      setSubmitError(null);
      if (
        vlessChanged &&
        activeServerId === id &&
        (status === "connected" || status === "connecting")
      ) {
        markActiveServerEdited();
      }
    } catch (err: any) {
      setSubmitError(err?.message ?? String(err));
      clearLastError();
    }
  }

  async function handleDelete(id: string) {
    try {
      await serverRemove(id);
      setModal({ kind: "closed" });
      setSubmitError(null);
    } catch (err: any) {
      setSubmitError(err?.message ?? String(err));
      clearLastError();
    }
  }

  function dismissLastError() {
    clearLastError();
    setSubmitError(null);
  }

  // The page-level banner shows lastError (for non-modal mutations like
  // ToggleFavorite, or raw probe failures). When a mutation fails inside an
  // open modal, the handler stores submitError and calls clearLastError() —
  // the modal renders submitError inline so the banner stays clean.
  const errorMessage = lastError;

  return (
    <>
      <motion.section
        variants={containerVariants}
        initial="hidden"
        animate="show"
        className="flex flex-col gap-5"
      >
        <motion.div
          variants={itemVariants}
          className="flex items-center justify-between gap-4"
        >
          <div>
            <h1 className="text-[22px] font-semibold tracking-tight">{t("servers.title")}</h1>
            <p className="mt-1 text-[13px] text-white/50">{t("servers.description")}</p>
          </div>
          <div className="flex items-center gap-2">
            <SearchInput value={search} onChange={setSearch} />
            <SortToggle value={sort} onChange={setSort} />
            <FavoritesToggle
              value={favoritesFirst}
              onChange={setFavoritesFirst}
            />
            <AddServerButton onClick={() => setModal({ kind: "add" })} />
            <ProbeAllButton onClick={() => void dashProbeAll()} />
          </div>
        </motion.div>

        {errorMessage && (
          <ErrorBanner
            message={errorMessage}
            onDismiss={dismissLastError}
          />
        )}

        {!dash.bootstrapped ? null : grouped.length === 0 ? (
          <motion.div
            variants={itemVariants}
            className="glass-regular rounded-2xl p-10 text-center text-[13px] text-white/55"
          >
            {servers.length === 0
              ? t("servers.emptyState")
              : t("servers.noMatches", { query: search })}
          </motion.div>
        ) : (
          <AnimatePresence>
            {grouped.map((group) => (
              <motion.div
                key={group.id}
                layout
                variants={itemVariants}
                exit={{
                  opacity: 0,
                  height: 0,
                  marginTop: 0,
                  transition: { duration: 0.3, ease: SNAP_EASE },
                }}
                className="flex flex-col gap-2 overflow-hidden"
              >
              <div className="flex items-baseline justify-between px-1">
                <div className="flex items-baseline gap-2">
                  <span className="text-[12px] font-semibold">
                    {group.label}
                  </span>
                  {group.origin === "manual" && (
                    <span className="rounded bg-warn/15 px-1.5 py-px text-[8px] font-bold uppercase tracking-[0.16em] text-warn">
                      {t("servers.manualBadge")}
                    </span>
                  )}
                  {group.hint && (
                    <span className="font-mono text-[10px] tabular-nums text-white/40">
                      {group.hint}
                    </span>
                  )}
                </div>
              </div>
              <motion.div
                layout
                className="glass-regular flex flex-col overflow-hidden rounded-2xl"
              >
                <AnimatePresence mode="popLayout" initial={false}>
                  {group.rows.map((server, idx) => {
                  const active = server.id === activeServerId;
                  const hasPending =
                    desiredServerId != null &&
                    desiredServerId !== activeServerId;
                  return (
                    <motion.div
                      key={server.id}
                      layout
                      initial={{ opacity: 0, scale: 0.96 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, x: -24, scale: 0.96 }}
                      transition={{ duration: 0.26, ease: SNAP_EASE }}
                    >
                      <ServerRow
                        server={server}
                        active={active}
                        pending={server.id === desiredServerId && !active}
                        revertable={active && hasPending}
                        probing={dash.probeState.get(server.id) === "probing"}
                        isLast={idx === group.rows.length - 1}
                        onToggleFavorite={() => void toggleFavorite(server.id)}
                        onProbe={() => void dashProbeOne(server.id)}
                        onAction={() =>
                          setModal({
                            kind:
                              server.origin === MANUAL_ORIGIN ? "edit" : "view",
                            server,
                          })
                        }
                        onSelectActive={() => {
                          if (status === "connecting" || status === "disconnecting") return;
                          if (status === "connected") {
                            setDesiredServer(server.id);
                          } else {
                            void dashConnect(server.id).catch(() => {
                              /* dashStore sets lastError; banner shows it */
                            });
                          }
                        }}
                      />
                    </motion.div>
                  );
                })}
                </AnimatePresence>
              </motion.div>
            </motion.div>
          ))}
          </AnimatePresence>
        )}
      </motion.section>

      <AnimatePresence>
        {modal.kind !== "closed" && (
          <ServerModal
            modal={modal}
            submitError={submitError}
            onClose={() => {
              setModal({ kind: "closed" });
              setSubmitError(null);
            }}
            onAdd={handleAddManual}
            onSaveEdit={handleSaveEdit}
            onDelete={handleDelete}
          />
        )}
      </AnimatePresence>
    </>
  );
}

function ErrorBanner({
  message,
  onDismiss,
}: {
  message: string;
  onDismiss: () => void;
}) {
  const { t } = useTranslation();
  return (
    <motion.div
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -8 }}
      transition={{ duration: 0.22, ease: SNAP_EASE }}
      role="alert"
      className="flex items-start gap-3 rounded-xl border border-danger/40 bg-danger/[0.10] px-4 py-3"
    >
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-danger" />
      <div className="flex-1 text-[12px] text-white/85">{message}</div>
      <button
        onClick={onDismiss}
        className="rounded p-1 text-white/55 transition-colors hover:bg-white/[0.06] hover:text-white"
        aria-label={t("servers.dismissError")}
      >
        <X className="h-3.5 w-3.5" />
      </button>
    </motion.div>
  );
}

function SearchInput({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="glass-regular flex items-center gap-2 rounded-lg px-3 py-1.5">
      <Search className="h-3.5 w-3.5 text-white/45" />
      <input
        type="text"
        placeholder={t("servers.searchPlaceholder")}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-[200px] bg-transparent text-[12px] text-white placeholder:text-white/35 focus:outline-none"
      />
    </div>
  );
}

function SortToggle({
  value,
  onChange,
}: {
  value: Sort;
  onChange: (s: Sort) => void;
}) {
  const { t } = useTranslation();
  const sortLabel: Record<Sort, string> = {
    latency: t("servers.sortLatency"),
    name: t("servers.sortName"),
  };
  return (
    <div className="glass-regular flex gap-0 rounded-lg p-1">
      {(["latency", "name"] as const).map((s) => (
        <button
          key={s}
          onClick={() => onChange(s)}
          className={cn(
            "relative rounded-md px-2.5 py-1 text-[10px] font-medium uppercase tracking-[0.12em] transition-colors duration-instant ease-snap",
            value === s ? "text-white" : "text-white/55 hover:text-white",
          )}
        >
          {value === s && (
            <motion.div
              layoutId="servers-sort-pill"
              className="absolute inset-0 rounded-md bg-white/[0.14]"
              transition={{ type: "spring", stiffness: 380, damping: 32 }}
            />
          )}
          <span className="relative z-10">{sortLabel[s]}</span>
        </button>
      ))}
    </div>
  );
}

function FavoritesToggle({
  value,
  onChange,
}: {
  value: boolean;
  onChange: (v: boolean) => void;
}) {
  const { t } = useTranslation();
  return (
    <button
      onClick={() => onChange(!value)}
      className={cn(
        "glass-regular flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[11px] font-medium transition-colors duration-instant ease-snap",
        value ? "text-warn" : "text-white/55 hover:text-white",
      )}
      title={t("servers.favouritesFirst")}
    >
      <Star className={cn("h-3.5 w-3.5", value && "fill-warn stroke-warn")} />
    </button>
  );
}

function AddServerButton({ onClick }: { onClick: () => void }) {
  const { t } = useTranslation();
  return (
    <motion.button
      onClick={onClick}
      whileHover={{ y: -1 }}
      whileTap={{ scale: 0.96 }}
      transition={{ duration: 0.15, ease: SNAP_EASE }}
      className="glass-regular flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[11px] font-semibold text-white transition-colors duration-instant ease-snap hover:!border-white/30 hover:bg-white/[0.08]"
    >
      <Plus className="h-3.5 w-3.5" />
      {t("servers.addServer")}
    </motion.button>
  );
}

function ProbeAllButton({ onClick }: { onClick: () => void }) {
  const { t } = useTranslation();
  return (
    <motion.button
      onClick={onClick}
      whileHover={{ y: -1 }}
      whileTap={{ scale: 0.96 }}
      transition={{ duration: 0.15, ease: SNAP_EASE }}
      className="flex items-center gap-1.5 rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-3 py-1.5 text-[11px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-shadow duration-instant ease-snap hover:shadow-[0_0_22px_rgba(120,200,255,0.45)]"
    >
      <Zap className="h-3.5 w-3.5" />
      {t("servers.probeAll")}
    </motion.button>
  );
}

function ServerRow({
  server,
  active,
  pending,
  revertable,
  probing,
  isLast,
  onToggleFavorite,
  onProbe,
  onAction,
  onSelectActive,
}: {
  server: Server;
  active: boolean;
  pending: boolean;
  revertable: boolean;
  probing: boolean;
  isLast: boolean;
  onToggleFavorite: () => void;
  onProbe: () => void;
  onAction: () => void;
  onSelectActive: () => void;
}) {
  const { t } = useTranslation();
  const ping = server.latencyMs;
  const pingColor =
    ping === 0
      ? "text-white/40"
      : ping < 25
        ? "text-success"
        : ping < 60
          ? "text-warn"
          : ping < 120
            ? "text-white/65"
            : "text-danger";

  const isManual = server.origin === MANUAL_ORIGIN;
  const ActionIcon = isManual ? Pencil : Eye;
  const actionTitle = isManual ? t("common.edit") : t("servers.viewDetails");

  return (
    <div
      onClick={() => {
        if (!active || revertable) onSelectActive();
      }}
      className={cn(
        "group relative flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors duration-instant ease-snap",
        !isLast && "border-b border-white/[0.05]",
        active && !revertable
          ? "cursor-default bg-success/[0.06]"
          : active
            ? "cursor-pointer bg-success/[0.06]"
            : "cursor-pointer hover:bg-white/[0.03]",
      )}
    >
      {active && (
        <motion.div
          layoutId="servers-active-ring"
          className="pointer-events-none absolute inset-0 border-y-2 border-success/55 bg-success/[0.02]"
          transition={{ type: "spring", stiffness: 380, damping: 32 }}
        />
      )}
      {pending && !active && (
        <motion.div
          layoutId="servers-pending-ring"
          className="pointer-events-none absolute inset-0 border-y-2 border-accent/55 bg-accent/[0.04]"
          transition={{ type: "spring", stiffness: 380, damping: 32 }}
        />
      )}

      <span className="relative z-10 inline-flex h-2 w-2 items-center justify-center">
        <span
          className={cn(
            "h-2 w-2 rounded-full",
            active
              ? "bg-success shadow-[0_0_6px_rgba(0,230,118,0.7)]"
              : pending
                ? "bg-accent shadow-[0_0_6px_rgba(52,120,224,0.7)]"
                : "bg-white/20",
          )}
        />
      </span>

      {server.country ? (
        <CountryFlag
          code={server.country}
          className="relative z-10 h-[14px] w-[21px] shrink-0 rounded-[2px] object-cover shadow-[0_0_0_1px_rgba(255,255,255,0.08)]"
        />
      ) : (
        <Globe
          className="relative z-10 h-[14px] w-[14px] shrink-0 text-white/40"
          aria-hidden
        />
      )}

      <div className="relative z-10 flex min-w-0 flex-1 items-baseline gap-3">
        <span className="text-[13px] font-medium">{server.name}</span>
        <span className="font-mono text-[10px] tabular-nums text-white/40">
          {server.address}
        </span>
      </div>

      <div className="relative z-10 flex w-[58px] shrink-0 justify-end">
        {active && (
          <motion.span
            animate={{ opacity: [0.55, 1, 0.55] }}
            transition={{
              duration: 1.8,
              repeat: Infinity,
              ease: "easeInOut",
            }}
            className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.16em] text-success"
          >
            <span className="h-1 w-1 rounded-full bg-success shadow-[0_0_4px_rgba(0,230,118,0.8)]" />
            {t("servers.active")}
          </motion.span>
        )}
        {pending && !active && (
          <span className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.16em] text-accent">
            <span className="h-1 w-1 rounded-full bg-accent shadow-[0_0_4px_rgba(52,120,224,0.8)]" />
            {t("servers.pending")}
          </span>
        )}
      </div>

      <button
        onClick={(e) => {
          e.stopPropagation();
          onToggleFavorite();
        }}
        className="relative z-10 rounded p-1 hover:bg-white/[0.06]"
        title={server.favorite ? t("servers.unfavourite") : t("servers.favourite")}
      >
        <Star
          className={cn(
            "h-3.5 w-3.5 transition-colors",
            server.favorite
              ? "fill-warn stroke-warn"
              : "stroke-white/30 group-hover:stroke-white/55",
          )}
        />
      </button>

      <button
        onClick={(e) => {
          e.stopPropagation();
          if (!probing) onProbe();
        }}
        className={cn(
          "relative z-10 min-w-[64px] rounded px-2 py-0.5 text-right font-mono text-[11px] font-semibold tabular-nums transition-colors hover:bg-white/[0.06]",
          probing ? "text-white/45" : pingColor,
        )}
        title={t("servers.reprobe")}
      >
        {probing ? t("servers.probing") : ping > 0 ? `${ping} ms` : "—"}
      </button>

      <button
        onClick={(e) => {
          e.stopPropagation();
          onAction();
        }}
        className="relative z-10 rounded p-1 text-white/40 transition-colors hover:bg-white/[0.06] hover:text-white"
        title={actionTitle}
      >
        <ActionIcon className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

// ─── VLESS parsing/building ────────────────────────────────────────────

interface VlessConfig {
  name: string;
  uuid: string;
  host: string;
  port: string; // string for input ergonomics; validated on save
  type: string;
  path: string;
  hostHeader: string;
  security: string; // none, tls, reality
  sni: string;
  fingerprint: string; // chrome, firefox, safari, ios, android, edge, 360, qq, random, randomized
  publicKey: string;
  shortId: string;
  flow: string; // empty, xtls-rprx-vision
  mode: string; // xhttp/grpc mode
  serviceName: string; // grpc
  seed: string; // mkcp
  headerType: string; // tcp/mkcp
  quicSecurity: string; // quic
  key: string; // quic
}

const EMPTY_VLESS: VlessConfig = {
  name: "",
  uuid: "",
  host: "",
  port: "443",
  type: "tcp",
  path: "",
  hostHeader: "",
  security: "none",
  sni: "",
  fingerprint: "",
  publicKey: "",
  shortId: "",
  flow: "",
  mode: "",
  serviceName: "",
  seed: "",
  headerType: "",
  quicSecurity: "",
  key: "",
};

export function parseVless(uri: string): VlessConfig {
  const c: VlessConfig = { ...EMPTY_VLESS };
  if (!uri) return c;
  // The WHATWG URL parser treats vless:// as a non-special scheme and does
  // NOT extract userinfo/hostname into url.username/url.hostname (returns
  // empty strings for both). Wails' bundled WebKit was more lenient; Chromium
  // in Electron strictly follows the spec. Parse manually with a regex that
  // tolerates the optional port, query, and fragment.
  //   vless://<uuid>@<host>[:<port>][?<query>][#<fragment>]
  const m = uri.trim().match(
    /^vless:\/\/([^@]+)@([^:/?#]+)(?::(\d+))?(?:\?([^#]*))?(?:#(.*))?$/,
  );
  if (!m) return c;
  try {
    c.uuid = decodeURIComponent(m[1]);
    c.host = m[2];
    c.port = m[3] || "443";
    c.name = m[5] ? decodeURIComponent(m[5]) : "";
    const p = new URLSearchParams(m[4] || "");
    c.type = p.get("type") || "tcp";
    c.path = p.get("path") ? decodeURIComponent(p.get("path")!) : "";
    c.hostHeader = p.get("host") || "";
    c.security = p.get("security") || "none";
    c.sni = p.get("sni") || "";
    c.fingerprint = p.get("fp") || "";
    c.publicKey = p.get("pbk") || "";
    c.shortId = p.get("sid") || "";
    c.flow = p.get("flow") || "";
    c.mode = p.get("mode") || "";
    c.serviceName = p.get("serviceName") || "";
    c.seed = p.get("seed") || "";
    c.headerType = p.get("headerType") || "";
    c.quicSecurity = p.get("quicSecurity") || "";
    c.key = p.get("key") || "";
  } catch {
    /* malformed encoding — keep empty */
  }
  return c;
}

export function buildVless(c: VlessConfig): string {
  if (!c.uuid || !c.host) return "";
  const params = new URLSearchParams();
  params.set("encryption", "none");
  if (c.flow) params.set("flow", c.flow);
  if (c.type) params.set("type", c.type);
  if (c.path) params.set("path", c.path);
  if (c.hostHeader) params.set("host", c.hostHeader);
  if (c.mode) params.set("mode", c.mode);
  if (c.serviceName) params.set("serviceName", c.serviceName);
  if (c.seed) params.set("seed", c.seed);
  if (c.headerType) params.set("headerType", c.headerType);
  if (c.quicSecurity) params.set("quicSecurity", c.quicSecurity);
  if (c.key) params.set("key", c.key);
  if (c.security) params.set("security", c.security);
  if (c.sni) params.set("sni", c.sni);
  if (c.fingerprint) params.set("fp", c.fingerprint);
  if (c.publicKey) params.set("pbk", c.publicKey);
  if (c.shortId) params.set("sid", c.shortId);
  const port = c.port || "443";
  const namePart = c.name ? `#${encodeURIComponent(c.name)}` : "";
  return `vless://${encodeURIComponent(c.uuid)}@${c.host}:${port}?${params.toString()}${namePart}`;
}

const TRANSPORT_TYPES = [
  { value: "tcp", label: "TCP" },
  { value: "ws", label: "WebSocket" },
  { value: "grpc", label: "gRPC" },
  { value: "httpupgrade", label: "HTTPUpgrade" },
  { value: "xhttp", label: "XHTTP" },
  { value: "mkcp", label: "mKCP" },
  { value: "quic", label: "QUIC" },
];

const SECURITY_TYPES = [
  { value: "none", label: "None" },
  { value: "tls", label: "TLS" },
  { value: "reality", label: "Reality" },
];

const FINGERPRINTS = [
  { value: "", label: "—" },
  { value: "chrome", label: "Chrome" },
  { value: "firefox", label: "Firefox" },
  { value: "safari", label: "Safari" },
  { value: "ios", label: "iOS" },
  { value: "android", label: "Android" },
  { value: "edge", label: "Edge" },
  { value: "randomized", label: "Randomized" },
];

const FLOWS = [
  { value: "", label: "—" },
  { value: "xtls-rprx-vision", label: "xtls-rprx-vision" },
];

function ServerModal({
  modal,
  submitError,
  onClose,
  onAdd,
  onSaveEdit,
  onDelete,
}: {
  modal: Exclude<ModalState, { kind: "closed" }>;
  submitError: string | null;
  onClose: () => void;
  onAdd: (name: string, uri: string) => void | Promise<void>;
  onSaveEdit: (id: string, name: string, uri: string) => void | Promise<void>;
  onDelete: (id: string) => void | Promise<void>;
}) {
  const isAdd = modal.kind === "add";
  const isEdit = modal.kind === "edit";
  const isView = modal.kind === "view";
  const editable = isAdd || isEdit;
  const server = !isAdd ? modal.server : null;
  const { t } = useTranslation();

  const TRANSPORT_OPTIONS = TRANSPORT_TYPES.map((o) => ({
    value: o.value,
    label: t(`servers.transport.${o.value}`),
  }));
  const SECURITY_OPTIONS = SECURITY_TYPES.map((o) => ({
    value: o.value,
    label: t(`servers.securityOption.${o.value}`),
  }));
  const FINGERPRINT_OPTIONS = FINGERPRINTS.map((o) => ({
    value: o.value,
    label: t(`servers.fingerprintOption.${o.value || "none"}`),
  }));
  const FLOW_OPTIONS = FLOWS.map((o) => ({
    value: o.value,
    label: o.value
      ? t("servers.flowOption.vision")
      : t("servers.flowOption.none"),
  }));

  const [config, setConfig] = useState<VlessConfig>(() =>
    server ? parseVless(server.uri) : { ...EMPTY_VLESS },
  );
  const [pasteText, setPasteText] = useState("");
  const [pasteError, setPasteError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  function update<K extends keyof VlessConfig>(key: K, value: VlessConfig[K]) {
    setConfig((prev) => ({ ...prev, [key]: value }));
  }

  const flowOk = config.type === "tcp" && config.security !== "none";
  useEffect(() => {
    if (!flowOk && config.flow) update("flow", "");
  }, [flowOk]);

  function parsePaste() {
    const trimmed = pasteText.trim();
    if (!trimmed) return;
    if (!trimmed.startsWith("vless://")) {
      setPasteError(t("servers.errExpectedVless"));
      return;
    }
    const parsed = parseVless(trimmed);
    if (!parsed.host || !parsed.uuid) {
      setPasteError(t("servers.errParse"));
      return;
    }
    setConfig(parsed);
    setPasteText("");
    setPasteError(null);
  }

  const computedUri = buildVless(config);
  const valid = !!config.name.trim() && !!config.uuid.trim() && !!config.host.trim();

  async function submit() {
    if (!valid || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      if (isAdd) await onAdd(config.name.trim(), computedUri);
      else if (isEdit && server)
        await onSaveEdit(server.id, config.name.trim(), computedUri);
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  async function deleteServer() {
    if (!server || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      await onDelete(server.id);
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  function copyUri() {
    if (!computedUri) return;
    void navigator.clipboard.writeText(computedUri);
  }

  const title = isAdd
    ? t("servers.addManualServer")
    : isEdit
      ? t("servers.editServer")
      : t("servers.serverDetails");

  return (
    <motion.div
      // initial={false} skips the enter animation: backdrop is fully
      // visible immediately, avoiding Chromium's 1-frame backdrop-filter
      // compositor lag. Exit fades out smoothly with the inner modal.
      initial={false}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.18, ease: SNAP_EASE }}
      className="fixed inset-0 z-50 flex items-center justify-center"
    >
      <button
        onClick={onClose}
        aria-label={t("servers.close")}
        className="absolute inset-0 cursor-default bg-bg-0/70 backdrop-blur-md"
      />
      <motion.div
        className="glass-elevated relative z-10 flex max-h-[88vh] w-[600px] flex-col rounded-2xl"
        initial={{ scale: 0.96, y: 8 }}
        animate={{ scale: 1, y: 0 }}
        exit={{ scale: 0.96, y: 8 }}
        transition={{ duration: 0.22, ease: SNAP_EASE }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-white/[0.08] px-6 py-5">
          <div className="flex items-baseline gap-2">
            <h2 className="text-[16px] font-semibold tracking-tight">
              {title}
            </h2>
            {server && (
              <span
                className={cn(
                  "rounded px-1.5 py-px text-[8px] font-bold uppercase tracking-[0.16em]",
                  server.origin === MANUAL_ORIGIN
                    ? "bg-warn/15 text-warn"
                    : "bg-white/[0.08] text-white/55",
                )}
              >
                {server.origin === MANUAL_ORIGIN ? t("servers.manualBadge") : t("servers.subscription")}
              </span>
            )}
          </div>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-white/55 transition-colors hover:bg-white/[0.06] hover:text-white"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex flex-col gap-5 overflow-y-auto px-6 py-5">
          {isAdd && (
            <div className="flex flex-col gap-2 rounded-lg border border-white/[0.08] bg-white/[0.03] p-3">
              <span className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/45">
                {t("servers.quickPaste")}
              </span>
              <div className="flex gap-2">
                <input
                  value={pasteText}
                  onChange={(e) => {
                    setPasteText(e.target.value);
                    setPasteError(null);
                  }}
                  placeholder={t("servers.pastePlaceholder")}
                  className="flex-1 rounded-lg border border-white/15 bg-white/[0.04] px-3 py-2 font-mono text-[11px] text-white placeholder:text-white/35 focus:border-accent-start/50 focus:bg-white/[0.06] focus:outline-none"
                />
                <button
                  onClick={parsePaste}
                  disabled={!pasteText.trim()}
                  className="rounded-lg bg-white/[0.08] px-3 py-2 text-[11px] font-semibold text-white transition-colors hover:bg-white/[0.14] disabled:opacity-40"
                >
                  {t("servers.parse")}
                </button>
              </div>
              {pasteError && (
                <span className="text-[10px] text-danger">{pasteError}</span>
              )}
            </div>
          )}

          <Section title={t("servers.sectionBasics")}>
            <Field label={t("servers.name")}>
              <TextInput
                value={config.name}
                onChange={(v) => update("name", v)}
                disabled={!editable}
                placeholder={t("servers.namePlaceholder")}
              />
            </Field>
            <Field label={t("servers.uuid")}>
              <TextInput
                value={config.uuid}
                onChange={(v) => update("uuid", v)}
                disabled={!editable}
                placeholder="00000000-0000-0000-0000-000000000000"
                mono
              />
            </Field>
            <div className="grid grid-cols-3 gap-3">
              <div className="col-span-2">
                <Field label={t("servers.address")}>
                  <TextInput
                    value={config.host}
                    onChange={(v) => update("host", v)}
                    disabled={!editable}
                    placeholder={t("servers.addressPlaceholder")}
                  />
                </Field>
              </div>
              <Field label={t("servers.port")}>
                <TextInput
                  value={config.port}
                  onChange={(v) => update("port", v.replace(/[^\d]/g, ""))}
                  disabled={!editable}
                  placeholder="443"
                  mono
                />
              </Field>
            </div>
          </Section>

          <Section title={t("servers.sectionTransport")}>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("servers.type")}>
                <Dropdown
                  value={config.type}
                  onChange={(v) => update("type", v)}
                  disabled={!editable}
                  options={TRANSPORT_OPTIONS}
                />
              </Field>
              <Field label={t("servers.flow")}>
                <Dropdown
                  value={config.flow}
                  onChange={(v) => update("flow", v)}
                  disabled={!editable || !flowOk}
                  options={FLOW_OPTIONS}
                />
                {!flowOk && (
                  <p className="mt-1 text-[11px] text-white/40">
                    {t("servers.flowIncompatible")}
                  </p>
                )}
              </Field>
            </div>
            <AnimatePresence initial={false}>
              {(config.type === "ws" ||
                config.type === "httpupgrade" ||
                config.type === "xhttp") && (
                <Reveal key="path">
                  <Field label={t("servers.path")}>
                    <TextInput
                      value={config.path}
                      onChange={(v) => update("path", v)}
                      disabled={!editable}
                      placeholder={t("servers.pathPlaceholder")}
                      mono
                    />
                  </Field>
                </Reveal>
              )}
              {config.type === "grpc" && (
                <Reveal key="serviceName">
                  <Field label={t("servers.serviceName")}>
                    <TextInput
                      value={config.serviceName}
                      onChange={(v) => update("serviceName", v)}
                      disabled={!editable}
                      placeholder={t("servers.serviceNamePlaceholder")}
                      mono
                    />
                  </Field>
                </Reveal>
              )}
              {(config.type === "xhttp" || config.type === "grpc") && (
                <Reveal key="mode">
                  <Field label={t("servers.mode")}>
                    <TextInput
                      value={config.mode}
                      onChange={(v) => update("mode", v)}
                      disabled={!editable}
                    />
                  </Field>
                </Reveal>
              )}
              {(config.type === "ws" ||
                config.type === "httpupgrade" ||
                config.type === "xhttp") && (
                <Reveal key="hostHeader">
                  <Field label={t("servers.hostHeader")}>
                    <TextInput
                      value={config.hostHeader}
                      onChange={(v) => update("hostHeader", v)}
                      disabled={!editable}
                      placeholder={t("servers.hostHeaderPlaceholder")}
                    />
                  </Field>
                </Reveal>
              )}
              {config.type === "mkcp" && (
                <Reveal key="mkcp">
                  <div className="flex flex-col gap-3">
                    <Field label={t("servers.seed")}>
                      <TextInput
                        value={config.seed}
                        onChange={(v) => update("seed", v)}
                        disabled={!editable}
                      />
                    </Field>
                    <Field label={t("servers.headerType")}>
                      <TextInput
                        value={config.headerType}
                        onChange={(v) => update("headerType", v)}
                        disabled={!editable}
                      />
                    </Field>
                  </div>
                </Reveal>
              )}
              {config.type === "quic" && (
                <Reveal key="quic">
                  <div className="flex flex-col gap-3">
                    <Field label={t("servers.quicSecurity")}>
                      <TextInput
                        value={config.quicSecurity}
                        onChange={(v) => update("quicSecurity", v)}
                        disabled={!editable}
                      />
                    </Field>
                    <Field label={t("servers.quicKey")}>
                      <TextInput
                        value={config.key}
                        onChange={(v) => update("key", v)}
                        disabled={!editable}
                        mono
                      />
                    </Field>
                  </div>
                </Reveal>
              )}
            </AnimatePresence>
          </Section>

          <Section title={t("servers.sectionSecurity")}>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("servers.security")}>
                <Dropdown
                  value={config.security}
                  onChange={(v) => update("security", v)}
                  disabled={!editable}
                  options={SECURITY_OPTIONS}
                />
              </Field>
              <Field label={t("servers.fingerprint")}>
                <Dropdown
                  value={config.fingerprint}
                  onChange={(v) => update("fingerprint", v)}
                  disabled={!editable || config.security === "none"}
                  options={FINGERPRINT_OPTIONS}
                />
              </Field>
            </div>
            <AnimatePresence initial={false}>
              {config.security !== "none" && (
                <Reveal key="sni">
                  <Field label={t("servers.sni")}>
                    <TextInput
                      value={config.sni}
                      onChange={(v) => update("sni", v)}
                      disabled={!editable}
                      placeholder={t("servers.sniPlaceholder")}
                    />
                  </Field>
                </Reveal>
              )}
              {config.security === "reality" && (
                <Reveal key="reality-keys">
                  <div className="flex flex-col gap-3">
                    <Field label={t("servers.publicKey")}>
                      <TextInput
                        value={config.publicKey}
                        onChange={(v) => update("publicKey", v)}
                        disabled={!editable}
                        placeholder={t("servers.publicKeyPlaceholder")}
                        mono
                      />
                    </Field>
                    <Field label={t("servers.shortId")}>
                      <TextInput
                        value={config.shortId}
                        onChange={(v) => update("shortId", v)}
                        disabled={!editable}
                        placeholder={t("servers.shortIdPlaceholder")}
                        mono
                      />
                    </Field>
                  </div>
                </Reveal>
              )}
            </AnimatePresence>
          </Section>

          <Section title={t("servers.sectionRawUrl")}>
            <div className="relative">
              <textarea
                value={computedUri}
                readOnly
                rows={4}
                className="w-full resize-none rounded-lg border border-white/[0.10] bg-white/[0.02] px-3 py-2 pr-10 font-mono text-[11px] text-white/85 focus:outline-none"
              />
              <button
                onClick={copyUri}
                disabled={!computedUri}
                title={t("servers.copyUrl")}
                className="absolute right-2 top-2 rounded-md p-1.5 text-white/55 transition-colors hover:bg-white/[0.08] hover:text-white disabled:opacity-30"
              >
                <Copy className="h-3.5 w-3.5" />
              </button>
            </div>
          </Section>

          {server && isView && (
            <Section title={t("servers.sectionMetadata")}>
              <div className="flex flex-col gap-2 rounded-lg border border-white/[0.08] bg-white/[0.02] p-3">
                <Meta
                  label={t("servers.origin")}
                  value={
                    server.origin === MANUAL_ORIGIN
                      ? t("servers.manualEntry")
                      : t("servers.subscriptionOrigin", { name: server.origin })
                  }
                />
                <Meta
                  label={t("servers.latency")}
                  value={
                    server.latencyMs > 0 ? `${server.latencyMs} ms` : "—"
                  }
                />
                <Meta label={t("servers.address")} value={server.address} mono />
                <Meta
                  label={t("servers.favouriteMeta")}
                  value={server.favorite ? t("servers.yes") : t("servers.no")}
                />
              </div>
            </Section>
          )}
        </div>

        {submitError && (
          <div
            role="alert"
            className="mx-6 mb-3 flex items-start gap-2 rounded-lg border border-danger/40 bg-danger/[0.08] px-3 py-2 text-[12px] text-danger"
          >
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span className="break-words">{submitError}</span>
          </div>
        )}

        <div className="flex items-center justify-between gap-3 border-t border-white/[0.08] px-6 py-4">
          {isEdit && server ? (
            <button
              onClick={() => void deleteServer()}
              disabled={submitting}
              className="flex items-center gap-1.5 rounded-lg border border-danger/40 bg-danger/[0.10] px-3 py-2 text-[12px] font-medium text-danger transition-colors hover:bg-danger/[0.20] disabled:opacity-50"
            >
              <Trash2 className="h-3.5 w-3.5" />
              Delete
            </button>
          ) : (
            <span />
          )}

          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="rounded-lg px-4 py-2 text-[12px] font-medium text-white/65 transition-colors hover:bg-white/[0.06] hover:text-white"
            >
              {isView ? t("servers.close") : t("servers.cancel")}
            </button>
            {editable && (
              <button
                onClick={() => void submit()}
                disabled={!valid || submitting}
                className="rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-4 py-2 text-[12px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-all hover:shadow-[0_0_22px_rgba(120,200,255,0.45)] disabled:opacity-40 disabled:shadow-none"
              >
                {submitting ? (isAdd ? t("servers.adding") : t("servers.saving")) : isAdd ? t("servers.add") : t("servers.save")}
              </button>
            )}
          </div>
        </div>
      </motion.div>
    </motion.div>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3">
      <span className="text-[10px] font-semibold uppercase tracking-[0.18em] text-white/45">
        {title}
      </span>
      <div className="flex flex-col gap-3">{children}</div>
    </div>
  );
}

function Reveal({ children }: { children: React.ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, height: 0 }}
      animate={{ opacity: 1, height: "auto" }}
      exit={{ opacity: 0, height: 0 }}
      transition={{ duration: 0.22, ease: SNAP_EASE }}
      style={{ overflow: "hidden" }}
    >
      {children}
    </motion.div>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-[9px] font-medium uppercase tracking-[0.18em] text-white/40">
        {label}
      </span>
      {children}
    </label>
  );
}

function TextInput({
  value,
  onChange,
  disabled,
  placeholder,
  mono,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  placeholder?: string;
  mono?: boolean;
}) {
  return (
    <input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      disabled={disabled}
      placeholder={placeholder}
      className={cn(
        "w-full rounded-lg border border-white/15 bg-white/[0.04] px-3 py-2 text-[12px] text-white placeholder:text-white/35 focus:border-accent-start/50 focus:bg-white/[0.06] focus:outline-none disabled:opacity-60",
        mono && "font-mono tabular-nums text-[11px]",
      )}
    />
  );
}

function Meta({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <span className="text-[10px] font-medium uppercase tracking-[0.14em] text-white/40">
        {label}
      </span>
      <span
        className={cn(
          "text-[11px] text-white/85",
          mono && "font-mono tabular-nums",
        )}
      >
        {value}
      </span>
    </div>
  );
}
