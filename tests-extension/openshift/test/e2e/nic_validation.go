package baremetal

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// NICInfo represents network interface card information
type NICInfo struct {
	Name       string `json:"name"`
	MAC        string `json:"mac"`
	PCIAddress string `json:"pciAddress"`
}

// NodeNICInfo represents the actual NIC info from a node
type NodeNICInfo struct {
	InterfaceName string
	MAC           string
	ParentDev     string // PCI address
}

// maskMAC masks a MAC address for logging (e.g., aa:bb:cc:dd:ee:ff -> aa:bb:**:**:**:ff)
func maskMAC(mac string) string {
	if mac == "" {
		return "N/A"
	}
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return "***masked***"
	}
	return fmt.Sprintf("%s:%s:**:**:**:%s", parts[0], parts[1], parts[5])
}

// isVLANInterface checks if a NIC name represents a VLAN sub-interface (e.g., ens1f0.1000)
// BMH hardware inspection discovers these from firmware/LLDP but they may not exist as
// OS-level interfaces on the node.
func isVLANInterface(name string) bool {
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		return false
	}
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(parts[1]) > 0
}

// deduplicateNICs removes duplicate NIC entries based on name
func deduplicateNICs(nics []NICInfo) []NICInfo {
	seen := make(map[string]bool)
	var uniqueNICs []NICInfo

	for _, nic := range nics {
		// Use name as the unique key
		if !seen[nic.Name] {
			seen[nic.Name] = true
			uniqueNICs = append(uniqueNICs, nic)
		} else {
			e2e.Logf("Removing duplicate NIC entry: %s (PCI: %s)", nic.Name, nic.PCIAddress)
		}
	}

	return uniqueNICs
}

// getNICsFromBMH extracts NIC information from BareMetalHost status
func getNICsFromBMH(oc *exutil.CLI, bmhName string) ([]NICInfo, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"bmh", bmhName, "-n", machineAPINamespace,
		"-o=jsonpath={.status.hardware.nics}").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get NICs from BMH %s: %v", bmhName, err)
	}

	var nics []NICInfo
	if err := json.Unmarshal([]byte(output), &nics); err != nil {
		return nil, fmt.Errorf("failed to parse NICs JSON from BMH %s: %v", bmhName, err)
	}

	// Remove duplicates
	nics = deduplicateNICs(nics)

	return nics, nil
}

// getNICsFromHardwareData extracts NIC information from HardwareData
func getNICsFromHardwareData(oc *exutil.CLI, bmhName string) ([]NICInfo, error) {
	// HardwareData resource has the same name as the BMH
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"hardwaredata", bmhName, "-n", machineAPINamespace,
		"-o=jsonpath={.spec.hardware.nics}").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get NICs from HardwareData %s: %v", bmhName, err)
	}

	var nics []NICInfo
	if err := json.Unmarshal([]byte(output), &nics); err != nil {
		return nil, fmt.Errorf("failed to parse NICs JSON from HardwareData %s: %v", bmhName, err)
	}

	// Remove duplicates
	nics = deduplicateNICs(nics)

	return nics, nil
}

// ConsumerRef represents the consumer reference in BMH spec
type ConsumerRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

var errBMHNotProvisioned = errors.New("BMH is not provisioned to a node")

// getNodeNameFromBMH gets the node name associated with a BMH.
// Returns errBMHNotProvisioned when the BMH has no consumerRef or the
// Machine has no nodeRef yet; all other errors indicate API or data failures.
func getNodeNameFromBMH(oc *exutil.CLI, bmhName string) (string, error) {
	consumerRefJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"bmh", bmhName, "-n", machineAPINamespace,
		"-o=jsonpath={.spec.consumerRef}").Output()
	if err != nil {
		return "", fmt.Errorf("failed to query consumerRef for BMH %s: %w", bmhName, err)
	}
	if consumerRefJSON == "" {
		return "", fmt.Errorf("BMH %s has no consumerRef: %w", bmhName, errBMHNotProvisioned)
	}

	var consumerRef ConsumerRef
	if err := json.Unmarshal([]byte(consumerRefJSON), &consumerRef); err != nil {
		return "", fmt.Errorf("failed to parse consumerRef for BMH %s: %w", bmhName, err)
	}

	if consumerRef.Name == "" {
		return "", fmt.Errorf("BMH %s consumerRef has empty name: %w", bmhName, errBMHNotProvisioned)
	}

	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"machine", consumerRef.Name, "-n", machineAPINamespace,
		"-o=jsonpath={.status.nodeRef.name}").Output()
	if err != nil {
		return "", fmt.Errorf("failed to query Machine %s for BMH %s: %w", consumerRef.Name, bmhName, err)
	}
	if nodeName == "" {
		return "", fmt.Errorf("Machine %s has no nodeRef: %w", consumerRef.Name, errBMHNotProvisioned)
	}

	return nodeName, nil
}

// extractJSON extracts JSON content from oc debug node output that includes non-JSON status lines
func extractJSON(output string) string {
	// Find the first '[' which starts the JSON array
	startIdx := strings.Index(output, "[")
	if startIdx == -1 {
		return output // No JSON array found, return as-is
	}

	// Find the last ']' which ends the JSON array
	endIdx := strings.LastIndex(output, "]")
	if endIdx == -1 || endIdx < startIdx {
		return output // Invalid JSON structure, return as-is
	}

	// Extract just the JSON part
	return output[startIdx : endIdx+1]
}

var macRegex = regexp.MustCompile(`(?i)\b([0-9a-f]{2}:[0-9a-f]{2}):[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:([0-9a-f]{2})\b`)

// maskMACsInOutput masks MAC addresses in command output for safe logging
func maskMACsInOutput(output string) string {
	return macRegex.ReplaceAllString(output, "$1:**:**:**:$2")
}

// getActualNICsFromNode gets the actual NIC information from a node using oc debug
func getActualNICsFromNode(oc *exutil.CLI, nodeName string, expectedNICs []NICInfo) ([]NodeNICInfo, error) {
	var actualNICs []NodeNICInfo
	var collectedErrors []string

	// Get all interfaces at once (optimization: single debug pod invocation)
	output, err := compat_otp.DebugNodeWithChroot(oc, nodeName, "ip", "-j", "-d", "link", "show")
	if err != nil {
		return nil, fmt.Errorf("failed to get interface info from node %s: %v", nodeName, err)
	}

	// Mask MAC addresses before logging raw output
	maskedOutput := maskMACsInOutput(output)
	e2e.Logf("Raw output from node %s (MACs masked):\n%s", nodeName, maskedOutput)

	// Extract just the JSON part from the output (removing debug pod messages)
	jsonOutput := extractJSON(output)
	maskedJSON := maskMACsInOutput(jsonOutput)
	e2e.Logf("Extracted JSON (MACs masked):\n%s", maskedJSON)

	// Parse JSON output - the output is an array of all interfaces
	var allNICData []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &allNICData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output from node %s: %v", nodeName, err)
	}

	// Match each expected NIC by name
	for _, expectedNIC := range expectedNICs {
		found := false
		for _, nicData := range allNICData {
			ifname, ok := nicData["ifname"].(string)
			if !ok {
				collectedErrors = append(collectedErrors, fmt.Sprintf("interface missing 'ifname' field"))
				continue
			}

			// Match by interface name
			if ifname != expectedNIC.Name {
				continue
			}

			found = true

			// Extract MAC address
			address, ok := nicData["address"].(string)
			if !ok {
				collectedErrors = append(collectedErrors, fmt.Sprintf("NIC %s missing 'address' field", ifname))
				continue
			}

			// parentdev might not always be present
			parentdev := "N/A"
			if pd, ok := nicData["parentdev"].(string); ok {
				parentdev = pd
			}

			actualNIC := NodeNICInfo{
				InterfaceName: ifname,
				MAC:           address,
				ParentDev:     parentdev,
			}
			actualNICs = append(actualNICs, actualNIC)
			e2e.Logf("Found NIC on node %s: %s (MAC: %s, PCI: %s)", nodeName, ifname, maskMAC(address), parentdev)
			break
		}

		if !found {
			collectedErrors = append(collectedErrors, fmt.Sprintf("NIC %s not found on node %s", expectedNIC.Name, nodeName))
		}
	}

	// Return aggregated errors if any occurred
	if len(collectedErrors) > 0 {
		return actualNICs, fmt.Errorf("errors collecting NIC data: %s", strings.Join(collectedErrors, "; "))
	}

	return actualNICs, nil
}

// findActualNICByName finds a NIC in the actual NICs list by interface name
func findActualNICByName(actualNICs []NodeNICInfo, name string) (NodeNICInfo, bool) {
	for _, nic := range actualNICs {
		if nic.InterfaceName == name {
			return nic, true
		}
	}
	return NodeNICInfo{}, false
}

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc           = compat_otp.NewCLI("nic-validation", compat_otp.KubeConfigPath())
	)

	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
	})

	// author: sgoveas@redhat.com
	g.It("Author:sgoveas-Medium-88285-Verify NIC information in BMH, HardwareData and actual nodes match [Level0]", func() {
		g.By("Get all BareMetalHost resources")
		bmhNamesOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get BMH names")
		bmhNames := strings.Fields(bmhNamesOutput)
		o.Expect(bmhNames).NotTo(o.BeEmpty(), "No BareMetalHost resources found")
		e2e.Logf("Found %d BareMetalHost resources: %v", len(bmhNames), bmhNames)

		g.By("Verify NIC information for each BareMetalHost")
		for _, bmhName := range bmhNames {
			e2e.Logf("\n========== Validating BMH: %s ==========", bmhName)

			// Get NICs from BMH status
			bmhNICs, err := getNICsFromBMH(oc, bmhName)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to get NICs from BMH %s", bmhName))
			o.Expect(bmhNICs).NotTo(o.BeEmpty(), fmt.Sprintf("No NICs found in BMH %s", bmhName))
			e2e.Logf("BMH %s has %d NICs", bmhName, len(bmhNICs))

			// Get NICs from HardwareData
			hdNICs, err := getNICsFromHardwareData(oc, bmhName)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to get NICs from HardwareData for %s", bmhName))
			o.Expect(hdNICs).NotTo(o.BeEmpty(), fmt.Sprintf("No NICs found in HardwareData for %s", bmhName))
			e2e.Logf("HardwareData for %s has %d NICs", bmhName, len(hdNICs))

			// Compare BMH and HardwareData NICs
			g.By(fmt.Sprintf("Comparing NIC data between BMH and HardwareData for %s", bmhName))
			o.Expect(len(bmhNICs)).To(o.Equal(len(hdNICs)),
				fmt.Sprintf("Number of NICs mismatch for %s: BMH has %d, HardwareData has %d",
					bmhName, len(bmhNICs), len(hdNICs)))

			// Create a map of HardwareData NICs by name for efficient lookup
			hdNICMap := make(map[string]NICInfo, len(hdNICs))
			for _, nic := range hdNICs {
				hdNICMap[nic.Name] = nic
			}

			// Compare each BMH NIC with corresponding HardwareData NIC by name
			for _, bmhNIC := range bmhNICs {
				hdNIC, found := hdNICMap[bmhNIC.Name]
				o.Expect(found).To(o.BeTrue(),
					fmt.Sprintf("NIC %s from BMH not found in HardwareData for %s", bmhNIC.Name, bmhName))

				e2e.Logf("Comparing NIC %s: BMH[%s/%s] vs HD[%s/%s]",
					bmhNIC.Name, maskMAC(bmhNIC.MAC), bmhNIC.PCIAddress,
					maskMAC(hdNIC.MAC), hdNIC.PCIAddress)

				o.Expect(strings.ToLower(bmhNIC.MAC)).To(o.Equal(strings.ToLower(hdNIC.MAC)),
					fmt.Sprintf("NIC %s MAC mismatch for %s", bmhNIC.Name, bmhName))
				o.Expect(bmhNIC.PCIAddress).To(o.Equal(hdNIC.PCIAddress),
					fmt.Sprintf("NIC %s PCI address mismatch for %s", bmhNIC.Name, bmhName))
			}

			// Get the node name corresponding to this BMH
			nodeName, err := getNodeNameFromBMH(oc, bmhName)
			if err != nil {
				if errors.Is(err, errBMHNotProvisioned) {
					e2e.Logf("Skipping node validation for BMH %s: %v", bmhName, err)
					continue
				}
				o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to resolve node for BMH %s", bmhName))
			}
			e2e.Logf("BMH %s corresponds to node %s", bmhName, nodeName)

			// Filter out VLAN sub-interfaces before node comparison
			var physicalNICs []NICInfo
			for _, nic := range bmhNICs {
				if isVLANInterface(nic.Name) {
					e2e.Logf("Skipping VLAN sub-interface %s for node validation", nic.Name)
					continue
				}
				physicalNICs = append(physicalNICs, nic)
			}

			// Get actual NIC info from the node
			g.By(fmt.Sprintf("Verifying actual NIC information on node %s", nodeName))
			actualNICs, err := getActualNICsFromNode(oc, nodeName, physicalNICs)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to get actual NICs from node %s: %v", nodeName, err))
			o.Expect(actualNICs).NotTo(o.BeEmpty(), fmt.Sprintf("No actual NICs found on node %s", nodeName))

			// Compare with actual node data
			for _, bmhNIC := range physicalNICs {
				actualNIC, found := findActualNICByName(actualNICs, bmhNIC.Name)
				o.Expect(found).To(o.BeTrue(),
					fmt.Sprintf("NIC %s from BMH not found on node %s", bmhNIC.Name, nodeName))

				e2e.Logf("Verifying NIC %s: MAC=%s, PCI=%s", actualNIC.InterfaceName, maskMAC(actualNIC.MAC), actualNIC.ParentDev)

				// Compare MAC addresses (case-insensitive)
				o.Expect(strings.ToLower(actualNIC.MAC)).To(o.Equal(strings.ToLower(bmhNIC.MAC)),
					fmt.Sprintf("MAC address mismatch for NIC %s on node %s", bmhNIC.Name, nodeName))

				// Compare PCI addresses
				o.Expect(actualNIC.ParentDev).To(o.Equal(bmhNIC.PCIAddress),
					fmt.Sprintf("PCI address mismatch for NIC %s on node %s", bmhNIC.Name, nodeName))

				e2e.Logf("✓ NIC %s verified successfully", bmhNIC.Name)
			}

			e2e.Logf("✓ All NIC validations passed for BMH %s\n", bmhName)
		}
	})
})
