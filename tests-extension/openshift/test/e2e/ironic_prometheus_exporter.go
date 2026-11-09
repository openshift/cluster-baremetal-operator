package baremetal

import (
	"context"
	"fmt"
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

		g.By("Record BareMetalHost state before Day-2 enablement")
		bmhsBefore := bmhSnapshot(oc)
		e2e.Logf("BareMetalHosts before enabling IPE: %v", bmhsBefore)

		g.By("Enable Ironic Prometheus Exporter via provisioning configuration patch")
		patchPrometheusExporter(oc, true)

		g.By("Verify prometheusExporter is enabled in provisioning configuration")
		o.Expect(getPrometheusExporterEnabled(oc)).Should(o.Equal("true"))

		g.By("Wait for metal3 pod to be recreated after enabling IPE")
		metal3Pod := waitForMetal3PodRecreated(oc, oldMetal3Pod, "enabling IPE")
		e2e.Logf("Metal3 pod recreated: %s", metal3Pod)

		g.By("Verify metal3 pod gained the ironic-prometheus-exporter container")
		o.Expect(metal3PodContainerNames(oc, metal3Pod)).To(o.ContainElement(ipeContainerName), "expected container %s in pod %s after enabling IPE", ipeContainerName, metal3Pod)

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

		g.By("Verify Day-2 enablement did not change BareMetalHost state or trigger re-inspection")
		waitForBMHsUnchanged(oc, bmhsBefore, 10*time.Minute)

		if countDeployedBareMetalHosts(oc) > 0 {
			g.By("Verify Ironic collects BMC sensor data and IPE exposes hardware payload metrics")
			verifyIPESensorCollection(oc, metal3Pod)
		} else {
			e2e.Logf("No deployed BareMetalHosts; Ironic only collects sensor data for active nodes, skipping hardware payload check")
		}

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

		g.By("Verify metal3 pod no longer has the ironic-prometheus-exporter container")
		o.Expect(metal3PodContainerNames(oc, newMetal3PodAfterDisable)).NotTo(o.ContainElement(ipeContainerName), "expected container %s removed from pod %s after disabling IPE", ipeContainerName, newMetal3PodAfterDisable)

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
			defer func() {
				g.By("Restore original prometheusExporter state")
				restorePrometheusExporter(oc, originalExporterState)

				cboStatus, err := checkOperator(oc, "baremetal")
				o.Expect(err).NotTo(o.HaveOccurred(), "failed to check baremetal ClusterOperator after restore")
				o.Expect(cboStatus).To(o.BeTrue(), "expected baremetal ClusterOperator available after restore")
			}()

			g.By("Disable Ironic Prometheus Exporter")
			patchPrometheusExporter(oc, false)
			g.By("Verify ServiceMonitor, PrometheusRule and metrics port are removed after IPE is disabled")
			waitForIPEMonitoringResources(oc, false)
		})

		g.It("Author:jhajyahy-Medium-90681-Verify IPE ServiceMonitor PrometheusRule and metrics port are created [Serial]", func() {
			g.By("Verify metal3-ironic-prometheus-exporter ServiceMonitor exists")
			o.Expect(ipeServiceMonitorExists(oc)).To(o.BeTrue(), "expected ServiceMonitor %s in %s", ipeServiceMonitorName, machineAPINamespace)

			g.By("Verify metal3-defaults PrometheusRule exists")
			o.Expect(ipePrometheusRuleExists(oc)).To(o.BeTrue(), "expected PrometheusRule %s in %s", ipePrometheusRuleName, machineAPINamespace)

			g.By("Verify metal3-state service exposes metrics port 9608")
			o.Expect(metal3StateMetricsPort(oc)).To(o.Equal(ipeMetricsPort), "expected metal3-state metrics port %s in %s", ipeMetricsPort, machineAPINamespace)
		})

		g.It("Author:jhajyahy-Medium-90683-Verify default Baremetal PrometheusRule groups are loaded [Serial]", func() {
			g.By("Verify PrometheusRule CR has baremetal.health and baremetal.monitoring groups")
			groupNames, err := oc.AsAdmin().Run("get").Args("prometheusrule", ipePrometheusRuleName, "-n", machineAPINamespace, "-o=jsonpath={.spec.groups[*].name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get PrometheusRule %s group names", ipePrometheusRuleName)
			o.Expect(groupNames).To(o.ContainSubstring("baremetal.health"), "expected PrometheusRule %s to include group baremetal.health, got %q", ipePrometheusRuleName, groupNames)
			o.Expect(groupNames).To(o.ContainSubstring("baremetal.monitoring"), "expected PrometheusRule %s to include group baremetal.monitoring, got %q", ipePrometheusRuleName, groupNames)

			g.By("Verify BaremetalStaleMetrics uses a 5m gap on last payload timestamp")
			staleExpr, err := oc.AsAdmin().Run("get").Args("prometheusrule", ipePrometheusRuleName, "-n", machineAPINamespace, `-o=jsonpath={.spec.groups[?(@.name=="baremetal.monitoring")].rules[?(@.alert=="BaremetalStaleMetrics")].expr}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get BaremetalStaleMetrics expr from PrometheusRule %s", ipePrometheusRuleName)
			o.Expect(staleExpr).To(o.ContainSubstring("baremetal_last_payload_timestamp_seconds"), "expected BaremetalStaleMetrics expr to use baremetal_last_payload_timestamp_seconds, got %q", staleExpr)
			o.Expect(staleExpr).To(o.ContainSubstring("> 300"), "expected BaremetalStaleMetrics expr to use a 300s gap, got %q", staleExpr)

			staleFor, err := oc.AsAdmin().Run("get").Args("prometheusrule", ipePrometheusRuleName, "-n", machineAPINamespace, `-o=jsonpath={.spec.groups[?(@.name=="baremetal.monitoring")].rules[?(@.alert=="BaremetalStaleMetrics")].for}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get BaremetalStaleMetrics for from PrometheusRule %s", ipePrometheusRuleName)
			o.Expect(staleFor).To(o.Equal("5m"), "expected BaremetalStaleMetrics for=5m, got %q", staleFor)

			g.By("Verify CMO Prometheus loaded the metal3-defaults alert groups")
			waitForMetal3DefaultAlertRules(oc)
		})

		g.It("Author:jhajyahy-Medium-90726-Verify Ironic collects BMC sensor data and IPE exports hardware metrics [Serial]", func() {
			if countDeployedBareMetalHosts(oc) == 0 {
				g.Skip("No provisioned or externally provisioned BareMetalHosts; Ironic skips undeployed nodes for sensor collection")
			}

			metal3Pod := getMetal3StatePod(oc)
			e2e.Logf("Checking IPE sensor collection on metal3 pod %s", metal3Pod)
			verifyIPESensorCollection(oc, metal3Pod)
		})

		g.It("Author:jhajyahy-Medium-90682-Verify CMO scrapes IPE metrics [Serial]", func() {
			g.By("Wait for CMO Prometheus to scrape metal3-state metrics")
			up := waitForPrometheusQuery(oc, `up{job="metal3-state"}`, 3*time.Minute)
			upValue, ok := up.Data.Result[0].Value[1].(string)
			o.Expect(ok).To(o.BeTrue(), "up{job=\"metal3-state\"} value is not a string")
			o.Expect(upValue).To(o.Equal("1"), "expected CMO scrape target job=metal3-state to be up")
			e2e.Logf("CMO scrape target job=metal3-state is up")

			var redfishHosts []bmhInfo
			for _, h := range deployedBareMetalHosts(oc) {
				if isRedfishBMH(h) {
					redfishHosts = append(redfishHosts, h)
				}
			}
			if len(redfishHosts) == 0 {
				e2e.Logf("No deployed Redfish BareMetalHosts; skipping per-host hardware payload check")
				return
			}

			g.By("Verify CMO scraped an IPE hardware payload series for every deployed Redfish BMH")
			e2e.Logf("Deployed Redfish BMHs: %v", redfishHosts)
			waitForIPESeriesForHosts(oc, redfishHosts, 5*time.Minute)
		})

		g.It("Author:jhajyahy-Medium-91036-Verify sensorCollectionInterval is validated and applied to Ironic [Serial]", func() {
			originalInterval := getPrometheusExporterField(oc, "sensorCollectionInterval")
			effectiveOriginal := originalInterval
			if effectiveOriginal == "" {
				effectiveOriginal = ipeDefaultSensorInterval
			}
			e2e.Logf("Original sensorCollectionInterval: %q (effective %s)", originalInterval, effectiveOriginal)

			g.By("Verify CBO passes the configured interval to Ironic")
			metal3Pod := getMetal3StatePod(oc)
			o.Expect(metal3IronicEnv(oc, metal3Pod, "OS_SENSOR_DATA__INTERVAL")).To(o.Equal(effectiveOriginal), "expected OS_SENSOR_DATA__INTERVAL to match spec.prometheusExporter.sensorCollectionInterval")

			g.By("Verify the API rejects an interval below 60 seconds")
			out, err := patchPrometheusExporterField(oc, "sensorCollectionInterval", "59")
			o.Expect(err).To(o.HaveOccurred(), "expected sensorCollectionInterval=59 to be rejected, got: %s", out)
			o.Expect(out).To(o.ContainSubstring("60"), "expected rejection to mention the 60s minimum, got: %s", out)
			o.Expect(getPrometheusExporterField(oc, "sensorCollectionInterval")).To(o.Equal(originalInterval), "rejected patch must not change the interval")

			newInterval := "120"
			if effectiveOriginal == newInterval {
				newInterval = "180"
			}

			defer func() {
				current := getPrometheusExporterField(oc, "sensorCollectionInterval")
				if current == originalInterval {
					return
				}
				g.By("Restore original sensorCollectionInterval")
				podBeforeRestore := getMetal3StatePod(oc)
				restorePrometheusExporterField(oc, "sensorCollectionInterval", originalInterval)
				waitForMetal3PodRecreated(oc, podBeforeRestore, "restoring sensorCollectionInterval")
			}()

			g.By("Set sensorCollectionInterval to " + newInterval)
			_, err = patchPrometheusExporterField(oc, "sensorCollectionInterval", newInterval)
			o.Expect(err).NotTo(o.HaveOccurred())
			metal3Pod = waitForMetal3PodRecreated(oc, metal3Pod, "changing sensorCollectionInterval")
			o.Expect(metal3IronicEnv(oc, metal3Pod, "OS_SENSOR_DATA__INTERVAL")).To(o.Equal(newInterval), "expected OS_SENSOR_DATA__INTERVAL to follow the new interval")

			g.By("Verify the interval survives a metal3 pod restart")
			err = oc.AsAdmin().Run("delete").Args("pod", metal3Pod, "-n", machineAPINamespace).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			metal3Pod = waitForMetal3PodRecreated(oc, metal3Pod, "deleting metal3 pod")
			o.Expect(metal3IronicEnv(oc, metal3Pod, "OS_SENSOR_DATA__INTERVAL")).To(o.Equal(newInterval), "expected OS_SENSOR_DATA__INTERVAL to persist across metal3 pod restart")

			g.By("Verify cluster baremetal operator is available")
			cboStatus, err := checkOperator(oc, "baremetal")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(cboStatus).To(o.BeTrue())
		})

		g.It("Author:jhajyahy-Medium-91037-Verify disableDefaultPrometheusRules removes only the default PrometheusRule [Serial]", func() {
			originalDisable := getPrometheusExporterField(oc, "disableDefaultPrometheusRules")
			e2e.Logf("Original disableDefaultPrometheusRules: %q", originalDisable)
			metal3Pod := getMetal3StatePod(oc)

			defer func() {
				g.By("Restore original disableDefaultPrometheusRules")
				restorePrometheusExporterField(oc, "disableDefaultPrometheusRules", originalDisable)
				if originalDisable != "true" {
					waitForIPEPrometheusRuleState(oc, true)
					waitForMetal3DefaultAlertRules(oc)
				}
			}()

			g.By("Disable default PrometheusRules")
			_, err := patchPrometheusExporterField(oc, "disableDefaultPrometheusRules", "true")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verify PrometheusRule is removed while ServiceMonitor and metrics port stay")
			waitForIPEPrometheusRuleState(oc, false)

			g.By("Verify CMO Prometheus unloaded the metal3-defaults rule groups")
			waitForMetal3DefaultRulesUnloaded(oc)

			g.By("Verify metrics are still scraped and the metal3 pod was not restarted")
			up := waitForPrometheusQuery(oc, `up{job="metal3-state"}`, 3*time.Minute)
			upValue, ok := up.Data.Result[0].Value[1].(string)
			o.Expect(ok).To(o.BeTrue(), "up{job=\"metal3-state\"} value is not a string")
			o.Expect(upValue).To(o.Equal("1"), "expected job=metal3-state to stay up with default rules disabled")
			o.Expect(getMetal3StatePod(oc)).To(o.Equal(metal3Pod), "toggling default rules must not restart the metal3 pod")

			g.By("Re-enable default PrometheusRules")
			_, err = patchPrometheusExporterField(oc, "disableDefaultPrometheusRules", "false")
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPEPrometheusRuleState(oc, true)
			waitForMetal3DefaultAlertRules(oc)
		})

		// Runs last in this ordered container: it detaches a worker BMH for
		// about 15 minutes, and a failure skips later ordered specs.
		g.It("Author:jhajyahy-Longduration-NonPreRelease-Medium-91038-Verify BaremetalStaleMetrics fires only for the host that stopped reporting [Disruptive]", func() {
			var target string
			var redfishHosts []bmhInfo
			for _, h := range deployedBareMetalHosts(oc) {
				if !isRedfishBMH(h) {
					continue
				}
				redfishHosts = append(redfishHosts, h)
				// Only detach workers; never take a control-plane host out of Ironic.
				if target == "" && h.ProvisioningState == "provisioned" {
					target = h.Name
				}
			}
			if target == "" {
				g.Skip("No provisioned Redfish worker BareMetalHost to detach")
			}
			e2e.Logf("Target BMH for stale-metrics check: %s", target)

			g.By("Verify the target host currently reports hardware metrics")
			waitForIPESeriesForHosts(oc, redfishHosts, 5*time.Minute)
			preFiring, err := staleAlertFiringNodes(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, n := range preFiring {
				o.Expect(ipeNodeNameMatchesBMH(n, target)).To(o.BeFalse(), "BaremetalStaleMetrics already firing for %s before the test", target)
			}
			e2e.Logf("BaremetalStaleMetrics firing before detach: %v", preFiring)

			defer func() {
				g.By("Re-attach the target BMH")
				setBMHDetached(oc, target, false)
				waitForBMHOperationalStatus(oc, target, "OK")
				waitForBMHState(oc, target, "provisioned")
			}()

			g.By("Detach the target BMH so Ironic stops collecting its sensor data")
			setBMHDetached(oc, target, true)
			waitForBMHOperationalStatus(oc, target, "detached")

			g.By("Wait for BaremetalStaleMetrics to fire for the target host only")
			// 300s stale threshold + 5m 'for' + scrape/evaluation slack.
			err = wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 20*time.Minute, true, func(context.Context) (bool, error) {
				firing, qerr := staleAlertFiringNodes(oc)
				if qerr != nil {
					e2e.Logf("%v", qerr)
					return false, nil
				}
				targetFiring := false
				for _, n := range firing {
					if ipeNodeNameMatchesBMH(n, target) {
						targetFiring = true
						continue
					}
					preExisting := false
					for _, p := range preFiring {
						if p == n {
							preExisting = true
						}
					}
					if !preExisting {
						return false, fmt.Errorf("BaremetalStaleMetrics fired for %s, which is still attached", n)
					}
				}
				e2e.Logf("BaremetalStaleMetrics firing: %v (target firing=%v)", firing, targetFiring)
				return targetFiring, nil
			})
			compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("BaremetalStaleMetrics did not fire for detached BMH %s", target))

			g.By("Re-attach the target BMH and wait for the alert to resolve")
			setBMHDetached(oc, target, false)
			waitForBMHOperationalStatus(oc, target, "OK")
			err = wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
				firing, qerr := staleAlertFiringNodes(oc)
				if qerr != nil {
					e2e.Logf("%v", qerr)
					return false, nil
				}
				for _, n := range firing {
					if ipeNodeNameMatchesBMH(n, target) {
						e2e.Logf("BaremetalStaleMetrics still firing for %s", target)
						return false, nil
					}
				}
				return true, nil
			})
			compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("BaremetalStaleMetrics did not resolve for re-attached BMH %s", target))
		})
	})
})
