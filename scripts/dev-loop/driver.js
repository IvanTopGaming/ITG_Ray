#!/usr/bin/env node
// CDP driver — connects to the WebView2 inside ITGRay.exe over the
// SSH-forwarded localhost:9223 endpoint. Subcommands:
//
//   driver.js screenshot <out.png>     full-page screenshot
//   driver.js console                  dump recent console messages
//   driver.js click <selector>         click a CSS selector
//   driver.js text <selector>          read text content of selector
//   driver.js eval '<js-expr>'         evaluate JS, print result as JSON
//   driver.js wait <selector> [ms]     wait until selector appears (default 5s)
//   driver.js status                   summary: title + url + visible body text
//
// All commands re-attach freshly (no persistent state across invocations).
"use strict";
const { chromium } = require("playwright");
const fs = require("fs");

const ENDPOINT = process.env.ITGRAY_CDP_URL || "http://localhost:9223";

async function attach() {
  const browser = await chromium.connectOverCDP(ENDPOINT);
  const ctxs = browser.contexts();
  if (!ctxs.length) throw new Error("no browser contexts available — is ITGRay.exe running with debug port?");
  const pages = ctxs[0].pages();
  if (!pages.length) throw new Error("no pages — webview not initialised yet");
  const page = pages[0];
  return { browser, page };
}

async function main() {
  const [, , cmd, ...args] = process.argv;
  if (!cmd) {
    console.error("usage: driver.js <screenshot|console|click|text|eval|wait|status> [args]");
    process.exit(2);
  }
  const { browser, page } = await attach();

  // Tap into console to surface page errors no matter the command.
  const consoleLog = [];
  page.on("console", (msg) => consoleLog.push(`[${msg.type()}] ${msg.text()}`));
  page.on("pageerror", (err) => consoleLog.push(`[error] ${err.message}`));

  try {
    switch (cmd) {
      case "screenshot": {
        const out = args[0] || "screenshot.png";
        await page.screenshot({ path: out, fullPage: false });
        console.log(`wrote ${out} (${page.url()})`);
        break;
      }
      case "console": {
        // Eval to flush pending logs, then dump.
        await page.evaluate(() => 1);
        await new Promise((r) => setTimeout(r, 200));
        consoleLog.forEach((l) => console.log(l));
        break;
      }
      case "click": {
        const sel = args[0];
        if (!sel) throw new Error("click: selector required");
        await page.click(sel, { timeout: 5000 });
        console.log(`clicked: ${sel}`);
        break;
      }
      case "text": {
        const sel = args[0];
        if (!sel) throw new Error("text: selector required");
        const t = await page.textContent(sel);
        console.log(t ?? "");
        break;
      }
      case "eval": {
        const expr = args.join(" ");
        if (!expr) throw new Error("eval: expression required");
        const v = await page.evaluate(`(${expr})`);
        console.log(typeof v === "object" ? JSON.stringify(v, null, 2) : String(v));
        break;
      }
      case "wait": {
        const sel = args[0];
        const timeoutMs = parseInt(args[1] || "5000", 10);
        await page.waitForSelector(sel, { timeout: timeoutMs });
        console.log(`appeared: ${sel}`);
        break;
      }
      case "status": {
        const title = await page.title();
        const url = page.url();
        const bodyText = (await page.textContent("body").catch(() => "")) || "";
        const trimmed = bodyText.replace(/\s+/g, " ").trim().slice(0, 400);
        console.log(JSON.stringify({ title, url, body: trimmed }, null, 2));
        break;
      }
      default:
        throw new Error(`unknown command: ${cmd}`);
    }
    if (consoleLog.length && cmd !== "console") {
      console.error("--- console messages during this command ---");
      consoleLog.forEach((l) => console.error(l));
    }
  } finally {
    await browser.close().catch(() => {});
  }
}

main().catch((err) => {
  console.error("driver error:", err.message);
  process.exit(1);
});
