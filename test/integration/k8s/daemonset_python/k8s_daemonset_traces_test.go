//go:build integration

package otel

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/mariomac/guara/pkg/test"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/grafana/beyla/test/integration/components/jaeger"
)

// For the DaemonSet scenario, we only check that Beyla is able to instrument any
// process in the system. We just check that traces are properly generated without
// entering in too many details
func TestBasicTracing(t *testing.T) {
	feat := features.New("Beyla is able to instrument an arbitrary process").
		Assess("it sends traces for that service",
			func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
				var trace jaeger.Trace
				test.Eventually(t, testTimeout, func(t require.TestingT) {
					resp, err := http.Get("http://localhost:38083/greeting")
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, resp.StatusCode)

					resp, err = http.Get(jaegerQueryURL + "?service=python3.11&operation=GET%20%2Fgreeting")
					require.NoError(t, err)
					if resp == nil {
						return
					}
					require.Equal(t, http.StatusOK, resp.StatusCode)
					var tq jaeger.TracesQuery
					require.NoError(t, json.NewDecoder(resp.Body).Decode(&tq))
					traces := tq.FindBySpan(jaeger.Tag{Key: "url.path", Type: "string", Value: "/greeting"})
					require.NotEmpty(t, traces)
					trace = traces[0]
					require.NotEmpty(t, trace.Spans)

					// Check the information of the parent span
					res := trace.FindByOperationName("GET /greeting")
					require.Len(t, res, 1)
					parent := res[0]
					sd := jaeger.Diff([]jaeger.Tag{
						{Key: "service.namespace", Type: "string", Value: "integration-test"},
						{Key: "telemetry.sdk.language", Type: "string", Value: "python"},
					}, trace.Processes[parent.ProcessID].Tags)
					require.Empty(t, sd, sd.String())
				}, test.Interval(100*time.Millisecond))
				return ctx
			},
		).Feature()
	cluster.TestEnv().Test(t, feat)
}
