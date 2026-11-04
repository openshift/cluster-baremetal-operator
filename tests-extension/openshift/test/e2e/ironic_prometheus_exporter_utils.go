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

func countProvisionedBareMetalHosts(oc *exutil.CLI) int {
	out, err := oc.AsAdmin().Run("get").Args("bmh", "-n", machineAPINamespace, "-o=jsonpath={range .items[*]}{.metadata.name}{\" \"}{.status.provisioning.state}{\"\\n\"}{end}").Output()
	if err != nil {
		e2e.Logf("Failed to list BareMetalHosts: %v", err)
		return 0
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "provisioned" {
			count++
		}
	}
	return count
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
