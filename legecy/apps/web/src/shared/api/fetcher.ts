interface GraphQLErrorResponse {
  message?: string;
  extensions?: {
    code?: string;
    retryAfterSeconds?: number;
    rateLimitScope?: string;
  };
}

interface GraphQLResponse<TData> {
  data?: TData;
  errors?: GraphQLErrorResponse[];
}

export class GraphQLRequestError extends Error {
  readonly code: string;
  readonly retryAfterSeconds: number | null;
  readonly rateLimitScope: "IDENTITY" | "IP" | null;

  constructor(
    message: string,
    code: string,
    retryAfterSeconds?: number | null,
    rateLimitScope?: string | null,
    options?: ErrorOptions,
  ) {
    super(message, options);
    this.name = "GraphQLRequestError";
    this.code = code;
    this.retryAfterSeconds = retryAfterSeconds ?? null;
    this.rateLimitScope =
      rateLimitScope === "IDENTITY" || rateLimitScope === "IP"
        ? rateLimitScope
        : null;
  }
}

// Custom react-query codegen fetcher. Keeping the GraphQL extensions intact
// lets feature code distinguish a real rate limit from an unavailable API or
// a browser network failure.
export function fetcher<TData, TVariables>(
  operation: string | { toString(): string },
  variables?: TVariables,
  headers?: RequestInit["headers"],
) {
  return async (): Promise<TData> => {
    let response: Response;
    try {
      response = await fetch("/graphql", {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Content-Type": "application/json",
          "X-GOGG-CSRF": "1",
          ...headerRecord(headers),
        },
        body: JSON.stringify({ query: operation.toString(), variables }),
      });
    } catch (cause) {
      throw new GraphQLRequestError(
        "Unable to reach the API",
        "NETWORK_ERROR",
        null,
        null,
        { cause },
      );
    }

    let payload: GraphQLResponse<TData>;
    try {
      payload = (await response.json()) as GraphQLResponse<TData>;
    } catch (cause) {
      throw new GraphQLRequestError(
        `API returned HTTP ${response.status}`,
        "INVALID_RESPONSE",
        null,
        null,
        { cause },
      );
    }

    const firstError = payload.errors?.[0];
    if (firstError) {
      const retryAfter = firstError.extensions?.retryAfterSeconds;
      throw new GraphQLRequestError(
        firstError.message || "GraphQL request failed",
        firstError.extensions?.code || "GRAPHQL_ERROR",
        typeof retryAfter === "number" && retryAfter > 0 ? retryAfter : null,
        firstError.extensions?.rateLimitScope,
      );
    }
    if (!response.ok || payload.data === undefined) {
      throw new GraphQLRequestError(
        `API returned HTTP ${response.status}`,
        "HTTP_ERROR",
      );
    }
    return payload.data;
  };
}

export function asGraphQLRequestError(
  error: unknown,
): GraphQLRequestError | null {
  return error instanceof GraphQLRequestError ? error : null;
}

function headerRecord(
  headers?: RequestInit["headers"],
): Record<string, string> {
  if (!headers) return {};
  return Object.fromEntries(new Headers(headers).entries());
}
