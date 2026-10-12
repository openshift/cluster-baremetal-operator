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
