import type { TFunction } from "i18next";
import type { Sub } from "./subsAdapter";

export function subscriptionSyncMessage(sub: Sub, t: TFunction): string | undefined {
  const message = sub.lastSyncMessage;
  if (!message) return undefined;
  if (sub.status === "error") {
    if (message === "unrecognized subscription format / empty response" || message === "empty subscription body") {
      return t("subscriptions.unsupportedResponse");
    }
    const match = /^no usable VLESS servers: invalid=(\d+) skipped=(\d+) protocols=(.*)$/.exec(message);
    if (match) {
      const summary = t("subscriptions.noUsableServers", { invalid: match[1], skipped: match[2] });
      const protocols = match[3].split(",").filter(Boolean).map((protocol) =>
        protocol === "xray-complex" ? t("subscriptions.advancedXray") : protocol,
      ).join(", ");
      return protocols ? `${summary} ${t("subscriptions.unsupportedProtocols", { protocols })}` : summary;
    }
    return message;
  }
  if (sub.status === "ok") {
    const match = /^imported=(\d+) invalid=(\d+) skipped=(\d+)$/.exec(message);
    if (match && (Number(match[2]) > 0 || Number(match[3]) > 0)) {
      return t("subscriptions.partialImport", { imported: match[1], invalid: match[2], skipped: match[3] });
    }
  }
  return undefined;
}
