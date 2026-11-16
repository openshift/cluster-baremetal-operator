package baremetal

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	metal3StateLabel = "baremetal.openshift.io/cluster-baremetal-operator=metal3-state"
	vmediaContainer  = "metal3-httpd"
	vmediaPort       = "6183"
	ironicAPIPort    = "6385"

	vmediaProtocolEnv = "IRONIC_VMEDIA_SSL_PROTOCOL"
	ironicProtocolEnv = "IRONIC_SSL_PROTOCOL"
	vmediaTLS12Env    = "IRONIC_VMEDIA_TLS_12_CIPHERS"
	vmediaTLS13Env    = "IRONIC_VMEDIA_TLS_13_CIPHERS"
	ironicTLS12Env    = "IRONIC_TLS_12_CIPHERS"
	ironicTLS13Env    = "IRONIC_TLS_13_CIPHERS"
	vmediaCurvesEnv   = "IRONIC_VMEDIA_CURVES"
	ironicCurvesEnv   = "IRONIC_TLS_CURVES"
)

// virtualMediaServerTLS is the Provisioning spec exercised by METAL-1947.
// The reprovision test targets a bare-metal IPI worker whose BMC boots
// through virtual media, which is the CI hardware path.
type virtualMediaServerTLS struct {
	MinTLSVersion string   `json:"minTLSVersion,omitempty"`
	Ciphers       []string `json:"ciphers,omitempty"`
	Groups        []string `json:"groups,omitempty"`
}

type provisioningTLSDoc struct {
	Spec struct {
		DisableVirtualMediaTLS bool            `json:"disableVirtualMediaTLS"`
		VirtualMediaServerTLS  json.RawMessage `json:"virtualMediaServerTLS"`
	} `json:"spec"`
}

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc = compat_otp.NewCLI("virtual-media-server-tls", compat_otp.KubeConfigPath())
	)

	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
		if !virtualMediaTLSFieldAvailable(oc) {
			g.Skip("provisioning.spec.virtualMediaServerTLS is not in this cluster's CRD")
		}
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-High-METAL-1947-Virtual media TLS listener matches the Provisioning spec [Level0]", func() {
		doc := getProvisioningTLS(oc)
		if doc.Spec.DisableVirtualMediaTLS {
			g.Skip("virtual media TLS is disabled")
		}

		env := runningHTTPDEnv(oc)
		spec := decodeVirtualMediaTLS(doc.Spec.VirtualMediaServerTLS)
		if spec == nil {
			g.By("Unset override keeps the virtual-media listener on the cluster TLS profile")
			o.Expect(env[vmediaProtocolEnv]).To(o.Equal(env[ironicProtocolEnv]))
			o.Expect(env[vmediaTLS12Env]).To(o.Equal(env[ironicTLS12Env]))
			o.Expect(env[vmediaTLS13Env]).To(o.Equal(env[ironicTLS13Env]))
			o.Expect(env[vmediaCurvesEnv]).To(o.Equal(env[ironicCurvesEnv]))
			return
		}

		g.By("Configured override is applied only to the virtual-media listener variables")
		protocol, ok := apacheProtocolForMinTLS(spec.MinTLSVersion)
		o.Expect(ok).To(o.BeTrue(), "unsupported minTLSVersion %q", spec.MinTLSVersion)
		o.Expect(env[vmediaProtocolEnv]).To(o.Equal(protocol))
		if len(spec.Ciphers) > 0 {
			tls12, tls13 := splitTLSCiphers(spec.Ciphers)
			o.Expect(env[vmediaTLS12Env]).To(o.Equal(strings.Join(tls12, ":")))
			o.Expect(env[vmediaTLS13Env]).To(o.Equal(strings.Join(tls13, ":")))
		}
		if len(spec.Groups) > 0 {
			o.Expect(env[vmediaCurvesEnv]).To(o.Equal(strings.Join(spec.Groups, ":")))
		}
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-High-METAL-1947-Virtual media TLS override does not change Ironic API TLS [Serial][Disruptive]", func() {
		doc := getProvisioningTLS(oc)
		if doc.Spec.DisableVirtualMediaTLS {
			g.Skip("virtual media TLS is disabled")
		}

		g.By("Reject virtual-media TLS settings the API does not allow")
		expectProvisioningPatchRejected(oc, `{"spec":{"virtualMediaServerTLS":{"minTLSVersion":"TLSv1.2"}}}`)
		expectProvisioningPatchRejected(oc, `{"spec":{"virtualMediaServerTLS":{"minTLSVersion":"VersionTLS12","ciphers":["ECDHE RSA"]}}}`)
		expectProvisioningPatchRejected(oc, `{"spec":{"virtualMediaServerTLS":{"minTLSVersion":"VersionTLS12","groups":["not-a-group"]}}}`)

		baseline := runningHTTPDEnv(oc)
		bmoBefore, err := bmoContainerArgs(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		original := append(json.RawMessage(nil), doc.Spec.VirtualMediaServerTLS...)

		defer func() {
			g.By("Restore the original virtual-media TLS setting")
			restoreVirtualMediaTLS(oc, original)
			restored, waitErr := waitForHTTPDEnv(oc, func(env map[string]string) bool {
				return env[vmediaProtocolEnv] == baseline[vmediaProtocolEnv] &&
					env[vmediaTLS12Env] == baseline[vmediaTLS12Env] &&
					env[vmediaTLS13Env] == baseline[vmediaTLS13Env] &&
					env[vmediaCurvesEnv] == baseline[vmediaCurvesEnv]
			})
			if waitErr != nil {
				e2e.Logf("virtual-media env after restore: %v", restored)
			}
			o.Expect(waitErr).NotTo(o.HaveOccurred())
		}()

		override := virtualMediaServerTLS{
			MinTLSVersion: "VersionTLS12",
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "TLS_AES_128_GCM_SHA256"},
			Groups:        []string{"secp384r1"},
		}
		g.By("Apply a virtual-media TLS override")
		patch, err := json.Marshal(map[string]any{"spec": map[string]any{"virtualMediaServerTLS": override}})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = patchProvisioning(oc, string(patch), false)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait until metal3-httpd serves the override")
		env, err := waitForHTTPDEnv(oc, func(env map[string]string) bool {
			return env[vmediaProtocolEnv] == "-ALL +TLSv1.2 +TLSv1.3" &&
				env[vmediaTLS12Env] == "ECDHE-RSA-AES128-GCM-SHA256" &&
				env[vmediaTLS13Env] == "TLS_AES_128_GCM_SHA256" &&
				env[vmediaCurvesEnv] == "secp384r1"
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(env[ironicProtocolEnv]).To(o.Equal(baseline[ironicProtocolEnv]))
		o.Expect(env[ironicTLS12Env]).To(o.Equal(baseline[ironicTLS12Env]))
		o.Expect(env[ironicTLS13Env]).To(o.Equal(baseline[ironicTLS13Env]))
		o.Expect(env[ironicCurvesEnv]).To(o.Equal(baseline[ironicCurvesEnv]))

		bmoAfter, err := bmoContainerArgs(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmoAfter).To(o.Equal(bmoBefore))
		o.Expect(bmoAfter).NotTo(o.ContainSubstring("VMEDIA"))

		g.By("Virtual-media port accepts TLS 1.2 with the configured cipher")
		pod := runningMetal3Pod(oc)
		vmediaOut, vmediaErr := tlsHandshake(oc, pod, vmediaPort, "-tls1_2")
		o.Expect(handshakeAccepted(vmediaOut, vmediaErr)).To(o.BeTrue(), "virtual-media TLS 1.2 handshake failed: %s", handshakeText(vmediaOut, vmediaErr))

		if baseline[ironicProtocolEnv] == "-ALL +TLSv1.3" {
			g.By("Ironic API port still rejects TLS 1.2")
			apiOut, apiErr := tlsHandshake(oc, pod, ironicAPIPort, "-tls1_2")
			o.Expect(handshakeAccepted(apiOut, apiErr)).To(o.BeFalse(), "Ironic API unexpectedly accepted TLS 1.2: %s", handshakeText(apiOut, apiErr))
		}

		g.By("Reject disabling virtual media TLS while the override is set")
		out, err := patchProvisioning(oc, `{"spec":{"disableVirtualMediaTLS":true}}`, true)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("virtualMediaServerTLS cannot be set when disableVirtualMediaTLS is true"))
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Longduration-NonPreRelease-High-METAL-1947-Worker reprovision uses the virtual-media TLS listener [Serial][Disruptive]", func() {
		doc := getProvisioningTLS(oc)
		if doc.Spec.DisableVirtualMediaTLS {
			g.Skip("virtual media TLS is disabled")
		}
		bmhName, machineName, machineSet := virtualMediaMachinesetWorker(oc)
		if bmhName == "" {
			g.Skip("no virtual-media worker BareMetalHost is owned by a MachineSet")
		}

		originReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		originReplicas, err := strconv.Atoi(originReplicasStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		if originReplicas < 1 {
			g.Skip("worker MachineSet has no replicas to reprovision")
		}

		baseline := runningHTTPDEnv(oc)
		original := append(json.RawMessage(nil), doc.Spec.VirtualMediaServerTLS...)
		defer func() {
			g.By("Restore the original virtual-media TLS setting")
			restoreVirtualMediaTLS(oc, original)
			_, waitErr := waitForHTTPDEnv(oc, func(env map[string]string) bool {
				return env[vmediaProtocolEnv] == baseline[vmediaProtocolEnv]
			})
			o.Expect(waitErr).NotTo(o.HaveOccurred())
		}()

		g.By("Apply a TLS 1.2 minimum for the virtual-media listener")
		patch, err := json.Marshal(map[string]any{
			"spec": map[string]any{"virtualMediaServerTLS": virtualMediaServerTLS{MinTLSVersion: "VersionTLS12"}},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = patchProvisioning(oc, string(patch), false)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = waitForHTTPDEnv(oc, func(env map[string]string) bool {
			return env[vmediaProtocolEnv] == "-ALL +TLSv1.2 +TLSv1.3"
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		scaledDown := false
		defer func() {
			if !scaledDown {
				_, _ = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machines.machine.openshift.io", machineName, "machine.openshift.io/cluster-api-delete-machine-", "-n", machineAPINamespace).Output()
				return
			}
			g.By("Restore the worker MachineSet replica count")
			currentReplicasStr, replicaErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
			o.Expect(replicaErr).NotTo(o.HaveOccurred())
			if currentReplicasStr != originReplicasStr {
				_, scaleErr := oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
				o.Expect(scaleErr).NotTo(o.HaveOccurred())
			}
			waitForBMHState(oc, bmhName, "provisioned")
			nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "worker did not return Ready after virtual-media reprovision")
			operatorErr := clusterOperatorHealthcheck(oc, 1500, "METAL-1947-reprovision.log")
			compat_otp.AssertWaitPollNoErr(operatorErr, "cluster operators did not recover after virtual-media reprovision")
		}()

		g.By("Deprovision one virtual-media worker")
		_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machines.machine.openshift.io", machineName, "machine.openshift.io/cluster-api-delete-machine=yes", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%d", originReplicas-1)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		scaledDown = true
		waitForBMHState(oc, bmhName, "available")

		g.By("Provision the worker again through the virtual-media TLS listener")
		since := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, bmhName, "provisioned")

		ironicLogs, ironicErr := metal3LogsSince(oc, "metal3-ironic", since)
		httpdLogs, httpdErr := metal3LogsSince(oc, vmediaContainer, since)
		if !strings.Contains(ironicLogs+"\n"+httpdLogs, ":"+vmediaPort) {
			o.Expect(ironicErr).NotTo(o.HaveOccurred())
			o.Expect(httpdErr).NotTo(o.HaveOccurred())
			e2e.Failf("provisioning logs since %s did not mention the virtual-media HTTPS port %s", since, vmediaPort)
		}
	})
})

func virtualMediaTLSFieldAvailable(oc *exutil.CLI) bool {
	_, err := oc.AsAdmin().WithoutNamespace().Run("explain").Args("provisioning.spec.virtualMediaServerTLS", "--api-version=metal3.io/v1alpha1").Output()
	return err == nil
}

func getProvisioningTLS(oc *exutil.CLI) provisioningTLSDoc {
	raw, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("provisioning", "provisioning-configuration", "-o=json").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	var doc provisioningTLSDoc
	o.Expect(json.Unmarshal([]byte(raw), &doc)).NotTo(o.HaveOccurred())
	return doc
}

func decodeVirtualMediaTLS(raw json.RawMessage) *virtualMediaServerTLS {
	if virtualMediaTLSUnset(raw) {
		return nil
	}
	var spec virtualMediaServerTLS
	o.Expect(json.Unmarshal(raw, &spec)).NotTo(o.HaveOccurred())
	return &spec
}

func virtualMediaTLSUnset(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null"
}

func patchProvisioning(oc *exutil.CLI, patch string, dryRun bool) (string, error) {
	args := []string{"provisioning", "provisioning-configuration", "--type=merge", "-p", patch}
	if dryRun {
		args = append(args, "--dry-run=server")
	}
	return oc.AsAdmin().WithoutNamespace().Run("patch").Args(args...).Output()
}

func expectProvisioningPatchRejected(oc *exutil.CLI, patch string) {
	_, err := patchProvisioning(oc, patch, true)
	o.Expect(err).To(o.HaveOccurred(), "expected the API to reject %s", patch)
}

func restoreVirtualMediaTLS(oc *exutil.CLI, original json.RawMessage) {
	if virtualMediaTLSUnset(original) {
		_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"provisioning", "provisioning-configuration", "--type=json",
			"-p", `[{"op":"remove","path":"/spec/virtualMediaServerTLS"}]`,
		).Output()
		if err != nil {
			e2e.Logf("removing virtualMediaServerTLS: %v", err)
		}
		return
	}
	patch, err := json.Marshal(map[string]any{
		"spec": map[string]json.RawMessage{"virtualMediaServerTLS": original},
	})
	if err != nil {
		e2e.Logf("encoding original virtualMediaServerTLS: %v", err)
		return
	}
	if _, err := patchProvisioning(oc, string(patch), false); err != nil {
		e2e.Logf("restoring virtualMediaServerTLS: %v", err)
	}
}

func runningHTTPDEnv(oc *exutil.CLI) map[string]string {
	pod := runningMetal3Pod(oc)
	env, err := httpdEnv(oc, pod)
	o.Expect(err).NotTo(o.HaveOccurred())
	return env
}

func runningMetal3Pod(oc *exutil.CLI) string {
	names, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"pods", "-n", machineAPINamespace, "-l", metal3StateLabel,
		"-o=jsonpath={.items[*].metadata.name}",
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, name := range strings.Fields(names) {
		if podPhase(oc, name) == "Running" {
			return name
		}
	}
	e2e.Failf("no Running metal3 pod in %s", machineAPINamespace)
	return ""
}

func waitForHTTPDEnv(oc *exutil.CLI, ready func(map[string]string) bool) (map[string]string, error) {
	var found map[string]string
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 6*time.Minute, true, func(context.Context) (bool, error) {
		names, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"pods", "-n", machineAPINamespace, "-l", metal3StateLabel,
			"-o=jsonpath={.items[*].metadata.name}",
		).Output()
		if err != nil {
			e2e.Logf("listing metal3 pods: %v", err)
			return false, nil
		}
		for _, name := range strings.Fields(names) {
			if podPhase(oc, name) != "Running" {
				continue
			}
			env, err := httpdEnv(oc, name)
			if err != nil {
				e2e.Logf("reading %s env: %v", name, err)
				continue
			}
			if ready(env) {
				found = env
				e2e.Logf("metal3-httpd %s has the expected virtual-media TLS env", name)
				return true, nil
			}
		}
		return false, nil
	})
	return found, err
}

func httpdEnv(oc *exutil.CLI, pod string) (map[string]string, error) {
	out, err := execHTTPD(oc, pod, "/usr/bin/env")
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && key != "" {
			env[key] = value
		}
	}
	return env, nil
}

func tlsHandshake(oc *exutil.CLI, pod, port, tlsFlag string) (string, error) {
	script := fmt.Sprintf("openssl s_client -connect 127.0.0.1:%s %s </dev/null", port, tlsFlag)
	return execHTTPD(oc, pod, "/bin/sh", "-c", script)
}

func handshakeText(out string, err error) string {
	if err == nil {
		return out
	}
	return out + "\n" + err.Error()
}

// handshakeAccepted reports a completed TLS 1.2 handshake that selected the
// virtual-media cipher. Certificate verification can make s_client exit
// non-zero after a successful handshake, so the cipher line is the signal.
func handshakeAccepted(out string, err error) bool {
	text := handshakeText(out, err)
	if strings.Contains(text, "handshake failure") ||
		strings.Contains(text, "no protocols available") ||
		strings.Contains(text, "alert protocol version") ||
		strings.Contains(text, "Cipher is (NONE)") ||
		strings.Contains(text, "Cipher    : (NONE)") {
		return false
	}
	return strings.Contains(text, "ECDHE-RSA-AES128-GCM-SHA256")
}

func execHTTPD(oc *exutil.CLI, pod string, command ...string) (string, error) {
	args := append([]string{"-n", machineAPINamespace, pod, "-c", vmediaContainer, "--"}, command...)
	return oc.AsAdmin().WithoutNamespace().Run("exec").Args(args...).Output()
}

func podPhase(oc *exutil.CLI, pod string) string {
	phase, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", machineAPINamespace, pod, "-o=jsonpath={.status.phase}").Output()
	if err != nil {
		return ""
	}
	return phase
}

func bmoContainerArgs(oc *exutil.CLI) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"deploy", "metal3-baremetal-operator", "-n", machineAPINamespace,
		"-o=jsonpath={.spec.template.spec.containers[0].args}",
	).Output()
}

func virtualMediaMachinesetWorker(oc *exutil.CLI) (bmhName, machineName, machineSet string) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"machines.machine.openshift.io", "-n", machineAPINamespace,
		"-l", "machine.openshift.io/cluster-api-machine-role=worker",
		`-o=jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`,
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	bmhOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"bmh", "-n", machineAPINamespace,
		`-o=jsonpath={range .items[*]}{.metadata.name},{.spec.consumerRef.name}{"\n"}{end}`,
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	machineToBMH := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(bmhOutput), "\n") {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			machineToBMH[parts[1]] = parts[0]
		}
	}

	for _, machine := range strings.Split(strings.TrimSpace(output), "\n") {
		if machine == "" {
			continue
		}
		host := machineToBMH[machine]
		if host == "" {
			continue
		}
		method, bootErr := getBootMethod(oc, host)
		if bootErr != nil || method != bootMethodVMedia {
			continue
		}
		owner, ownerErr := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"machines.machine.openshift.io", machine, "-n", machineAPINamespace,
			"-o=jsonpath={.metadata.ownerReferences[?(@.kind==\"MachineSet\")].name}",
		).Output()
		if ownerErr != nil || owner == "" {
			continue
		}
		return host, machine, owner
	}
	return "", "", ""
}

func metal3LogsSince(oc *exutil.CLI, container, since string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("logs").Args(
		"-n", machineAPINamespace, "deploy/metal3", "-c", container, "--since-time="+since,
	).Output()
}

func apacheProtocolForMinTLS(min string) (string, bool) {
	switch min {
	case "VersionTLS10":
		return "-ALL +TLSv1 +TLSv1.1 +TLSv1.2 +TLSv1.3", true
	case "VersionTLS11":
		return "-ALL +TLSv1.1 +TLSv1.2 +TLSv1.3", true
	case "VersionTLS12":
		return "-ALL +TLSv1.2 +TLSv1.3", true
	case "VersionTLS13":
		return "-ALL +TLSv1.3", true
	default:
		return "", false
	}
}

func splitTLSCiphers(ciphers []string) (tls12, tls13 []string) {
	for _, cipher := range ciphers {
		if strings.HasPrefix(cipher, "TLS_") {
			tls13 = append(tls13, cipher)
		} else {
			tls12 = append(tls12, cipher)
		}
	}
	return tls12, tls13
}
