// cmd/itgray-electron/src/main/autostart.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { makeAutostart, resolveAutostartPath } from "./autostart";

type FakeAutoLaunch = {
  isEnabled: () => Promise<boolean>;
  enable: () => Promise<void>;
  disable: () => Promise<void>;
};

function makeFake(initial: boolean): FakeAutoLaunch {
  let enabled = initial;
  return {
    isEnabled: async () => enabled,
    enable: async () => {
      enabled = true;
    },
    disable: async () => {
      enabled = false;
    },
  };
}

test("get returns current OS state", async () => {
  const fake = makeFake(true);
  const a = makeAutostart(() => fake as never);
  assert.equal(await a.get(), true);
});

test("set(true) enables", async () => {
  const fake = makeFake(false);
  const a = makeAutostart(() => fake as never);
  await a.set(true);
  assert.equal(await fake.isEnabled(), true);
});

test("set(false) disables", async () => {
  const fake = makeFake(true);
  const a = makeAutostart(() => fake as never);
  await a.set(false);
  assert.equal(await fake.isEnabled(), false);
});

test("reconcile is no-op when desired matches OS state", async () => {
  const fake = makeFake(true);
  let enableCalls = 0;
  let disableCalls = 0;
  const wrap: FakeAutoLaunch = {
    isEnabled: fake.isEnabled,
    enable: async () => {
      enableCalls++;
      await fake.enable();
    },
    disable: async () => {
      disableCalls++;
      await fake.disable();
    },
  };
  const a = makeAutostart(() => wrap as never);
  await a.reconcile(true);
  assert.equal(enableCalls, 0);
  assert.equal(disableCalls, 0);
});

test("reconcile applies when desired differs from OS state", async () => {
  const fake = makeFake(false);
  const a = makeAutostart(() => fake as never);
  await a.reconcile(true);
  assert.equal(await fake.isEnabled(), true);
});

test("resolveAutostartPath prefers APPIMAGE when set", () => {
  assert.equal(
    resolveAutostartPath("/home/u/Applications/ITGRay.AppImage", "/tmp/.mount_ITGRayXXXX/itgray-electron"),
    "/home/u/Applications/ITGRay.AppImage",
  );
});

test("resolveAutostartPath falls back to exe path when APPIMAGE unset", () => {
  assert.equal(resolveAutostartPath(undefined, "/opt/ITGRay/itgray-electron"), "/opt/ITGRay/itgray-electron");
});

test("resolveAutostartPath ignores empty APPIMAGE", () => {
  assert.equal(resolveAutostartPath("", "/opt/ITGRay/itgray-electron"), "/opt/ITGRay/itgray-electron");
});
