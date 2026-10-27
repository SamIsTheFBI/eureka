package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Registry keeps lightweight atomic counters for gateway observability.
type Registry struct {
	ToolCallsTotal atomic.Uint64
	ToolCallsError atomic.Uint64
	HttpRequests   atomic.Uint64
	AuthFailures   atomic.Uint64
}

// Global registry instance.
var DefaultRegistry = &Registry{}

// StartServer starts the metrics listener on port 9090.
func StartServer(port string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP gateway_tool_calls_total Total tool calls handled\n")
		fmt.Fprintf(w, "# TYPE gateway_tool_calls_total counter\n")
		fmt.Fprintf(w, "gateway_tool_calls_total %d\n", DefaultRegistry.ToolCallsTotal.Load())

		fmt.Fprintf(w, "# HELP gateway_tool_calls_error Total tool calls with errors\n")
		fmt.Fprintf(w, "# TYPE gateway_tool_calls_error counter\n")
		fmt.Fprintf(w, "gateway_tool_calls_error %d\n", DefaultRegistry.ToolCallsError.Load())

		fmt.Fprintf(w, "# HELP gateway_http_requests_total Total HTTP requests handled\n")
		fmt.Fprintf(w, "# TYPE gateway_http_requests_total counter\n")
		fmt.Fprintf(w, "gateway_http_requests_total %d\n", DefaultRegistry.HttpRequests.Load())

		fmt.Fprintf(w, "# HELP gateway_auth_failures_total Total failed authentication attempts\n")
		fmt.Fprintf(w, "# TYPE gateway_auth_failures_total counter\n")
		fmt.Fprintf(w, "gateway_auth_failures_total %d\n", DefaultRegistry.AuthFailures.Load())
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	go func() {
		_ = srv.ListenAndServe()
	}()
	return srv
}
