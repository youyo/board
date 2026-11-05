import { describe, expect, it } from "vitest";
import { mockOutboundFetch } from "../helpers/mock-fetch";
import { callWorker } from "../helpers/worker";

describe("E2E: /healthz", () => {
  it("Access の JWT なしで {status:'ok'} を返す(JWKS も取りに行かない)", async () => {
    const m = mockOutboundFetch({});
    const res = await callWorker("/healthz");
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ status: "ok" });
    expect(m.calls).toHaveLength(0);
  });
});
