# Urumi Threat Model & Security Posture

## 1. Trust Boundaries

```
[Untrusted Internet / Public Clients]
               │ Ingress TLS & Origin Verification
               ▼
[Platform Namespace: Gateway / Dashboard / Postgres]
               │ Cross-Namespace NetworkPolicy (TCP 8080) + Shared Bearer Token
               ▼
[Store Namespace: WooCommerce MCP / WordPress / MySQL]
```

## 2. Threat Analysis & Mitigations

### 1. Upstream Credential Harvesting & Leaks
* **Threat:** Clients or third-party upstreams attempting to capture user tokens or upstream master credentials.
* **Mitigation:** Gateway authenticates to upstreams using dedicated, scoped upstream credentials. User API keys (`urmk_...`) are verified at the gateway boundary and never forwarded upstream. Upstream secrets are encrypted at rest with AES-256-GCM and never returned via the Admin API.

### 2. Upstream Tool Poisoning & Rug Pulls
* **Threat:** A compromised or third-party upstream (such as a public docs server) embeds prompt injections in tool descriptions or alters schemas to trick the LLM.
* **Mitigation:**
  * Strict Aggregator sanitation strips control characters and enforces a 1,000-character description limit.
  * Every description is forcibly prefixed with its upstream identity (e.g. `[WooCommerce]`).
  * System prompt instructs the model that tool outputs are data, not instructions.
  * In production, write actions can be gated by human confirmation.

### 3. Server-Side Request Forgery (SSRF)
* **Threat:** An admin enters a malicious upstream URL pointing to cloud metadata (`169.254.169.254`), loopback, or private cluster IP addresses.
* **Mitigation:**
  * Gateway dialer validates IP ranges before connecting and prevents redirects to disallowed private subnets.
  * NetworkPolicies enforce egress restrictions at the kernel/packet level.

### 4. API Key Compromise & Revocation Latency
* **Threat:** An exported or leaked API key is abused before an in-memory cache expires.
* **Mitigation:** API key validity is checked directly against PostgreSQL via an indexed prefix lookup on every request. Marking a key revoked (`revoked_at = NOW()`) rejects the very next request within milliseconds.
