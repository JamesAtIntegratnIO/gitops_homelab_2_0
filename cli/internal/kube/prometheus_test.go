package kube

import (
	"encoding/json"
	"testing"
)

func TestPrometheusResponseParsing(t *testing.T) {
	raw := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {
						"alertname": "KubePodCrashLooping",
						"alertstate": "firing",
						"severity": "warning",
						"namespace": "monitoring",
						"pod": "prometheus-0",
						"controller": "statefulset/prometheus"
					},
					"value": [1709000000, "1"]
				},
				{
					"metric": {
						"alertname": "TargetDown",
						"alertstate": "firing",
						"severity": "critical",
						"namespace": "kube-system"
					},
					"value": [1709000000, "1"]
				}
			]
		}
	}`

	var promResp prometheusResponse
	if err := json.Unmarshal([]byte(raw), &promResp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if promResp.Status != "success" {
		t.Errorf("status = %q, want success", promResp.Status)
	}
	if promResp.Data.ResultType != "vector" {
		t.Errorf("resultType = %q, want vector", promResp.Data.ResultType)
	}
	if len(promResp.Data.Result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(promResp.Data.Result))
	}

	// Parse into PrometheusAlert like QueryFiringAlerts does
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

	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}

	if alerts[0].AlertName != "KubePodCrashLooping" {
		t.Errorf("alert[0].AlertName = %q, want KubePodCrashLooping", alerts[0].AlertName)
	}
	if alerts[0].Severity != "warning" {
		t.Errorf("alert[0].Severity = %q, want warning", alerts[0].Severity)
	}
	if alerts[0].Pod != "prometheus-0" {
		t.Errorf("alert[0].Pod = %q, want prometheus-0", alerts[0].Pod)
	}
	if alerts[0].Controller != "statefulset/prometheus" {
		t.Errorf("alert[0].Controller = %q, want statefulset/prometheus", alerts[0].Controller)
	}

	if alerts[1].AlertName != "TargetDown" {
		t.Errorf("alert[1].AlertName = %q, want TargetDown", alerts[1].AlertName)
	}
	if alerts[1].Severity != "critical" {
		t.Errorf("alert[1].Severity = %q, want critical", alerts[1].Severity)
	}
	// Pod not set for second alert
	if alerts[1].Pod != "" {
		t.Errorf("alert[1].Pod = %q, want empty", alerts[1].Pod)
	}
}

func TestPrometheusResponseParsing_EmptyResults(t *testing.T) {
	raw := `{"status":"success","data":{"resultType":"vector","result":[]}}`

	var promResp prometheusResponse
	if err := json.Unmarshal([]byte(raw), &promResp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if promResp.Status != "success" {
		t.Errorf("status = %q, want success", promResp.Status)
	}
	if len(promResp.Data.Result) != 0 {
		t.Errorf("expected 0 results, got %d", len(promResp.Data.Result))
	}
}

func TestPrometheusResponseParsing_ErrorStatus(t *testing.T) {
	raw := `{"status":"error","data":{"resultType":"","result":[]}}`

	var promResp prometheusResponse
	if err := json.Unmarshal([]byte(raw), &promResp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if promResp.Status != "error" {
		t.Errorf("status = %q, want error", promResp.Status)
	}
}
