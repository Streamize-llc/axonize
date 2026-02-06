# Dashboard Guide

The Axonize dashboard provides a web UI for exploring traces, monitoring GPU performance, and viewing analytics.

## Accessing the Dashboard

After starting with `docker compose up -d`, open:

```
http://localhost:3000
```

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

By default, the dashboard proxies API requests through nginx to the Axonize server. To point at a different server:

Set `VITE_API_URL` before building:

```bash
VITE_API_URL=http://your-server:8080 npm run build
```

### Development Mode

For local development with hot reload:

```bash
cd dashboard
npm install
npm run dev
```

The dev server runs on port 3000 and proxies `/api` requests to `localhost:8080`.
