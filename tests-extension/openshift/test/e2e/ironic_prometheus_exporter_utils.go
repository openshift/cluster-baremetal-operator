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
)

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
	if restoreErr != nil {
		e2e.Logf("Warning: Failed to restore prometheusExporter state: %v", restoreErr)
		return
	}
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
