package kube

import (
	"context"
	"encoding/json"
	"fmt"
)

// PrometheusAlert represents a single firing alert from Prometheus.
type PrometheusAlert struct {
	AlertName  string
	State      string
	Severity   string
	Namespace  string
	Pod        string
	Message    string
	Controller string
}

// prometheusResponse is the JSON response from /api/v1/query.
type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// QueryFiringAlerts queries Prometheus for currently firing alerts via the
// Kubernetes service proxy (no port-forward needed).
func (c *Client) QueryFiringAlerts(ctx context.Context, promNamespace, promService string, promPort int) ([]PrometheusAlert, error) {
	// Use http:<svc>:<port> format for the service proxy — Kubernetes requires
	// the scheme prefix for non-default ports.
	proxyPath := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s:%d/proxy/api/v1/query",
		promNamespace, promService, promPort)

	resp, err := c.Clientset.CoreV1().RESTClient().Get().
		AbsPath(proxyPath).
		Param("query", `ALERTS{alertstate="firing"}`).
		SetHeader("Accept", "application/json").
		DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus: %w", err)
	}

	var promResp prometheusResponse
	if err := json.Unmarshal(resp, &promResp); err != nil {
		return nil, fmt.Errorf("parsing prometheus response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: status=%s", promResp.Status)
	}

	var alerts []PrometheusAlert
	for _, r := range promResp.Data.Result {
		alert := PrometheusAlert{
			AlertName:  r.Metric["alertname"],
			State:      r.Metric["alertstate"],
			Severity:   r.Metric["severity"],
			Namespace:  r.Metric["namespace"],
			Pod:        r.Metric["pod"],
			Controller: r.Metric["controller"],
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// QueryPrometheusRaw runs an arbitrary PromQL query and returns the raw JSON.
func (c *Client) QueryPrometheusRaw(ctx context.Context, promNamespace, promService string, promPort int, query string) ([]byte, error) {
	proxyPath := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s:%d/proxy/api/v1/query",
		promNamespace, promService, promPort)

	resp, err := c.Clientset.CoreV1().RESTClient().Get().
		AbsPath(proxyPath).
		Param("query", query).
		SetHeader("Accept", "application/json").
		DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus: %w", err)
	}
	return resp, nil
}
