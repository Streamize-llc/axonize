# Dashboard Guide

The Axonize dashboard provides a web UI for exploring traces, monitoring GPU performance, and viewing analytics.

## Accessing the Dashboard

The dashboard lives in a separate repository: [axonize-web](https://github.com/Streamize-llc/axonize-web).

```bash
git clone https://github.com/Streamize-llc/axonize-web.git
cd axonize-web
npm install
npm run dev
```

Then open `http://localhost:3000`.

## Pages

### Overview

The landing page shows key metrics:

- **Total Traces** — Number of inference operations tracked
- **Avg Latency** — Mean inference duration
- **Error Rate** — Percentage of failed operations
- **Active GPUs** — Number of GPUs that reported metrics

Charts show throughput over time and latency percentiles (p50, p95, p99).

### Traces

Browse and search through your inference traces:

- **Filter** by service name using the search box
- **Sort** by time (most recent first)
- **Paginate** through large result sets
- Click a trace ID to see its detail

### Trace Detail

Drill into a single trace:

- **Gantt Timeline** — Visual span timeline showing execution order and duration
- **Span Tree** — Hierarchical view of parent-child relationships
- **Span Panel** — Click any span to see attributes, status, timing, and errors

Color coding:
- Purple bars = successful spans
- Red bars = error spans

### GPUs

Monitor registered GPUs:

- **Card Grid** — Each GPU shows model, utilization, memory usage
- Progress bars color-coded by load (green < 70%, yellow < 90%, red >= 90%)
- Click a GPU card for details

### GPU Detail

Deep dive into a single GPU:

- **Spec Info** — Physical UUID, resource type, node, first/last seen
- **Utilization & Memory Chart** — Time series with dual Y-axes
- **Power Chart** — Power consumption over time

## Configuration

### API URL

By default, the dashboard proxies API requests to the Axonize server at `localhost:8080`. To point at a different server, set `VITE_API_URL` before building:

```bash
VITE_API_URL=http://your-server:8080 npm run build
```

See the [axonize-web README](https://github.com/Streamize-llc/axonize-web) for full setup and development instructions.
