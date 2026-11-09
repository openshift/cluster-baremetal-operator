package baremetal

import (
	"context"
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	ipeServiceMonitorName = "metal3-ironic-prometheus-exporter"
	ipePrometheusRuleName = "metal3-defaults"
	ipeMetricsPort        = "9608"
	ipeContainerName      = "metal3-ironic-prometheus-exporter"
	// Value of OS_SENSOR_DATA__INTERVAL when spec.prometheusExporter.sensorCollectionInterval
	// is unset (CRD default).
	ipeDefaultSensorInterval = "60"
	// Hardware payload series produced by IPE from Ironic BMC notifications.
	// ironic_* process timings on :9608 are not sufficient — those exist even
	// when get_sensors_data fails.
	ipeHardwarePayloadMetric = "baremetal_last_payload_timestamp_seconds"
)

func execInMetal3Ironic(oc *exutil.CLI, pod, command string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", machineAPINamespace, pod, "-c", "metal3-ironic", "--", "/bin/sh", "-c", command).Output()
}

func metal3IronicLogs(oc *exutil.CLI, pod string) (string, error) {
	return oc.AsAdmin().Run("logs").Args(pod, "-n", machineAPINamespace, "-c", "metal3-ironic", "--tail=8000").Output()
}

// bmhInfo is the subset of BareMetalHost fields the IPE tests compare.
type bmhInfo struct {
	Name              string
	ProvisioningState string
	OperationalStatus string
	BMCAddress        string
	InspectStart      string
}

// listBareMetalHosts returns all BMHs in openshift-machine-api. Fields are
// '|' separated because provisioning states such as "externally provisioned"
// contain spaces.
func listBareMetalHosts(oc *exutil.CLI) ([]bmhInfo, error) {
	out, err := oc.AsAdmin().Run("get").Args("bmh", "-n", machineAPINamespace,
		`-o=jsonpath={range .items[*]}{.metadata.name}{"|"}{.status.provisioning.state}{"|"}{.status.operationalStatus}{"|"}{.spec.bmc.address}{"|"}{.status.operationHistory.inspect.start}{"\n"}{end}`).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list BareMetalHosts: %w", err)
	}
	var hosts []bmhInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, "|")
		if len(fields) != 5 || fields[0] == "" {
			continue
		}
		hosts = append(hosts, bmhInfo{
			Name:              fields[0],
			ProvisioningState: fields[1],
			OperationalStatus: fields[2],
			BMCAddress:        fields[3],
			InspectStart:      fields[4],
		})
	}
	return hosts, nil
}

// isDeployedBMH reports whether Ironic holds the node in the active state.
// Ironic only collects sensor data for active (deployed) nodes; BMO adopts
// externally provisioned hosts (control plane) into active as well.
func isDeployedBMH(h bmhInfo) bool {
	return h.ProvisioningState == "provisioned" || h.ProvisioningState == "externally provisioned"
}

// isRedfishBMH reports whether the BMH uses a Redfish-based driver whose
// sensor payload IPE parses (redfish, redfish-virtualmedia, idrac-*).
func isRedfishBMH(h bmhInfo) bool {
	return strings.HasPrefix(h.BMCAddress, "redfish") || strings.HasPrefix(h.BMCAddress, "idrac")
}

func deployedBareMetalHosts(oc *exutil.CLI) []bmhInfo {
	hosts, err := listBareMetalHosts(oc)
	if err != nil {
		e2e.Logf("%v", err)
		return nil
	}
	var deployed []bmhInfo
	for _, h := range hosts {
		if isDeployedBMH(h) {
			deployed = append(deployed, h)
		}
	}
	return deployed
}

func countDeployedBareMetalHosts(oc *exutil.CLI) int {
	return len(deployedBareMetalHosts(oc))
}

// bmhSnapshot maps BMH name to the state that must not change when IPE is
// toggled (TC-IPE-046).
func bmhSnapshot(oc *exutil.CLI) map[string]bmhInfo {
	hosts, err := listBareMetalHosts(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	snap := make(map[string]bmhInfo, len(hosts))
	for _, h := range hosts {
		snap[h.Name] = h
	}
	return snap
}

// waitForBMHsUnchanged waits until every BMH is back to its snapshot
// provisioning state and operational status, and fails immediately if any
// host was re-inspected. The metal3 pod restart makes BMO re-adopt hosts, so
// a short transition is tolerated, but the hosts must settle to their
// original state.
func waitForBMHsUnchanged(oc *exutil.CLI, before map[string]bmhInfo, timeout time.Duration) {
	err := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, timeout, true, func(context.Context) (bool, error) {
		hosts, lerr := listBareMetalHosts(oc)
		if lerr != nil {
			e2e.Logf("%v", lerr)
			return false, nil
		}
		settled := true
		for _, h := range hosts {
			prev, ok := before[h.Name]
			if !ok {
				continue
			}
			if h.InspectStart != prev.InspectStart {
				return false, fmt.Errorf("BMH %s was re-inspected (inspect start %q -> %q)", h.Name, prev.InspectStart, h.InspectStart)
			}
			if h.ProvisioningState != prev.ProvisioningState || h.OperationalStatus != prev.OperationalStatus {
				e2e.Logf("BMH %s not settled: state %q/%q, want %q/%q", h.Name, h.ProvisioningState, h.OperationalStatus, prev.ProvisioningState, prev.OperationalStatus)
				settled = false
			}
		}
		return settled, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "BareMetalHosts did not return to their pre-IPE state")
}

func metal3PodContainerNames(oc *exutil.CLI, pod string) []string {
	out, err := oc.AsAdmin().Run("get").Args("pod", pod, "-n", machineAPINamespace, "-o=jsonpath={.spec.containers[*].name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get containers of pod %s", pod)
	return strings.Fields(out)
}

func assertSensorDataEnabled(oc *exutil.CLI, pod string) {
	send, err := execInMetal3Ironic(oc, pod, "printenv SEND_SENSOR_DATA")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to read SEND_SENSOR_DATA from metal3-ironic")
	o.Expect(strings.TrimSpace(send)).To(o.Equal("true"), "expected metal3-ironic SEND_SENSOR_DATA=true when IPE is enabled, got %q", send)

	conf, err := execInMetal3Ironic(oc, pod, "grep -E '^send_sensor_data' /conf/ironic/ironic.conf")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to read send_sensor_data from ironic.conf")
	o.Expect(conf).To(o.ContainSubstring("true"), "expected ironic.conf send_sensor_data=true when IPE is enabled, got %q", conf)
}

func sensorCollectionErrorLines(logs string) []string {
	var hits []string
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "got AsyncResource") ||
			strings.Contains(line, "could not get sensor data") ||
			strings.Contains(line, "Failed to get sensor data") {
			hits = append(hits, line)
			if len(hits) >= 8 {
				break
			}
		}
	}
	return hits
}

func hasSensorCollectionAttempt(logs string) bool {
	if strings.Contains(logs, "queued for async sensor data collection") {
		return true
	}
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "Completed sending sensor data") &&
			!strings.Contains(line, "evaluated 0 sync nodes with 0 workers and 0 async nodes") {
			return true
		}
	}
	return false
}

func waitForSensorCollectionAttempt(oc *exutil.CLI, pod string, timeout time.Duration) {
	err := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, timeout, true, func(context.Context) (bool, error) {
		logs, lerr := metal3IronicLogs(oc, pod)
		if lerr != nil {
			e2e.Logf("Failed to get metal3-ironic logs: %v", lerr)
			return false, nil
		}
		if errs := sensorCollectionErrorLines(logs); len(errs) > 0 {
			return false, fmt.Errorf("Ironic sensor collection failed:\n%s", strings.Join(errs, "\n"))
		}
		if hasSensorCollectionAttempt(logs) {
			e2e.Logf("Ironic _send_sensor_data evaluated provisioned nodes")
			return true, nil
		}
		e2e.Logf("Waiting for Ironic _send_sensor_data to evaluate provisioned nodes")
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Ironic _send_sensor_data did not evaluate any provisioned nodes (or collection failed)")
}

func waitForIPEHardwareMetrics(oc *exutil.CLI, pod string, timeout time.Duration) {
	metricsCmd := fmt.Sprintf("curl -s http://metal3-state.openshift-machine-api.svc:%s/metrics | grep -E '%s' | head -20", ipeMetricsPort, ipeHardwarePayloadMetric)
	err := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, timeout, true, func(context.Context) (bool, error) {
		logs, lerr := metal3IronicLogs(oc, pod)
		if lerr == nil {
			if errs := sensorCollectionErrorLines(logs); len(errs) > 0 {
				return false, fmt.Errorf("Ironic sensor collection failed before IPE hardware metrics appeared:\n%s", strings.Join(errs, "\n"))
			}
		}

		out, merr := execInMetal3Ironic(oc, pod, metricsCmd)
		if merr != nil || strings.TrimSpace(out) == "" {
			e2e.Logf("IPE hardware payload metric %s not present yet: %v", ipeHardwarePayloadMetric, merr)
			return false, nil
		}
		e2e.Logf("IPE hardware payload metrics:\n%s", out)
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("IPE did not expose %s; ironic_* process timings are not hardware sensor data", ipeHardwarePayloadMetric))
}

func verifyIPESensorCollection(oc *exutil.CLI, pod string) {
	assertSensorDataEnabled(oc, pod)
	waitForSensorCollectionAttempt(oc, pod, 3*time.Minute)
	waitForIPEHardwareMetrics(oc, pod, 3*time.Minute)
}

func getPrometheusExporterEnabled(oc *exutil.CLI) string {
	state, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.spec.prometheusExporter.enabled}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return state
}

func patchPrometheusExporter(oc *exutil.CLI, enabled bool) {
	patch := fmt.Sprintf(`{"spec":{"prometheusExporter":{"enabled":%t}}}`, enabled)
	err := oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", patch).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func restorePrometheusExporter(oc *exutil.CLI, originalExporterState string) {
	var patch string
	if originalExporterState == "" || originalExporterState == "false" {
		e2e.Logf("Restoring prometheusExporter to disabled state")
		patch = `{"spec":{"prometheusExporter":{"enabled":false}}}`
	} else if originalExporterState == "true" {
		e2e.Logf("Restoring prometheusExporter to enabled state")
		patch = `{"spec":{"prometheusExporter":{"enabled":true}}}`
	} else {
		return
	}

	restoreErr := oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", patch).Execute()
	o.Expect(restoreErr).NotTo(o.HaveOccurred(), "failed to restore prometheusExporter state")
	e2e.Logf("Waiting for metal3 deployment to be available after restore")
	waitForDeployStatus(oc, "metal3", machineAPINamespace, "True")
}

func ipeServiceMonitorExists(oc *exutil.CLI) bool {
	out, err := oc.AsAdmin().Run("get").Args("servicemonitor", ipeServiceMonitorName, "-n", machineAPINamespace, "--ignore-not-found", "-o=name").Output()
	return err == nil && strings.TrimSpace(out) != ""
}

func ipePrometheusRuleExists(oc *exutil.CLI) bool {
	out, err := oc.AsAdmin().Run("get").Args("prometheusrule", ipePrometheusRuleName, "-n", machineAPINamespace, "--ignore-not-found", "-o=name").Output()
	return err == nil && strings.TrimSpace(out) != ""
}

func metal3StateMetricsPort(oc *exutil.CLI) string {
	port, err := oc.AsAdmin().Run("get").Args("svc", "metal3-state", "-n", machineAPINamespace, `-o=jsonpath={.spec.ports[?(@.name=="metrics")].port}`).Output()
	if err != nil {
		e2e.Logf("Failed to get metal3-state metrics port: %v", err)
		return ""
	}
	return strings.TrimSpace(port)
}

func waitForIPEMonitoringResources(oc *exutil.CLI, wantPresent bool) {
	err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 180*time.Second, true, func(context.Context) (bool, error) {
		sm := ipeServiceMonitorExists(oc)
		pr := ipePrometheusRuleExists(oc)
		port := metal3StateMetricsPort(oc)
		e2e.Logf("IPE monitoring resources: servicemonitor=%v prometheusrule=%v metricsPort=%q (wantPresent=%v)", sm, pr, port, wantPresent)
		if wantPresent {
			return sm && pr && port == ipeMetricsPort, nil
		}
		return !sm && !pr && port == "", nil
	})
	if wantPresent {
		compat_otp.AssertWaitPollNoErr(err, "IPE ServiceMonitor, PrometheusRule, or metal3-state metrics port 9608 were not created")
		return
	}
	compat_otp.AssertWaitPollNoErr(err, "IPE ServiceMonitor, PrometheusRule, or metal3-state metrics port were not removed")
}

func waitForMetal3DefaultAlertRules(oc *exutil.CLI) {
	requiredAlerts := []string{
		"BaremetalTemperatureHealth",
		"BaremetalPowerHealth",
		"BaremetalFanHealth",
		"BaremetalDriveHealth",
		"BaremetalStaleMetrics",
	}

	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 3*time.Minute, true, func(context.Context) (bool, error) {
		resp, rerr := getPrometheusAlertRules(oc)
		if rerr != nil {
			e2e.Logf("Failed to get prometheus alert rules: %v", rerr)
			return false, nil
		}

		foundHealth := false
		foundMonitoring := false
		foundAlerts := map[string]string{}
		for _, group := range resp.Data.Groups {
			if !strings.Contains(group.File, ipePrometheusRuleName) {
				continue
			}
			if group.Name == "baremetal.health" {
				foundHealth = true
			}
			if group.Name == "baremetal.monitoring" {
				foundMonitoring = true
			}
			for _, rule := range group.Rules {
				foundAlerts[rule.Name] = rule.Query
			}
		}

		if !foundHealth || !foundMonitoring {
			e2e.Logf("metal3-defaults groups not loaded yet health=%v monitoring=%v", foundHealth, foundMonitoring)
			return false, nil
		}
		for _, name := range requiredAlerts {
			if _, ok := foundAlerts[name]; !ok {
				e2e.Logf("alert %s not loaded from metal3-defaults yet", name)
				return false, nil
			}
		}
		if !strings.Contains(foundAlerts["BaremetalStaleMetrics"], "baremetal_last_payload_timestamp_seconds") || !strings.Contains(foundAlerts["BaremetalStaleMetrics"], "> 300") {
			e2e.Logf("BaremetalStaleMetrics expr not as expected: %s", foundAlerts["BaremetalStaleMetrics"])
			return false, nil
		}

		e2e.Logf("Loaded metal3-defaults alert groups health=%v monitoring=%v alerts=%v", foundHealth, foundMonitoring, foundAlerts)
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "CMO Prometheus did not load metal3-defaults baremetal.health/baremetal.monitoring alert groups")
}

// ipeNodeNameMatchesBMH reports whether an IPE node_name label refers to the
// given BMH. BMO names Ironic nodes "<namespace>~<bmh name>".
func ipeNodeNameMatchesBMH(nodeName, bmh string) bool {
	return nodeName == bmh || strings.HasSuffix(nodeName, "~"+bmh)
}

// waitForIPESeriesForHosts waits until CMO Prometheus has a
// baremetal_last_payload_timestamp_seconds series for every given BMH.
func waitForIPESeriesForHosts(oc *exutil.CLI, hosts []bmhInfo, timeout time.Duration) {
	query := fmt.Sprintf(`%s{job="metal3-state"}`, ipeHardwarePayloadMetric)
	err := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, timeout, true, func(context.Context) (bool, error) {
		series, qerr := queryPrometheusSeriesLabels(oc, query)
		if qerr != nil {
			e2e.Logf("%v", qerr)
			return false, nil
		}
		var missing []string
		for _, h := range hosts {
			found := false
			for _, s := range series {
				if ipeNodeNameMatchesBMH(s["node_name"], h.Name) {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, h.Name)
			}
		}
		if len(missing) > 0 {
			e2e.Logf("No %s series yet for BMHs %v", ipeHardwarePayloadMetric, missing)
			return false, nil
		}
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("CMO did not scrape %s for every Redfish BMH", ipeHardwarePayloadMetric))
}

func getPrometheusExporterField(oc *exutil.CLI, field string) string {
	val, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", fmt.Sprintf("-o=jsonpath={.spec.prometheusExporter.%s}", field)).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to read spec.prometheusExporter.%s", field)
	return strings.TrimSpace(val)
}

// patchPrometheusExporterField merge-patches spec.prometheusExporter.<field>
// with a raw JSON value and returns the CLI output and error so callers can
// assert on API validation rejections.
func patchPrometheusExporterField(oc *exutil.CLI, field, rawJSONValue string) (string, error) {
	patch := fmt.Sprintf(`{"spec":{"prometheusExporter":{%q:%s}}}`, field, rawJSONValue)
	return oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", patch).Output()
}

// restorePrometheusExporterField puts a field back to its original value, or
// removes it (letting the CRD default apply) if it was unset.
// The original value must be a JSON literal (number or bool) as returned by
// getPrometheusExporterField.
func restorePrometheusExporterField(oc *exutil.CLI, field, original string) {
	value := "null"
	if original != "" {
		value = original
	}
	_, err := patchPrometheusExporterField(oc, field, value)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore spec.prometheusExporter.%s", field)
}

func metal3IronicEnv(oc *exutil.CLI, pod, name string) string {
	out, err := execInMetal3Ironic(oc, pod, "printenv "+name)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to read %s from metal3-ironic", name)
	return strings.TrimSpace(out)
}

// waitForIPEPrometheusRuleState waits for the metal3-defaults PrometheusRule
// to be present or absent while the ServiceMonitor and metrics port stay.
func waitForIPEPrometheusRuleState(oc *exutil.CLI, wantRule bool) {
	err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 180*time.Second, true, func(context.Context) (bool, error) {
		sm := ipeServiceMonitorExists(oc)
		pr := ipePrometheusRuleExists(oc)
		port := metal3StateMetricsPort(oc)
		e2e.Logf("servicemonitor=%v prometheusrule=%v metricsPort=%q (wantRule=%v)", sm, pr, port, wantRule)
		if !sm || port != ipeMetricsPort {
			return false, fmt.Errorf("ServiceMonitor or metrics port removed while only default rules were toggled (servicemonitor=%v port=%q)", sm, port)
		}
		return pr == wantRule, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("PrometheusRule %s presence did not become %v", ipePrometheusRuleName, wantRule))
}

// waitForMetal3DefaultRulesUnloaded waits until CMO Prometheus no longer
// serves any rule group from the metal3-defaults PrometheusRule.
func waitForMetal3DefaultRulesUnloaded(oc *exutil.CLI) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
		resp, rerr := getPrometheusAlertRules(oc)
		if rerr != nil {
			e2e.Logf("Failed to get prometheus alert rules: %v", rerr)
			return false, nil
		}
		for _, group := range resp.Data.Groups {
			if strings.Contains(group.File, ipePrometheusRuleName) {
				e2e.Logf("Prometheus still serves group %s from %s", group.Name, group.File)
				return false, nil
			}
		}
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "CMO Prometheus still serves metal3-defaults rule groups after disableDefaultPrometheusRules=true")
}

// setBMHDetached adds or removes the detached annotation. Removal is
// idempotent so it can run both in the test body and in deferred cleanup.
func setBMHDetached(oc *exutil.CLI, bmh string, detached bool) {
	arg := "baremetalhost.metal3.io/detached="
	if !detached {
		arg = "baremetalhost.metal3.io/detached-"
	}
	out, err := oc.AsAdmin().Run("annotate").Args("bmh", bmh, "-n", machineAPINamespace, arg, "--overwrite").Output()
	if err != nil && !detached && strings.Contains(out, "not found") && !strings.Contains(out, "baremetalhosts") {
		e2e.Logf("detached annotation already absent on BMH %s", bmh)
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to set detached=%v on BMH %s: %s", detached, bmh, out)
}

// staleAlertFiringNodes returns the node_name labels of firing
// BaremetalStaleMetrics alerts.
func staleAlertFiringNodes(oc *exutil.CLI) ([]string, error) {
	series, err := queryPrometheusSeriesLabels(oc, `ALERTS{alertname="BaremetalStaleMetrics",alertstate="firing"}`)
	if err != nil {
		return nil, err
	}
	nodes := make([]string, 0, len(series))
	for _, s := range series {
		nodes = append(nodes, s["node_name"])
	}
	return nodes, nil
}
