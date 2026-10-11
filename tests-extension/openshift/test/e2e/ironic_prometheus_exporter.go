package baremetal

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal][Level0] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc = compat_otp.NewCLI("ironic-prometheus-exporter", compat_otp.KubeConfigPath())
	)
	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Medium-88191-Verify Ironic Prometheus Exporter can be enabled and metrics are exposed [Serial]", func() {
		g.By("Save current provisioning-configuration")
		_, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=yaml").OutputToFile("prov-backup.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())

		originalExporterState := getPrometheusExporterEnabled(oc)
		e2e.Logf("Original prometheusExporter enabled state: %s", originalExporterState)

		defer func() {
			g.By("Restore original provisioning configuration")
			restorePrometheusExporter(oc, originalExporterState)

			cboStatus, err := checkOperator(oc, "baremetal")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(cboStatus).To(o.BeTrue())
		}()

		g.By("Get current metal3 pod name before enabling IPE")
		oldMetal3Pod := getMetal3StatePod(oc)
		e2e.Logf("Current metal3 pod before enabling IPE: %s", oldMetal3Pod)

		g.By("Enable Ironic Prometheus Exporter via provisioning configuration patch")
		patchPrometheusExporter(oc, true)

		g.By("Verify prometheusExporter is enabled in provisioning configuration")
		o.Expect(getPrometheusExporterEnabled(oc)).Should(o.Equal("true"))

		g.By("Wait for metal3 pod to be recreated after enabling IPE")
		metal3Pod := waitForMetal3PodRecreated(oc, oldMetal3Pod, "enabling IPE")
		e2e.Logf("Metal3 pod recreated: %s", metal3Pod)

		g.By("Verify metal3-state service exists")
		metal3Svc, err := oc.AsAdmin().Run("get").Args("service", "-n", machineAPINamespace, "metal3-state", "-o=jsonpath={.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metal3Svc).Should(o.Equal("metal3-state"))
		e2e.Logf("Metal3 service: %s", metal3Svc)

		g.By("Verify Ironic Prometheus Exporter metrics endpoint is accessible")
		err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 120*time.Second, true, func(context.Context) (bool, error) {
			metricsCmd := "curl -s http://metal3-state.openshift-machine-api.svc:9608/metrics"
			cmd := []string{"-n", machineAPINamespace, metal3Pod, "-c", "metal3-ironic", "--", "/bin/sh", "-c", metricsCmd}
			metrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(cmd...).Output()
			if err != nil {
				e2e.Logf("Failed to curl metrics endpoint: %v", err)
				return false, nil
			}

			if metrics == "" {
				e2e.Logf("Metrics endpoint returned empty response, retrying...")
				return false, nil
			}

			if !strings.Contains(metrics, "# HELP") && !strings.Contains(metrics, "# TYPE") {
				e2e.Logf("Metrics endpoint did not return Prometheus format, retrying...")
				return false, nil
			}

			e2e.Logf("Ironic Prometheus Exporter metrics endpoint is accessible and returning data")
			return true, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Failed to access metrics endpoint or metrics not available")

		g.By("Verify Ironic-specific metrics are exposed")
		metricsCmd := "curl -s http://metal3-state.openshift-machine-api.svc:9608/metrics | grep -E 'baremetal_|ironic_' | head -20"
		cmd := []string{"-n", machineAPINamespace, metal3Pod, "-c", "metal3-ironic", "--", "/bin/sh", "-c", metricsCmd}
		ironicMetrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(cmd...).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ironicMetrics).ShouldNot(o.BeEmpty(), "Expected to find Ironic-specific metrics (baremetal_* or ironic_*)")
		e2e.Logf("Sample Ironic metrics found:\n%s", ironicMetrics)

		g.By("Verify cluster baremetal operator is still available after enabling exporter")
		cboStatus, err := checkOperator(oc, "baremetal")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cboStatus).To(o.BeTrue())

		g.By("Test disabling Ironic Prometheus Exporter")
		oldMetal3PodBeforeDisable := metal3Pod
		e2e.Logf("Current metal3 pod before disabling IPE: %s", oldMetal3PodBeforeDisable)

		patchPrometheusExporter(oc, false)

		g.By("Verify prometheusExporter is disabled in provisioning configuration")
		o.Expect(getPrometheusExporterEnabled(oc)).Should(o.Equal("false"))

		g.By("Wait for metal3 pod to be recreated after disabling IPE")
		newMetal3PodAfterDisable := waitForMetal3PodRecreated(oc, oldMetal3PodBeforeDisable, "disabling IPE")

		g.By("Verify metrics endpoint is no longer accessible after disabling")
		err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 60*time.Second, true, func(context.Context) (bool, error) {
			metricsCmd := "curl -s -o /dev/null -w '%{http_code}' http://metal3-state.openshift-machine-api.svc:9608/metrics"
			cmd := []string{"-n", machineAPINamespace, newMetal3PodAfterDisable, "-c", "metal3-ironic", "--", "/bin/sh", "-c", metricsCmd}
			httpCode, execErr := oc.AsAdmin().WithoutNamespace().Run("exec").Args(cmd...).Output()

			if execErr != nil || httpCode != "200" {
				e2e.Logf("Metrics endpoint is no longer accessible (HTTP code: %s, error: %v) - expected", httpCode, execErr)
				return true, nil
			}

			e2e.Logf("Metrics endpoint still accessible (HTTP %s), waiting for it to become inaccessible...", httpCode)
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Metrics endpoint is still accessible after disabling IPE - expected it to become inaccessible")
		e2e.Logf("Metrics endpoint is no longer accessible after disabling IPE (expected)")

		g.By("Verify cluster baremetal operator is still available after disabling exporter")
		cboStatus, err = checkOperator(oc, "baremetal")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cboStatus).To(o.BeTrue())
	})

	// author: jhajyahy@redhat.com
	g.Context("IPE CMO integration [Serial]", g.Serial, g.Ordered, func() {
		var originalExporterState string

		g.BeforeAll(func() {
			SkipIfNotBaremetalCluster(oc)

			originalExporterState = getPrometheusExporterEnabled(oc)
			e2e.Logf("Original prometheusExporter enabled state: %s", originalExporterState)

			if originalExporterState != "true" {
				oldMetal3Pod := getMetal3StatePod(oc)
				g.By("Enable Ironic Prometheus Exporter")
				patchPrometheusExporter(oc, true)
				o.Expect(getPrometheusExporterEnabled(oc)).Should(o.Equal("true"))
				waitForMetal3PodRecreated(oc, oldMetal3Pod, "enabling IPE")
			}

			waitForIPEMonitoringResources(oc, true)
		})

		g.AfterAll(func() {
			g.By("Restore original prometheusExporter state")
			restorePrometheusExporter(oc, originalExporterState)

			cboStatus, err := checkOperator(oc, "baremetal")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(cboStatus).To(o.BeTrue())

			if originalExporterState == "" || originalExporterState == "false" {
				g.By("Verify ServiceMonitor, PrometheusRule and metrics port are removed after IPE is disabled")
				waitForIPEMonitoringResources(oc, false)
			}
		})

		g.It("Author:jhajyahy-Medium-Verify IPE ServiceMonitor PrometheusRule and metrics port are created [Serial]", func() {
			g.By("Verify metal3-ironic-prometheus-exporter ServiceMonitor exists")
			o.Expect(ipeServiceMonitorExists(oc)).To(o.BeTrue(), "expected ServiceMonitor %s in %s", ipeServiceMonitorName, machineAPINamespace)

			g.By("Verify metal3-defaults PrometheusRule exists")
			o.Expect(ipePrometheusRuleExists(oc)).To(o.BeTrue(), "expected PrometheusRule %s in %s", ipePrometheusRuleName, machineAPINamespace)

			g.By("Verify metal3-state service exposes metrics port 9608")
			o.Expect(metal3StateMetricsPort(oc)).To(o.Equal(ipeMetricsPort))
		})

		g.It("Author:jhajyahy-Medium-Verify CMO scrapes IPE metrics [Serial]", func() {
			g.By("Wait for CMO Prometheus to scrape metal3-state metrics")
			up := waitForPrometheusQuery(oc, `up{job="metal3-state"}`, 3*time.Minute)
			upValue, ok := up.Data.Result[0].Value[1].(string)
			o.Expect(ok).To(o.BeTrue(), "up{job=\"metal3-state\"} value is not a string")
			o.Expect(upValue).To(o.Equal("1"), "expected CMO scrape target job=metal3-state to be up")
			e2e.Logf("CMO scrape target is up: %+v", up.Data.Result[0].Metric)

			g.By("Verify CMO has ironic_* or baremetal_last_payload_timestamp_seconds series")
			series := waitForPrometheusQuery(oc, `{job="metal3-state",__name__=~"ironic_.*|baremetal_last_payload_timestamp_seconds"}`, 2*time.Minute)
			o.Expect(series.Data.Result).ShouldNot(o.BeEmpty(), "expected CMO to scrape ironic_* or baremetal_last_payload_timestamp_seconds")
			e2e.Logf("CMO scraped %d IPE series", len(series.Data.Result))
		})

		g.It("Author:jhajyahy-Medium-Verify default Baremetal PrometheusRule groups are loaded [Serial]", func() {
			g.By("Verify PrometheusRule CR has baremetal.health and baremetal.monitoring groups")
			groupNames, err := oc.AsAdmin().Run("get").Args("prometheusrule", ipePrometheusRuleName, "-n", machineAPINamespace, "-o=jsonpath={.spec.groups[*].name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(groupNames).To(o.ContainSubstring("baremetal.health"))
			o.Expect(groupNames).To(o.ContainSubstring("baremetal.monitoring"))

			g.By("Verify BaremetalStaleMetrics uses a 5m gap on last payload timestamp")
			staleExpr, err := oc.AsAdmin().Run("get").Args("prometheusrule", ipePrometheusRuleName, "-n", machineAPINamespace, `-o=jsonpath={.spec.groups[?(@.name=="baremetal.monitoring")].rules[0].expr}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(staleExpr).To(o.ContainSubstring("baremetal_last_payload_timestamp_seconds"))
			o.Expect(staleExpr).To(o.ContainSubstring("> 300"))

			staleFor, err := oc.AsAdmin().Run("get").Args("prometheusrule", ipePrometheusRuleName, "-n", machineAPINamespace, `-o=jsonpath={.spec.groups[?(@.name=="baremetal.monitoring")].rules[0].for}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(staleFor).To(o.Equal("5m"))

			g.By("Verify CMO Prometheus loaded the metal3-defaults alert groups")
			waitForMetal3DefaultAlertRules(oc)
		})
	})
})
