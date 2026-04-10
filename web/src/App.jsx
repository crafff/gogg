import { useEffect, useMemo, useState } from "react";

const POSITIONS = ["", "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"];

function App() {
  const [position, setPosition] = useState("");
  const [limit, setLimit] = useState(20);
  const [minGames, setMinGames] = useState(20);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [items, setItems] = useState([]);

  const query = useMemo(() => {
    const params = new URLSearchParams();
    params.set("limit", String(limit));
    params.set("minGames", String(minGames));
    if (position) {
      params.set("position", position);
    }
    return params.toString();
  }, [limit, minGames, position]);

  useEffect(() => {
    let cancelled = false;

    async function run() {
      setLoading(true);
      setError("");
      try {
        const res = await fetch(`/api/rankings/champions?${query}`);
        if (!res.ok) {
          throw new Error(`request failed (${res.status})`);
        }
        const data = await res.json();
        if (!cancelled) {
          setItems(Array.isArray(data.items) ? data.items : []);
        }
      } catch (e) {
        if (!cancelled) {
          setError(e.message || "failed to load rankings");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    run();
    return () => {
      cancelled = true;
    };
  }, [query]);

  return (
    <div className="page">
      <header className="hero">
        <h1>GOGG Champion Rankings</h1>
        <p>Data source: PostgreSQL match_participants and bans</p>
      </header>

      <section className="panel controls">
        <label>
          Position
          <select value={position} onChange={(e) => setPosition(e.target.value)}>
            {POSITIONS.map((v) => (
              <option key={v || "ALL"} value={v}>
                {v || "ALL"}
              </option>
            ))}
          </select>
        </label>

        <label>
          Min Games
          <input
            type="number"
            min="1"
            max="20000"
            value={minGames}
            onChange={(e) => setMinGames(Number(e.target.value || 20))}
          />
        </label>

        <label>
          Limit
          <input
            type="number"
            min="1"
            max="200"
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value || 20))}
          />
        </label>
      </section>

      <section className="panel table-panel">
        {loading && <p className="state">Loading rankings...</p>}
        {error && <p className="state error">{error}</p>}
        {!loading && !error && items.length === 0 && <p className="state">No ranking data found.</p>}

        {!loading && !error && items.length > 0 && (
          <table>
            <thead>
              <tr>
                <th>#</th>
                <th>Champion</th>
                <th>Games</th>
                <th>Wins</th>
                <th>Losses</th>
                <th>Win Rate</th>
                <th>Pick Rate</th>
                <th>Ban Rate</th>
                <th>KDA</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it, idx) => (
                <tr key={it.championId}>
                  <td>{idx + 1}</td>
                  <td>{it.championName}</td>
                  <td>{it.games}</td>
                  <td>{it.wins}</td>
                  <td>{it.losses}</td>
                  <td>{Number(it.winRate).toFixed(2)}%</td>
                  <td>{Number(it.pickRate).toFixed(2)}%</td>
                  <td>{Number(it.banRate).toFixed(2)}%</td>
                  <td>{Number(it.kda).toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}

export default App;
