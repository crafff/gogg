import http from "k6/http";
import { check, sleep } from "k6";
import { Trend } from "k6/metrics";

const baseURL = __ENV.BASE_URL || "http://api-observed:8080";
const scenario = __ENV.SCENARIO || "rankings";
const testMode = __ENV.TEST_MODE || "warm";
const responseBytes = new Trend("gogg_response_bytes");

export const options = {
  scenarios: {
    baseline: testMode === "cold"
      ? {
          executor: "shared-iterations",
          vus: 1,
          iterations: 1,
          maxDuration: "2m",
        }
      : {
          executor: "constant-vus",
          vus: Number(__ENV.VUS || 20),
          duration: __ENV.DURATION || "1m",
        },
  },
  thresholds: {
    checks: ["rate>0.99"],
    "http_req_failed{phase:load}": ["rate<0.01"],
    "http_req_duration{phase:load}": ["p(95)<500", "p(99)<1000"],
  },
  summaryTrendStats: ["avg", "med", "p(50)", "p(95)", "p(99)", "max"],
};

const requests = {
  rankings: (phase) =>
    http.get(
      `${baseURL}/api/v1/rankings/champions?region=KR&version=latest&tier=master_plus`,
      { tags: { endpoint: "rankings", phase } },
    ),
  versions: (phase) =>
    http.get(`${baseURL}/api/v1/versions`, { tags: { endpoint: "versions", phase } }),
  regions: (phase) =>
    http.get(`${baseURL}/api/v1/regions`, { tags: { endpoint: "regions", phase } }),
  graphql: (phase) =>
    http.post(
      `${baseURL}/graphql`,
      JSON.stringify({ operationName: "Versions", query: "query Versions { versions }" }),
      {
        headers: { "Content-Type": "application/json" },
        tags: { endpoint: "graphql_versions", phase },
      },
    ),
};

export function setup() {
  if (!requests[scenario]) {
    throw new Error(`unknown SCENARIO=${scenario}`);
  }
  if ((__ENV.CACHE_MODE || "warm") === "warm") {
    const response = requests[scenario]("warmup");
    if (response.status !== 200) {
      throw new Error(`warm-up failed: HTTP ${response.status}: ${response.body}`);
    }
  }
}

export default function () {
  const response = requests[scenario]("load");
  responseBytes.add(response.body ? response.body.length : 0, { endpoint: scenario });
  check(response, {
    "status is 200": (r) => r.status === 200,
    "response is JSON": (r) => (r.headers["Content-Type"] || "").includes("application/json"),
  });
  sleep(0.1);
}
