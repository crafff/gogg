import { afterEach, describe, expect, it, vi } from "vitest";

import { fetcher } from "./fetcher";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("GraphQL fetcher", () => {
  it("preserves rate-limit code and retry duration", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            errors: [
              {
                message: "retry later",
                extensions: {
                  code: "RATE_LIMITED",
                  retryAfterSeconds: 47,
                  rateLimitScope: "IP",
                },
              },
            ],
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      ),
    );

    const request = fetcher<{ ok: boolean }, Record<string, never>>(
      "query { ok }",
    );

    await expect(request()).rejects.toMatchObject({
      code: "RATE_LIMITED",
      retryAfterSeconds: 47,
      rateLimitScope: "IP",
      message: "retry later",
    });
  });

  it("classifies a browser connection failure as a network error", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("fetch failed")),
    );

    const request = fetcher<{ ok: boolean }, Record<string, never>>(
      "query { ok }",
    );

    await expect(request()).rejects.toMatchObject({
      code: "NETWORK_ERROR",
      retryAfterSeconds: null,
      rateLimitScope: null,
    });
  });
});
