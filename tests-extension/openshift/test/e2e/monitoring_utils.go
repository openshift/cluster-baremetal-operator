package baremetal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	maxCpuUsageAllowed       float64 = 90.0
	minRequiredMemoryInBytes         = 1000000000
)

type PrometheusAPIResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Instance string `json:"instance"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func getNodeCpuUsage(oc *exutil.CLI, node string, sampling_time int) float64 {
	samplingTime := strconv.Itoa(sampling_time)

	// URL-encode dynamic values to prevent injection
	encodedNode := url.QueryEscape(node)
	encodedSamplingTime := url.QueryEscape(samplingTime)

	cpu_sampling := "node_cpu_seconds_total%20%7Binstance%3D%27" + encodedNode
	cpu_sampling += "%27%2C%20mode%3D%27idle%27%7D%5B" + encodedSamplingTime + "m%5D"
	query := "query=100%20-%20(avg%20by%20(instance)(irate(" + cpu_sampling + "))%20*%20100)"
	queryUrl := "http://localhost:9090/api/v1/query?" + query

	jsonString, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "--", "curl", "-s", queryUrl).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	var response PrometheusAPIResponse
	unmarshalErr := json.Unmarshal([]byte(jsonString), &response)
	o.Expect(unmarshalErr).NotTo(o.HaveOccurred())

	cpuUsage, ok := response.Data.Result[0].Value[1].(string)
	o.Expect(ok).To(o.BeTrue(), "CPU usage value is not a string")

	cpu_usage, err := strconv.ParseFloat(cpuUsage, 64)
	o.Expect(err).NotTo(o.HaveOccurred())

	return cpu_usage
}

func getClusterUptime(ctx context.Context, oc *exutil.CLI) (int, error) {
	// Check if context is already cancelled before making network call
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("context cancelled before cluster uptime query: %w", err)
	}

	layout := "2006-01-02T15:04:05Z"
	completionTime, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[*].status.history[*].completionTime}").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get clusterversion completion time: %w", err)
	}

	timestamps := strings.Fields(completionTime)
	if len(timestamps) == 0 {
		return 0, fmt.Errorf("no completion time found in clusterversion history")
	}

	// Use first timestamp which is the oldest completion time (initial cluster deployment)
	returnTime, perr := time.Parse(layout, timestamps[0])
	if perr != nil {
		e2e.Logf("Error trying to parse uptime %s", perr)
		return 0, perr
	}
	now := time.Now()
	uptime := now.Sub(returnTime)
	uptimeByMin := int(uptime.Minutes())
	return uptimeByMin, nil
}

func getNodeavailMem(oc *exutil.CLI, node string) int {
	// URL-encode dynamic values to prevent injection
	encodedNode := url.QueryEscape(node)

	query := "query=node_memory_MemAvailable_bytes%7Binstance%3D%27" + encodedNode + "%27%7D"
	queryUrl := "http://localhost:9090/api/v1/query?" + query

	jsonString, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "--", "curl", "-s", queryUrl).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	var response PrometheusAPIResponse
	unmarshalErr := json.Unmarshal([]byte(jsonString), &response)
	o.Expect(unmarshalErr).NotTo(o.HaveOccurred())

	memUsage, ok := response.Data.Result[0].Value[1].(string)
	o.Expect(ok).To(o.BeTrue(), "Memory usage value is not a string")

	availableMemFloat, err := strconv.ParseFloat(memUsage, 64)
	o.Expect(err).NotTo(o.HaveOccurred())
	return int(availableMemFloat)
}

func queryPrometheus(oc *exutil.CLI, promQL string) (PrometheusAPIResponse, error) {
	queryUrl := "http://localhost:9090/api/v1/query?" + url.Values{"query": {promQL}}.Encode()
	jsonString, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "--", "curl", "-s", queryUrl).Output()
	if err != nil {
		return PrometheusAPIResponse{}, fmt.Errorf("prometheus query %q failed: %w", promQL, err)
	}

	var response PrometheusAPIResponse
	if unmarshalErr := json.Unmarshal([]byte(jsonString), &response); unmarshalErr != nil {
		return PrometheusAPIResponse{}, fmt.Errorf("failed to parse prometheus response for %q: %w: %s", promQL, unmarshalErr, jsonString)
	}
	if response.Status != "success" {
		return response, fmt.Errorf("prometheus query %q returned status %q", promQL, response.Status)
	}
	return response, nil
}

type prometheusRulesAPIResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups []prometheusRuleGroup `json:"groups"`
	} `json:"data"`
}

type prometheusRuleGroup struct {
	Name  string `json:"name"`
	File  string `json:"file"`
	Rules []struct {
		Name  string `json:"name"`
		Query string `json:"query"`
		Type  string `json:"type"`
	} `json:"rules"`
}

func getPrometheusAlertRules(oc *exutil.CLI) (prometheusRulesAPIResponse, error) {
	jsonString, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "--", "curl", "-s", "http://localhost:9090/api/v1/rules?type=alert").Output()
	if err != nil {
		return prometheusRulesAPIResponse{}, fmt.Errorf("failed to get prometheus alert rules: %w", err)
	}

	var response prometheusRulesAPIResponse
	if unmarshalErr := json.Unmarshal([]byte(jsonString), &response); unmarshalErr != nil {
		return prometheusRulesAPIResponse{}, fmt.Errorf("failed to parse prometheus rules response: %w", unmarshalErr)
	}
	if response.Status != "success" {
		return response, fmt.Errorf("prometheus rules API returned status %q", response.Status)
	}
	return response, nil
}

func waitForPrometheusQuery(oc *exutil.CLI, promQL string, timeout time.Duration) PrometheusAPIResponse {
	var last PrometheusAPIResponse
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, timeout, true, func(context.Context) (bool, error) {
		resp, qerr := queryPrometheus(oc, promQL)
		if qerr != nil {
			e2e.Logf("Prometheus query %q failed: %v", promQL, qerr)
			return false, nil
		}
		last = resp
		if len(resp.Data.Result) == 0 {
			e2e.Logf("Prometheus query %q returned no results, retrying...", promQL)
			return false, nil
		}
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("Prometheus query %q returned no results", promQL))
	return last
}
