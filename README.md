# Urumi: MCP Gateway on Kubernetes

Urumi is an enterprise-grade Model Context Protocol (MCP) gateway running entirely on Kubernetes. It provides AI clients (Claude Code, Cursor, MCP Inspector) and an integrated chat agent with a single secure, authenticated entrance to discover and execute tools across upstream providers—starting with a fully automated WooCommerce store.

---

## 🌟 Key Features

* **Unified MCP Endpoint (`/mcp`):** Connect external clients once to `http://app.localtest.me/mcp` using a personal API key (`urmk_...`).
* **Tool Aggregation & Namespacing:** Automatically aggregates tools across upstreams with predictable `<slug>__<tool>` naming (e.g., `woo__list_orders`, `utils__time_now`).
* **Immediate Key Revocation:** API keys are verified against PostgreSQL on every call; revoking a key takes effect immediately without cache delays.
* **Fault Isolation:** Built-in circuit breakers, bulkheads, and timeouts ensure slow or dead upstreams never degrade other tools.
* **Full Audit Attribution:** Every tool execution is attributed to a user and recorded with execution duration, server slug, tool name, and outcome.
* **In-Process Chat Agent:** Interactive dashboard chat agent that executes tools through the gateway as the logged-in user with live SSE progress streaming.
* **Zero Secret Leakage:** Upstream credentials are encrypted at rest with AES-256-GCM and never returned via the API or browser.

---

## 📂 Repository Layout

```text
.
├── services/
│   ├── gateway/       # Main Go MCP gateway, router, aggregator & admin API
│   ├── woo-mcp/       # Go MCP server implementing 5 WooCommerce REST tools
│   ├── utils-mcp/     # Go MCP server for time_now & calculate tools
│   └── dashboard/     # React + TypeScript + Vite Single Page Application
├── charts/
│   ├── common/        # Shared Helm library chart (labels, pod security)
│   ├── store/         # Helm chart: WordPress, WooCommerce, MySQL, woo-mcp
│   └── platform/      # Helm chart: Gateway, Postgres, Dashboard, utils-mcp
├── deploy/            # Cluster manifests, custom Dockerfiles, and demo script
├── docs/              # System architecture, tradeoffs, and threat model
└── Makefile           # Monorepo build, test, and deployment automation
```

---

## 🚀 Quickstart: Local Setup (k3d)

### 1. Prerequisites
* **Docker / OrbStack**
* **Go 1.24+**
* **Node.js 20+**
* **k3d**, **kubectl**, and **helm** (`brew install k3d kubectl helm`)

### 2. Create Cluster
```bash
k3d cluster create urumi -p "80:80@loadbalancer" -p "443:443@loadbalancer"
```

### 3. Build & Deploy Everything
Run the automated Makefile targets:
```bash
# 1. Compile binaries and frontend
make build

# 2. Run unit tests
make test

# 3. Build all Docker images
make images

# 4. Import images into local k3d cluster
make import

# 5. Deploy Helm charts
make up
```

---

## 🌐 Endpoints

| Service | Local URL | Default Credentials |
|---|---|---|
| **Dashboard** | `http://app.localtest.me` | `admin@localtest.me` / `AdminPassword123!` |
| **Unified MCP Door** | `http://app.localtest.me/mcp` | Bearer `<your-api-key>` |
| **WooCommerce Storefront** | `http://store.localtest.me` | Cash on Delivery (COD) enabled |
| **WordPress Admin** | `http://store.localtest.me/wp-admin` | `admin` / `AdminStorePass123!` |
| **Prometheus Metrics** | `gateway.platform:9090/metrics` | Internal cluster only |

---

## 🔌 Connecting External AI Clients

1. Go to `http://app.localtest.me`, sign in, and navigate to **API Keys**.
2. Click **+ Create API Key** and copy your generated key (`urmk_...`).
3. Connect your preferred client:

### Claude Code CLI
```bash
claude mcp add --transport http urumi http://app.localtest.me/mcp --header "Authorization: Bearer <YOUR_KEY>"
```

### Cursor (`.cursor/mcp.json`)
```json
{
  "mcpServers": {
    "urumi": {
      "url": "http://app.localtest.me/mcp",
      "headers": {
        "Authorization": "Bearer <YOUR_KEY>"
      }
    }
  }
}
```

### MCP Inspector
```bash
npx @modelcontextprotocol/inspector http://app.localtest.me/mcp --header "Authorization: Bearer <YOUR_KEY>"
```

---

## 🧪 Automated Demo & Verification

Run the end-to-end definition-of-done verification script:
```bash
make demo
```

The script exercises:
1. Storefront accessibility and checkout availability
2. Dashboard authentication
3. API key generation and immediate lookup
4. Multi-server aggregation (`woo__*`, `utils__*`, `docs__*`)
5. Namespaced tool execution (`utils__time_now`, `woo__list_orders`)
6. Instant key revocation proof (returns HTTP 401)
7. Audit log verification proving attribution of every call
