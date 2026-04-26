import React, { useEffect, useMemo, useRef, useState } from "react";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import {
  ChevronDown,
  Copy,
  Eye,
  Pencil,
  Plus,
  Search,
  Star,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import { cn } from "@/lib/cn";

type Origin = "subscription" | "manual";
type Sort = "latency" | "name";
type ProbeState = "idle" | "probing" | "ok" | "error";

interface FakeSub {
  id: string;
  name: string;
  syncedAgo: string;
}

interface FakeServer {
  id: string;
  subId: string | null;
  origin: Origin;
  flag: string;
  city: string;
  code: string;
  pingMs: number;
  favorite: boolean;
  vlessUri: string;
}

const SUBS: FakeSub[] = [
  { id: "sub-1", name: "Main provider", syncedAgo: "2 min ago" },
  { id: "sub-2", name: "Backup provider", syncedAgo: "14 min ago" },
];

const SAMPLE_URI = (city: string) =>
  `vless://550e8400-e29b-41d4-a716-446655440000@host.example.com:443?type=ws&path=%2Fws&security=reality&sni=www.cloudflare.com&fp=chrome&pbk=PUBKEY&sid=0011#${encodeURIComponent(city)}`;

const INITIAL_SERVERS: FakeServer[] = [
  // Manual (user-added)
  {
    id: "m1",
    subId: null,
    origin: "manual",
    flag: "🇨🇦",
    city: "Toronto",
    code: "manual",
    pingMs: 92,
    favorite: true,
    vlessUri: SAMPLE_URI("Toronto"),
  },
  // Main provider
  { id: "s1",  subId: "sub-1", origin: "subscription", flag: "🇳🇱", city: "Amsterdam",   code: "NL-AMS-03", pingMs: 12,  favorite: true,  vlessUri: SAMPLE_URI("Amsterdam") },
  { id: "s2",  subId: "sub-1", origin: "subscription", flag: "🇩🇪", city: "Frankfurt",   code: "DE-FRA-01", pingMs: 28,  favorite: false, vlessUri: SAMPLE_URI("Frankfurt") },
  { id: "s3",  subId: "sub-1", origin: "subscription", flag: "🇫🇮", city: "Helsinki",    code: "FI-HEL-02", pingMs: 34,  favorite: true,  vlessUri: SAMPLE_URI("Helsinki") },
  { id: "s4",  subId: "sub-1", origin: "subscription", flag: "🇸🇪", city: "Stockholm",   code: "SE-STO-01", pingMs: 41,  favorite: false, vlessUri: SAMPLE_URI("Stockholm") },
  { id: "s5",  subId: "sub-1", origin: "subscription", flag: "🇬🇧", city: "London",      code: "GB-LON-04", pingMs: 47,  favorite: false, vlessUri: SAMPLE_URI("London") },
  { id: "s6",  subId: "sub-1", origin: "subscription", flag: "🇫🇷", city: "Paris",       code: "FR-PAR-02", pingMs: 38,  favorite: false, vlessUri: SAMPLE_URI("Paris") },
  { id: "s7",  subId: "sub-1", origin: "subscription", flag: "🇨🇭", city: "Zurich",      code: "CH-ZRH-01", pingMs: 52,  favorite: false, vlessUri: SAMPLE_URI("Zurich") },
  { id: "s8",  subId: "sub-1", origin: "subscription", flag: "🇪🇸", city: "Madrid",      code: "ES-MAD-01", pingMs: 64,  favorite: false, vlessUri: SAMPLE_URI("Madrid") },
  // Backup
  { id: "s9",  subId: "sub-2", origin: "subscription", flag: "🇺🇸", city: "New York",    code: "US-NYC-04", pingMs: 112, favorite: false, vlessUri: SAMPLE_URI("New York") },
  { id: "s10", subId: "sub-2", origin: "subscription", flag: "🇺🇸", city: "Los Angeles", code: "US-LAX-02", pingMs: 158, favorite: false, vlessUri: SAMPLE_URI("Los Angeles") },
  { id: "s11", subId: "sub-2", origin: "subscription", flag: "🇯🇵", city: "Tokyo",       code: "JP-TYO-01", pingMs: 235, favorite: false, vlessUri: SAMPLE_URI("Tokyo") },
  { id: "s12", subId: "sub-2", origin: "subscription", flag: "🇸🇬", city: "Singapore",   code: "SG-SIN-03", pingMs: 198, favorite: false, vlessUri: SAMPLE_URI("Singapore") },
];

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
  | { kind: "view"; server: FakeServer }
  | { kind: "edit"; server: FakeServer };

export function Servers() {
  const [servers, setServers] = useState<FakeServer[]>(INITIAL_SERVERS);
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<Sort>("latency");
  const [favoritesFirst, setFavoritesFirst] = useState(true);
  const [activeServerId, setActiveServerId] = useState<string | null>("s1");
  const [probe, setProbe] = useState<Map<string, ProbeState>>(new Map());
  const [modal, setModal] = useState<ModalState>({ kind: "closed" });

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return servers.filter(
      (s) =>
        !q ||
        s.city.toLowerCase().includes(q) ||
        s.code.toLowerCase().includes(q),
    );
  }, [servers, search]);

  const grouped = useMemo(() => {
    const out: Array<{
      id: string;
      label: string;
      hint: string | null;
      origin: Origin;
      rows: FakeServer[];
    }> = [];

    const sortRows = (rows: FakeServer[]) =>
      [...rows].sort((a, b) => {
        if (favoritesFirst && a.favorite !== b.favorite) {
          return a.favorite ? -1 : 1;
        }
        if (sort === "latency") return a.pingMs - b.pingMs;
        return a.city.localeCompare(b.city);
      });

    const manualRows = filtered.filter((s) => s.origin === "manual");
    if (manualRows.length) {
      out.push({
        id: MANUAL_GROUP_ID,
        label: "Manual",
        hint: `${manualRows.length} server${manualRows.length === 1 ? "" : "s"}`,
        origin: "manual",
        rows: sortRows(manualRows),
      });
    }

    for (const sub of SUBS) {
      const rows = filtered.filter((s) => s.subId === sub.id);
      if (rows.length) {
        out.push({
          id: sub.id,
          label: sub.name,
          hint: `synced ${sub.syncedAgo}`,
          origin: "subscription",
          rows: sortRows(rows),
        });
      }
    }
    return out;
  }, [filtered, sort, favoritesFirst]);

  function setProbing(id: string) {
    setProbe((prev) => {
      const m = new Map(prev);
      m.set(id, "probing");
      return m;
    });
  }

  function finishProbe(id: string) {
    setServers((prev) =>
      prev.map((s) =>
        s.id === id
          ? {
              ...s,
              pingMs: Math.max(
                8,
                Math.round(s.pingMs * (0.8 + Math.random() * 0.5)),
              ),
            }
          : s,
      ),
    );
    setProbe((prev) => {
      const m = new Map(prev);
      m.set(id, "ok");
      return m;
    });
  }

  function probeOne(id: string) {
    setProbing(id);
    setTimeout(() => finishProbe(id), 300 + Math.random() * 600);
  }

  function probeAll() {
    servers.forEach((s, i) => {
      setTimeout(() => {
        setProbing(s.id);
        setTimeout(() => finishProbe(s.id), 300 + Math.random() * 600);
      }, i * 80);
    });
  }

  function toggleFavorite(id: string) {
    setServers((prev) =>
      prev.map((s) => (s.id === id ? { ...s, favorite: !s.favorite } : s)),
    );
  }

  function selectServer(id: string) {
    setActiveServerId(id);
  }

  function handleAddManual(name: string, uri: string) {
    const id = `m-${Date.now()}`;
    setServers((prev) => [
      {
        id,
        subId: null,
        origin: "manual",
        flag: "🌐",
        city: name,
        code: "manual",
        pingMs: 50 + Math.floor(Math.random() * 150),
        favorite: false,
        vlessUri: uri,
      },
      ...prev,
    ]);
    setModal({ kind: "closed" });
  }

  function handleSaveEdit(id: string, name: string, uri: string) {
    setServers((prev) =>
      prev.map((s) => (s.id === id ? { ...s, city: name, vlessUri: uri } : s)),
    );
    setModal({ kind: "closed" });
  }

  function handleDelete(id: string) {
    setServers((prev) => prev.filter((s) => s.id !== id));
    if (activeServerId === id) setActiveServerId(null);
    setModal({ kind: "closed" });
  }

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
          <h1 className="text-[22px] font-semibold tracking-tight">Servers</h1>
          <div className="flex items-center gap-2">
            <SearchInput value={search} onChange={setSearch} />
            <SortToggle value={sort} onChange={setSort} />
            <FavoritesToggle
              value={favoritesFirst}
              onChange={setFavoritesFirst}
            />
            <AddServerButton onClick={() => setModal({ kind: "add" })} />
            <ProbeAllButton onClick={probeAll} />
          </div>
        </motion.div>

        {grouped.length === 0 ? (
          <motion.div
            variants={itemVariants}
            className="glass-regular rounded-2xl p-10 text-center text-[13px] text-white/55"
          >
            Nothing matches “{search}”.
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
                      manual
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
                  {group.rows.map((server, idx) => (
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
                        active={server.id === activeServerId}
                        probing={probe.get(server.id) === "probing"}
                        isLast={idx === group.rows.length - 1}
                        onSelect={() => selectServer(server.id)}
                        onToggleFavorite={() => toggleFavorite(server.id)}
                        onProbe={() => probeOne(server.id)}
                        onAction={() =>
                          setModal({
                            kind:
                              server.origin === "manual" ? "edit" : "view",
                            server,
                          })
                        }
                      />
                    </motion.div>
                  ))}
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
            onClose={() => setModal({ kind: "closed" })}
            onAdd={handleAddManual}
            onSaveEdit={handleSaveEdit}
            onDelete={handleDelete}
          />
        )}
      </AnimatePresence>
    </>
  );
}

function SearchInput({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <div className="glass-regular flex items-center gap-2 rounded-lg px-3 py-1.5">
      <Search className="h-3.5 w-3.5 text-white/45" />
      <input
        type="text"
        placeholder="Search city or code"
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
          <span className="relative z-10">{s}</span>
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
  return (
    <button
      onClick={() => onChange(!value)}
      className={cn(
        "glass-regular flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[11px] font-medium transition-colors duration-instant ease-snap",
        value ? "text-warn" : "text-white/55 hover:text-white",
      )}
      title="Favourites first"
    >
      <Star className={cn("h-3.5 w-3.5", value && "fill-warn stroke-warn")} />
    </button>
  );
}

function AddServerButton({ onClick }: { onClick: () => void }) {
  return (
    <motion.button
      onClick={onClick}
      whileHover={{ y: -1 }}
      whileTap={{ scale: 0.96 }}
      transition={{ duration: 0.15, ease: SNAP_EASE }}
      className="glass-regular flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[11px] font-semibold text-white transition-colors duration-instant ease-snap hover:!border-white/30 hover:bg-white/[0.08]"
    >
      <Plus className="h-3.5 w-3.5" />
      Add server
    </motion.button>
  );
}

function ProbeAllButton({ onClick }: { onClick: () => void }) {
  return (
    <motion.button
      onClick={onClick}
      whileHover={{ y: -1 }}
      whileTap={{ scale: 0.96 }}
      transition={{ duration: 0.15, ease: SNAP_EASE }}
      className="flex items-center gap-1.5 rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-3 py-1.5 text-[11px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-shadow duration-instant ease-snap hover:shadow-[0_0_22px_rgba(120,200,255,0.45)]"
    >
      <Zap className="h-3.5 w-3.5" />
      Probe all
    </motion.button>
  );
}

function ServerRow({
  server,
  active,
  probing,
  isLast,
  onSelect,
  onToggleFavorite,
  onProbe,
  onAction,
}: {
  server: FakeServer;
  active: boolean;
  probing: boolean;
  isLast: boolean;
  onSelect: () => void;
  onToggleFavorite: () => void;
  onProbe: () => void;
  onAction: () => void;
}) {
  const pingColor =
    server.pingMs < 25
      ? "text-success"
      : server.pingMs < 60
        ? "text-warn"
        : server.pingMs < 120
          ? "text-white/65"
          : "text-danger";

  const ActionIcon = server.origin === "manual" ? Pencil : Eye;
  const actionTitle = server.origin === "manual" ? "Edit" : "View details";

  return (
    <button
      onClick={onSelect}
      className={cn(
        "group relative flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors duration-instant ease-snap",
        !isLast && "border-b border-white/[0.05]",
        active ? "bg-success/[0.06]" : "hover:bg-white/[0.03]",
      )}
    >
      {active && (
        <motion.div
          layoutId="servers-active-ring"
          className="pointer-events-none absolute inset-0 border-y-2 border-success/55 bg-success/[0.02]"
          transition={{ type: "spring", stiffness: 380, damping: 32 }}
        />
      )}

      <span className="relative z-10 inline-flex h-2 w-2 items-center justify-center">
        <span
          className={cn(
            "h-2 w-2 rounded-full",
            active
              ? "bg-success shadow-[0_0_6px_rgba(0,230,118,0.7)]"
              : "bg-white/20",
          )}
        />
      </span>

      <span className="relative z-10 text-[20px] leading-none">
        {server.flag}
      </span>

      <div className="relative z-10 flex min-w-0 flex-1 items-baseline gap-3">
        <span className="text-[13px] font-medium">{server.city}</span>
        <span className="font-mono text-[10px] tabular-nums text-white/40">
          {server.code}
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
            Active
          </motion.span>
        )}
      </div>

      <button
        onClick={(e) => {
          e.stopPropagation();
          onToggleFavorite();
        }}
        className="relative z-10 rounded p-1 hover:bg-white/[0.06]"
        title={server.favorite ? "Unfavourite" : "Favourite"}
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
        title="Re-probe latency"
      >
        {probing ? "probing…" : `${server.pingMs} ms`}
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
    </button>
  );
}

// ─── VLESS parsing/building ────────────────────────────────────────────

interface VlessConfig {
  name: string;
  uuid: string;
  host: string;
  port: string; // string for input ergonomics; validated on save
  type: string; // tcp, ws, grpc, http, h2, kcp, quic
  path: string;
  hostHeader: string;
  security: string; // none, tls, reality
  sni: string;
  fingerprint: string; // chrome, firefox, safari, ios, android, edge, 360, qq, random, randomized
  publicKey: string;
  shortId: string;
  flow: string; // empty, xtls-rprx-vision
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
};

function parseVless(uri: string): VlessConfig {
  const c: VlessConfig = { ...EMPTY_VLESS };
  if (!uri) return c;
  try {
    // URL constructor only handles a few schemes well — vless:// is RFC-compliant
    const url = new URL(uri.trim());
    if (url.protocol !== "vless:") return c;
    c.uuid = decodeURIComponent(url.username);
    c.host = url.hostname;
    c.port = url.port || "443";
    c.name = decodeURIComponent(url.hash.slice(1));
    const p = url.searchParams;
    c.type = p.get("type") || "tcp";
    c.path = p.get("path") ? decodeURIComponent(p.get("path")!) : "";
    c.hostHeader = p.get("host") || "";
    c.security = p.get("security") || "none";
    c.sni = p.get("sni") || "";
    c.fingerprint = p.get("fp") || "";
    c.publicKey = p.get("pbk") || "";
    c.shortId = p.get("sid") || "";
    c.flow = p.get("flow") || "";
  } catch {
    /* invalid — keep empty */
  }
  return c;
}

function buildVless(c: VlessConfig): string {
  if (!c.uuid || !c.host) return "";
  const params = new URLSearchParams();
  params.set("encryption", "none");
  if (c.flow) params.set("flow", c.flow);
  if (c.type) params.set("type", c.type);
  if (c.path) params.set("path", c.path);
  if (c.hostHeader) params.set("host", c.hostHeader);
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
  { value: "http", label: "HTTP/2" },
  { value: "kcp", label: "mKCP" },
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
  onClose,
  onAdd,
  onSaveEdit,
  onDelete,
}: {
  modal: Exclude<ModalState, { kind: "closed" }>;
  onClose: () => void;
  onAdd: (name: string, uri: string) => void;
  onSaveEdit: (id: string, name: string, uri: string) => void;
  onDelete: (id: string) => void;
}) {
  const isAdd = modal.kind === "add";
  const isEdit = modal.kind === "edit";
  const isView = modal.kind === "view";
  const editable = isAdd || isEdit;
  const server = !isAdd ? modal.server : null;

  const [config, setConfig] = useState<VlessConfig>(() =>
    server ? parseVless(server.vlessUri) : { ...EMPTY_VLESS },
  );
  const [pasteText, setPasteText] = useState("");
  const [pasteError, setPasteError] = useState<string | null>(null);

  function update<K extends keyof VlessConfig>(key: K, value: VlessConfig[K]) {
    setConfig((prev) => ({ ...prev, [key]: value }));
  }

  function parsePaste() {
    const trimmed = pasteText.trim();
    if (!trimmed) return;
    if (!trimmed.startsWith("vless://")) {
      setPasteError("Expected a vless:// URL");
      return;
    }
    const parsed = parseVless(trimmed);
    if (!parsed.host || !parsed.uuid) {
      setPasteError("Could not parse host or UUID");
      return;
    }
    setConfig(parsed);
    setPasteText("");
    setPasteError(null);
  }

  const computedUri = buildVless(config);
  const valid = !!config.name.trim() && !!config.uuid.trim() && !!config.host.trim();

  function submit() {
    if (!valid) return;
    if (isAdd) onAdd(config.name.trim(), computedUri);
    else if (isEdit && server)
      onSaveEdit(server.id, config.name.trim(), computedUri);
  }

  function copyUri() {
    if (!computedUri) return;
    void navigator.clipboard.writeText(computedUri);
  }

  const title = isAdd
    ? "Add manual server"
    : isEdit
      ? "Edit server"
      : "Server details";

  return (
    <motion.div
      className="fixed inset-0 z-50 flex items-center justify-center"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.18, ease: SNAP_EASE }}
    >
      <button
        onClick={onClose}
        aria-label="Close"
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
                  server.origin === "manual"
                    ? "bg-warn/15 text-warn"
                    : "bg-white/[0.08] text-white/55",
                )}
              >
                {server.origin}
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
                Quick paste
              </span>
              <div className="flex gap-2">
                <input
                  value={pasteText}
                  onChange={(e) => {
                    setPasteText(e.target.value);
                    setPasteError(null);
                  }}
                  placeholder="vless://… (paste full URL to fill fields)"
                  className="flex-1 rounded-lg border border-white/15 bg-white/[0.04] px-3 py-2 font-mono text-[11px] text-white placeholder:text-white/35 focus:border-accent-start/50 focus:bg-white/[0.06] focus:outline-none"
                />
                <button
                  onClick={parsePaste}
                  disabled={!pasteText.trim()}
                  className="rounded-lg bg-white/[0.08] px-3 py-2 text-[11px] font-semibold text-white transition-colors hover:bg-white/[0.14] disabled:opacity-40"
                >
                  Parse
                </button>
              </div>
              {pasteError && (
                <span className="text-[10px] text-danger">{pasteError}</span>
              )}
            </div>
          )}

          <Section title="Basics">
            <Field label="Name">
              <TextInput
                value={config.name}
                onChange={(v) => update("name", v)}
                disabled={!editable}
                placeholder="e.g. Provider — Tokyo"
              />
            </Field>
            <Field label="UUID">
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
                <Field label="Address">
                  <TextInput
                    value={config.host}
                    onChange={(v) => update("host", v)}
                    disabled={!editable}
                    placeholder="host.example.com"
                  />
                </Field>
              </div>
              <Field label="Port">
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

          <Section title="Transport">
            <div className="grid grid-cols-2 gap-3">
              <Field label="Type">
                <SelectInput
                  value={config.type}
                  onChange={(v) => update("type", v)}
                  disabled={!editable}
                  options={TRANSPORT_TYPES}
                />
              </Field>
              <Field label="Flow">
                <SelectInput
                  value={config.flow}
                  onChange={(v) => update("flow", v)}
                  disabled={!editable}
                  options={FLOWS}
                />
              </Field>
            </div>
            <AnimatePresence initial={false}>
              {(config.type === "ws" ||
                config.type === "grpc" ||
                config.type === "http") && (
                <Reveal key="path">
                  <Field
                    label={config.type === "grpc" ? "Service name" : "Path"}
                  >
                    <TextInput
                      value={config.path}
                      onChange={(v) => update("path", v)}
                      disabled={!editable}
                      placeholder={
                        config.type === "grpc" ? "grpc-service" : "/path"
                      }
                      mono
                    />
                  </Field>
                </Reveal>
              )}
              {(config.type === "ws" || config.type === "http") && (
                <Reveal key="hostHeader">
                  <Field label="Host header">
                    <TextInput
                      value={config.hostHeader}
                      onChange={(v) => update("hostHeader", v)}
                      disabled={!editable}
                      placeholder="example.com (optional)"
                    />
                  </Field>
                </Reveal>
              )}
            </AnimatePresence>
          </Section>

          <Section title="Security">
            <div className="grid grid-cols-2 gap-3">
              <Field label="Security">
                <SelectInput
                  value={config.security}
                  onChange={(v) => update("security", v)}
                  disabled={!editable}
                  options={SECURITY_TYPES}
                />
              </Field>
              <Field label="Fingerprint">
                <SelectInput
                  value={config.fingerprint}
                  onChange={(v) => update("fingerprint", v)}
                  disabled={!editable || config.security === "none"}
                  options={FINGERPRINTS}
                />
              </Field>
            </div>
            <AnimatePresence initial={false}>
              {config.security !== "none" && (
                <Reveal key="sni">
                  <Field label="SNI / Server name">
                    <TextInput
                      value={config.sni}
                      onChange={(v) => update("sni", v)}
                      disabled={!editable}
                      placeholder="www.cloudflare.com"
                    />
                  </Field>
                </Reveal>
              )}
              {config.security === "reality" && (
                <Reveal key="reality-keys">
                  <div className="flex flex-col gap-3">
                    <Field label="Public key">
                      <TextInput
                        value={config.publicKey}
                        onChange={(v) => update("publicKey", v)}
                        disabled={!editable}
                        placeholder="x25519 public key"
                        mono
                      />
                    </Field>
                    <Field label="Short ID">
                      <TextInput
                        value={config.shortId}
                        onChange={(v) => update("shortId", v)}
                        disabled={!editable}
                        placeholder="0011"
                        mono
                      />
                    </Field>
                  </div>
                </Reveal>
              )}
            </AnimatePresence>
          </Section>

          <Section title="Raw URL">
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
                title="Copy URL"
                className="absolute right-2 top-2 rounded-md p-1.5 text-white/55 transition-colors hover:bg-white/[0.08] hover:text-white disabled:opacity-30"
              >
                <Copy className="h-3.5 w-3.5" />
              </button>
            </div>
          </Section>

          {server && isView && (
            <Section title="Metadata">
              <div className="flex flex-col gap-2 rounded-lg border border-white/[0.08] bg-white/[0.02] p-3">
                <Meta
                  label="Origin"
                  value={
                    server.origin === "manual"
                      ? "Manual entry"
                      : `Subscription · ${server.subId}`
                  }
                />
                <Meta label="Latency" value={`${server.pingMs} ms`} />
                <Meta label="Code" value={server.code} mono />
                <Meta
                  label="Favourite"
                  value={server.favorite ? "Yes" : "No"}
                />
              </div>
            </Section>
          )}
        </div>

        <div className="flex items-center justify-between gap-3 border-t border-white/[0.08] px-6 py-4">
          {isEdit && server ? (
            <button
              onClick={() => onDelete(server.id)}
              className="flex items-center gap-1.5 rounded-lg border border-danger/40 bg-danger/[0.10] px-3 py-2 text-[12px] font-medium text-danger transition-colors hover:bg-danger/[0.20]"
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
              {isView ? "Close" : "Cancel"}
            </button>
            {editable && (
              <button
                onClick={submit}
                disabled={!valid}
                className="rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-4 py-2 text-[12px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-all hover:shadow-[0_0_22px_rgba(120,200,255,0.45)] disabled:opacity-40 disabled:shadow-none"
              >
                {isAdd ? "Add" : "Save"}
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

function SelectInput({
  value,
  onChange,
  disabled,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  options: Array<{ value: string; label: string }>;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const current = options.find((o) => o.value === value);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", handler);
      document.removeEventListener("keydown", esc);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => !disabled && setOpen((o) => !o)}
        disabled={disabled}
        className={cn(
          "flex w-full items-center justify-between rounded-lg border bg-white/[0.04] px-3 py-2 text-left text-[12px] text-white transition-colors duration-instant ease-snap",
          open
            ? "border-accent-start/50 bg-white/[0.06]"
            : "border-white/15 hover:border-white/25 hover:bg-white/[0.05]",
          disabled && "cursor-not-allowed opacity-60",
        )}
      >
        <span className={cn(!current?.label && "text-white/35")}>
          {current?.label ?? value ?? "—"}
        </span>
        <ChevronDown
          className={cn(
            "h-3.5 w-3.5 text-white/45 transition-transform duration-instant ease-snap",
            open && "rotate-180",
          )}
        />
      </button>
      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -4, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -4, scale: 0.98 }}
            transition={{ duration: 0.15, ease: SNAP_EASE }}
            className="absolute left-0 right-0 top-full z-30 mt-1 max-h-[220px] overflow-y-auto rounded-lg border border-white/[0.18] bg-bg-1/95 p-1 shadow-[0_18px_36px_-10px_rgba(0,0,0,0.6)] backdrop-blur-xl"
          >
            {options.map((o) => (
              <button
                key={o.value}
                type="button"
                onClick={() => {
                  onChange(o.value);
                  setOpen(false);
                }}
                className={cn(
                  "flex w-full items-center rounded-md px-2.5 py-1.5 text-left text-[12px] transition-colors duration-instant ease-snap",
                  o.value === value
                    ? "bg-white/[0.12] text-white"
                    : "text-white/75 hover:bg-white/[0.06] hover:text-white",
                )}
              >
                {o.label}
              </button>
            ))}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
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
