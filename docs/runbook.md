# Urumi Operations Runbook

## 1. Cluster Status & Diagnostics

### Check All Pods and Deployments
```bash
# Store namespace
kubectl get pods,svc,pvc,ingress -n store

# Platform namespace
kubectl get pods,svc,pvc,ingress -n platform
```

### Inspect Container Logs
```bash
# Gateway logs
kubectl logs -n platform -l app=gateway --tail=100 -f

# WooCommerce MCP logs
kubectl logs -n store -l app=woo-mcp --tail=100 -f

# WordPress logs
kubectl logs -n store -l app=wordpress --tail=100 -f
```

## 2. Upstream Server Troubleshooting

### Server Status Is 'Unreachable'
1. Check the `reason` field in the dashboard Servers tab or via `/api/servers`.
2. Common codes:
   - `connection_refused`: The target service pod is not running or the port is wrong. Verify with `kubectl get pods -n store`.
   - `timeout`: Network latency or pod is overloaded. The default connect deadline is 3s and call timeout is 30s.
   - `auth_failed`: Bearer token or custom header mismatch. Verify the secret matches between namespaces:
     ```bash
     kubectl get secret woo-mcp-token -n store -o yaml
     kubectl get secret woo-mcp-token -n platform -o yaml
     ```

### Simulating Upstream Pod Outage (Demo Rehearsal)
To demonstrate failure isolation:
```bash
# Scale down woo-mcp to 0 replicas
kubectl scale deployment woo-mcp -n store --replicas=0

# Verify gateway continues serving utils-mcp and docs-mcp
curl -s -X POST http://app.localtest.me/mcp \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"utils__time_now","arguments":{}}}'

# Restore woo-mcp
kubectl scale deployment woo-mcp -n store --replicas=1
```

## 3. Database Maintenance & Backups

### PostgreSQL Dump (Platform Database)
```bash
kubectl exec -n platform statefulset/postgres -- pg_dump -U postgres gateway > backup_gateway.sql
```

### MySQL Dump (WordPress / Store Database)
```bash
kubectl exec -n store statefulset/mysql -- mysqldump -u wordpress -pStorePassword123! wordpress > backup_store.sql
```

## 4. Upgrading Releases with Rollback Guarantee

Deployments use `--atomic` to automatically revert on upgrade failure:
```bash
# Safe atomic upgrade
helm upgrade store charts/store -n store -f charts/store/values-local.yaml --atomic --wait

# History & manual rollback if necessary
helm history platform -n platform
helm rollback platform <revision> --wait
```
