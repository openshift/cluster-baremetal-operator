package baremetal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
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
	patch, err := json.Marshal([]map[string]any{
		{"op": "add", "path": "/spec/virtualMediaServerTLS", "value": original},
	})
	if err != nil {
		e2e.Logf("encoding original virtualMediaServerTLS: %v", err)
		return
	}
	if _, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
		"provisioning", "provisioning-configuration", "--type=json", "-p", string(patch),
	).Output(); err != nil {
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

func tlsHandshake(ctx context.Context, oc *exutil.CLI, pod, port, tlsFlag string) (string, error) {
	// timeout stops a handshake that accepts TCP and then stalls. The context
	// stops the local oc process when the exec stream never returns.
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	script := fmt.Sprintf("timeout 10 openssl s_client -connect 127.0.0.1:%s %s </dev/null", port, tlsFlag)
	return execHTTPDWithin(ctx, oc, pod, "/bin/sh", "-c", script)
}

func execHTTPDWithin(ctx context.Context, oc *exutil.CLI, pod string, command ...string) (string, error) {
	args := append([]string{"-n", machineAPINamespace, pod, "-c", vmediaContainer, "--"}, command...)
	cmd, stdout, stderr, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(args...).Background()
	if err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err = <-done:
	case <-ctx.Done():
		if cmd.Process != nil {
			if killErr := cmd.Process.Kill(); killErr != nil {
				e2e.Logf("stopping oc exec: %v", killErr)
			}
		}
		waitErr := <-done
		if waitErr != nil {
			err = fmt.Errorf("oc exec stopped after %w: %v", ctx.Err(), waitErr)
		} else {
			err = fmt.Errorf("oc exec stopped after %w", ctx.Err())
		}
	}
	out := strings.TrimSpace(stdout.String())
	if stderrText := strings.TrimSpace(stderr.String()); stderrText != "" {
		if out != "" {
			out += "\n"
		}
		out += stderrText
	}
	return out, err
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
	if tlsNegotiationFailed(text) || handshakeSetupFailed(text) {
		return false
	}
	return strings.Contains(text, "ECDHE-RSA-AES128-GCM-SHA256")
}

// handshakeRejected reports an explicit TLS negotiation failure. A missing
// cipher, a failed exec, or a refused connection is not a rejection.
func handshakeRejected(out string, err error) bool {
	text := handshakeText(out, err)
	if handshakeSetupFailed(text) || strings.Contains(text, "ECDHE-RSA-AES128-GCM-SHA256") {
		return false
	}
	return tlsNegotiationFailed(text)
}

func handshakeNegotiatedTLS13(out string, err error) bool {
	text := handshakeText(out, err)
	if tlsNegotiationFailed(text) || handshakeSetupFailed(text) {
		return false
	}
	return strings.Contains(text, "TLSv1.3")
}

func tlsNegotiationFailed(text string) bool {
	return strings.Contains(text, "handshake failure") ||
		strings.Contains(text, "no protocols available") ||
		strings.Contains(text, "alert protocol version") ||
		strings.Contains(text, "Cipher is (NONE)") ||
		strings.Contains(text, "Cipher    : (NONE)")
}

func handshakeSetupFailed(text string) bool {
	return strings.Contains(text, "executable file not found") ||
		strings.Contains(text, "openssl: not found") ||
		strings.Contains(text, "unable to upgrade connection") ||
		strings.Contains(text, "cannot exec") ||
		strings.Contains(text, "Connection refused") ||
		strings.Contains(text, "exit code 124") ||
		strings.Contains(text, "exit status 124") ||
		strings.Contains(text, "signal: killed") ||
		strings.Contains(text, "oc exec exceeded") ||
		strings.Contains(text, "oc exec stopped") ||
		strings.Contains(text, "context deadline exceeded") ||
		strings.Contains(text, "context canceled")
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
