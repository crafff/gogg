import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Trend } from "k6/metrics";

const baseURL = __ENV.BASE_URL || "http://api-observed:8080";
const scenario = __ENV.SCENARIO || "rankings_graphql";
const testMode = __ENV.TEST_MODE || "warm";
const responseBytes = new Trend("gogg_response_bytes");
const contractMismatches = new Counter("gogg_contract_mismatches");

const fixedVersion = __ENV.VERSION || "16.13";
const defaultRegion = __ENV.REGION || "KR";
const defaultTier = __ENV.TIER || "master_plus";
const defaultPosition = __ENV.POSITION || "";

const tierToGraphQL = {
  master_plus: "MASTER_PLUS",
  master: "MASTER",
  grandmaster_plus: "GRANDMASTER_PLUS",
  grandmaster: "GRANDMASTER",
  challenger: "CHALLENGER",
};

const championRankingsQuery = `
  query ChampionRankings($filter: ChampionRankingsFilter) {
    championRankings(filter: $filter) {
      items {
        championId championName teamPosition games wins losses
        winRate pickRate banRate kda
      }
      totalMatches
      resolvedVersion
    }
  }
`;

const workloads = {
  rankings_graphql: { transport: "graphql", region: defaultRegion, tier: defaultTier, position: defaultPosition },
  rankings_rest: { transport: "rest", region: defaultRegion, tier: defaultTier, position: defaultPosition },
  "RKG-GQL-KR-MP-ALL": { transport: "graphql", region: "KR", tier: "master_plus", position: "" },
  "RKG-GQL-KR-MP-MID": { transport: "graphql", region: "KR", tier: "master_plus", position: "MIDDLE" },
  "RKG-GQL-NA1-MP-ALL": { transport: "graphql", region: "NA1", tier: "master_plus", position: "" },
  "RKG-GQL-NA1-MP-MID": { transport: "graphql", region: "NA1", tier: "master_plus", position: "MIDDLE" },
  "RKG-GQL-KR-C-ALL": { transport: "graphql", region: "KR", tier: "challenger", position: "" },
  "RKG-REST-KR-MP-ALL": { transport: "rest", region: "KR", tier: "master_plus", position: "" },
  "RKG-REST-KR-MP-MID": { transport: "rest", region: "KR", tier: "master_plus", position: "MIDDLE" },
  versions: { transport: "catalog", path: "/api/v1/versions", endpoint: "versions" },
  regions: { transport: "catalog", path: "/api/v1/regions", endpoint: "regions" },
  graphql_versions: { transport: "graphql_catalog", endpoint: "graphql_versions" },
};

const isContractMatrix = scenario === "contract_matrix";
const isCatalog = ["versions", "regions"].includes(scenario);
const isGraphQLCatalog = scenario === "graphql_versions";
const latencyLimits = testMode === "cold" && !isCatalog && !isGraphQLCatalog
  ? { p95: 20000, p99: 25000 }
  : isGraphQLCatalog
    ? { p95: 300, p99: 750 }
    : { p95: 100, p99: 250 };
const thresholds = isContractMatrix
  ? {
      checks: ["rate==1"],
      gogg_contract_mismatches: ["count==0"],
    }
  : {
      checks: ["rate>0.999"],
      "http_req_failed{phase:load}": ["rate<0.001"],
      "http_req_duration{phase:load}": [`p(95)<${latencyLimits.p95}`, `p(99)<${latencyLimits.p99}`],
      gogg_response_bytes: ["p(99)<262144"],
    };
if (!isContractMatrix && testMode !== "cold") {
  thresholds["http_reqs{phase:load}"] = [isCatalog ? "rate>50" : "rate>20"];
}

export const options = {
  scenarios: {
    baseline: isContractMatrix || testMode === "cold"
      ? {
          executor: "shared-iterations",
          vus: 1,
          iterations: 1,
          maxDuration: isContractMatrix ? "30m" : "2m",
        }
      : {
          executor: "constant-vus",
          vus: Number(__ENV.VUS || 20),
          duration: __ENV.DURATION || "1m",
        },
  },
  thresholds,
  summaryTrendStats: ["avg", "med", "p(50)", "p(95)", "p(99)", "max"],
};

function graphqlFilter(region, tier, position) {
  return {
    queueId: 420,
    region,
    version: fixedVersion,
    tierGroup: tierToGraphQL[tier],
    position,
    minGames: 20,
    positionThreshold: 5.0,
  };
}

function requestGraphQL(region, tier, position, phase, endpoint) {
  return http.post(
    `${baseURL}/graphql`,
    JSON.stringify({
      operationName: "ChampionRankings",
      query: championRankingsQuery,
      variables: { filter: graphqlFilter(region, tier, position) },
    }),
    {
      headers: { "Content-Type": "application/json" },
      tags: { endpoint, phase, workload: scenario },
    },
  );
}

function requestREST(region, tier, position, phase, endpoint) {
  const params = [
    "queueId=420",
    `region=${encodeURIComponent(region)}`,
    `version=${encodeURIComponent(fixedVersion)}`,
    `tier=${encodeURIComponent(tier)}`,
    "minGames=20",
    "positionThreshold=5",
  ];
  if (position) params.push(`position=${encodeURIComponent(position)}`);
  return http.get(`${baseURL}/api/v1/rankings/champions?${params.join("&")}`, {
    tags: { endpoint, phase, workload: scenario },
  });
}

function requestWorkload(workload, phase) {
  if (workload.transport === "catalog") {
    return http.get(`${baseURL}${workload.path}`, {
      tags: { endpoint: workload.endpoint, phase, workload: scenario },
    });
  }
  if (workload.transport === "graphql_catalog") {
    return http.post(
      `${baseURL}/graphql`,
      JSON.stringify({ operationName: "Versions", query: "query Versions { versions }" }),
      {
        headers: { "Content-Type": "application/json" },
        tags: { endpoint: workload.endpoint, phase, workload: scenario },
      },
    );
  }
  if (workload.transport === "graphql") {
    return requestGraphQL(workload.region, workload.tier, workload.position, phase, "rankings_graphql");
  }
  return requestREST(workload.region, workload.tier, workload.position, phase, "rankings_rest");
}

function validRankingItem(item, position) {
  return item.championId > 0 && item.championName !== "" && item.games >= 20 &&
    item.wins + item.losses === item.games && item.winRate >= 0 && item.winRate <= 100 &&
    item.pickRate >= 0 && item.pickRate <= 100 && item.banRate >= 0 && item.banRate <= 100 &&
    Number.isFinite(item.kda) && item.kda >= 0 &&
    (!position || (item.teamPosition.length === 1 && item.teamPosition[0] === position));
}

function checkWorkloadResponse(response, workload) {
  let payload;
  try {
    payload = response.json();
  } catch (_) {
    payload = null;
  }
  if (workload.transport === "catalog" || workload.transport === "graphql_catalog") {
    return check(response, {
      "status is 200": (r) => r.status === 200,
      "response is JSON": (r) => (r.headers["Content-Type"] || "").includes("application/json"),
      "catalog payload is present": () => payload !== null && !payload?.errors?.length,
    });
  }
  const result = workload.transport === "graphql" ? payload?.data?.championRankings : payload;
  const noGraphQLErrors = workload.transport !== "graphql" || !payload?.errors?.length;
  const items = result?.items;
  const uniqueChampionIDs = Array.isArray(items) && new Set(items.map((item) => item.championId)).size === items.length;
  return check(response, {
    "status is 200": (r) => r.status === 200,
    "response is JSON": (r) => (r.headers["Content-Type"] || "").includes("application/json"),
    "GraphQL has no errors": () => noGraphQLErrors,
    "items satisfy rankings contract": () => Array.isArray(items) && items.length <= 500 &&
      uniqueChampionIDs && items.every((item) => validRankingItem(item, workload.position)),
  });
}

function canonicalItems(items) {
  return items
    .map((item) => ({
      championId: item.championId,
      championName: item.championName,
      teamPosition: [...item.teamPosition].sort(),
      games: item.games,
      wins: item.wins,
      losses: item.losses,
      winRate: item.winRate,
      pickRate: item.pickRate,
      banRate: item.banRate,
      kda: item.kda,
    }))
    .sort((a, b) => a.championId - b.championId);
}

function runContractMatrix() {
  contractMismatches.add(0);
  const regions = ["KR", "NA1"];
  const tiers = Object.keys(tierToGraphQL);
  const positions = ["", "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"];

  for (const region of regions) {
    for (const tier of tiers) {
      for (const position of positions) {
        const rest = requestREST(region, tier, position, "contract", "contract_rest");
        const gql = requestGraphQL(region, tier, position, "contract", "contract_graphql");
        let restBody;
        let gqlBody;
        try {
          restBody = rest.json();
          gqlBody = gql.json();
        } catch (_) {
          contractMismatches.add(1);
          check(null, { "contract response is JSON": () => false });
          continue;
        }
        const gqlResult = gqlBody?.data?.championRankings;
        const equal = rest.status === 200 && gql.status === 200 && !gqlBody?.errors?.length &&
          gqlResult && restBody.meta?.version === fixedVersion &&
          restBody.meta?.totalMatches === gqlResult.totalMatches &&
          JSON.stringify(canonicalItems(restBody.items || [])) ===
            JSON.stringify(canonicalItems(gqlResult.items || []));
        if (!equal) contractMismatches.add(1);
        check(null, {
          [`REST/GraphQL parity ${region}/${tier}/${position || "ALL"}`]: () => equal,
        });
      }
    }
  }
}

export function setup() {
  if (isContractMatrix) return;
  const workload = workloads[scenario];
  if (!workload) throw new Error(`unknown SCENARIO=${scenario}`);
  if ((__ENV.CACHE_MODE || "warm") === "warm") {
    const response = requestWorkload(workload, "warmup");
    if (response.status !== 200) {
      throw new Error(`warm-up failed: HTTP ${response.status}: ${response.body}`);
    }
  }
}

export default function () {
  if (isContractMatrix) {
    runContractMatrix();
    return;
  }
  const workload = workloads[scenario];
  const response = requestWorkload(workload, "load");
  responseBytes.add(response.body ? response.body.length : 0, { endpoint: scenario });
  checkWorkloadResponse(response, workload);
  sleep(0.1);
}
