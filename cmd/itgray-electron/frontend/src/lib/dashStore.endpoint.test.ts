import { describe, it, expect, vi } from "vitest";

vi.mock("@/lib/itg/runtime", () => ({
  EventsOn: () => () => {},
}));
vi.mock("@/lib/itg/AppService", () => ({
  GetSnapshot: () => Promise.resolve({}),
}));
vi.mock("@/lib/itg/RunService", () => ({
  Connect: () => {},
  Disconnect: () => {},
}));
vi.mock("@/lib/itg/ServersService", () => ({
  TestLatency: () => {},
}));

import { extractEndpoint } from "./dashStore";

describe("extractEndpoint", () => {
  it("pulls socks/http ports from the connected network payload", () => {
    const ep = extractEndpoint({
      status: "connected",
      network: { socksPort: 1080, httpPort: 8888 },
    });
    expect(ep).toEqual({ socksPort: 1080, httpPort: 8888 });
  });

  it("returns null when network is missing or malformed", () => {
    expect(extractEndpoint({ status: "connected" })).toBeNull();
    expect(extractEndpoint({ status: "connected", network: {} })).toBeNull();
  });
});
