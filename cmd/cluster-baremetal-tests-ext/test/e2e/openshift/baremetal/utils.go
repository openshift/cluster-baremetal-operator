package baremetal

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	machineAPINamespace              = "openshift-machine-api"
	maxCpuUsageAllowed       float64 = 90.0
	minRequiredMemoryInBytes         = 1000000000
	clusterProfileDir                = "CLUSTER_PROFILE_DIR"
	proxyFile                        = "proxy"
)

type Response struct {
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

func checkOperatorsRunning(oc *exutil.CLI) (bool, error) {
	jpath := `{range .items[*]}{.metadata.name}:{.status.conditions[?(@.type=='Available')].status}{':'}{.status.conditions[?(@.type=='Degraded')].status}{'\n'}{end}`
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusteroperators.config.openshift.io", "-o", "jsonpath="+jpath).Output()
	if err != nil {
		return false, fmt.Errorf("failed to execute 'oc get clusteroperators.config.openshift.io' command: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		e2e.Logf("%s", line)
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			return false, fmt.Errorf("unexpected clusteroperator status format: %q", line)
		}
		available := parts[1] == "True"
		degraded := parts[2] == "False"

		if !available || !degraded {
			return false, nil
		}
	}

	return true, nil
}

func checkNodesRunning(oc *exutil.CLI) (bool, error) {
	nodeNames, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
	if nodeErr != nil {
		return false, fmt.Errorf("failed to execute 'oc get nodes' command: %v", nodeErr)
	}
	nodes := strings.Fields(nodeNames)
	e2e.Logf("\nNode Names are %v", nodeNames)
	for _, node := range nodes {
		nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
		if statusErr != nil {
			return false, fmt.Errorf("failed to execute 'oc get nodes' command: %v", statusErr)
		}
		e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

		if nodeStatus != "True" {
			return false, nil
		}
	}
	return true, nil
}

func waitForDeployStatus(oc *exutil.CLI, depName string, nameSpace string, depStatus string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, true, func(context.Context) (bool, error) {
		statusOp, err := oc.AsAdmin().Run("get").Args("-n", nameSpace, "deployment", depName, "-o=jsonpath={.status.conditions[?(@.type=='Available')].status}").Output()
		if err != nil {
			return false, err
		}

		if strings.Contains(statusOp, depStatus) {
			e2e.Logf("Deployment %v state is %v", depName, depStatus)
			return true, nil
		}
		e2e.Logf("deployment %v is state %v, Trying again", depName, statusOp)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "The test deployment job is not running")
}

func getPodName(oc *exutil.CLI, ns string) string {
	podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nPod Name is %v", podName)
	return podName
}

func getPodStatus(oc *exutil.CLI, namespace string, podName string) string {
	podStatus, err := oc.AsAdmin().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s status is %q", podName, podStatus)
	return podStatus
}

func getNodeCpuUsage(oc *exutil.CLI, node string, sampling_time int) float64 {
	samplingTime := strconv.Itoa(sampling_time)

	cpu_sampling := "node_cpu_seconds_total%20%7Binstance%3D%27" + node
	cpu_sampling += "%27%2C%20mode%3D%27idle%27%7D%5B5" + samplingTime + "m%5D"
	query := "query=100%20-%20(avg%20by%20(instance)(irate(" + cpu_sampling + "))%20*%20100)"
	url := "http://localhost:9090/api/v1/query?" + query

	jsonString, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "--", "curl", "-s", url).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	var response Response
	unmarshalErr := json.Unmarshal([]byte(jsonString), &response)
	o.Expect(unmarshalErr).NotTo(o.HaveOccurred())
	o.Expect(len(response.Data.Result)).To(o.BeNumerically(">", 0), "Prometheus returned no results for CPU query on node %s", node)
	o.Expect(len(response.Data.Result[0].Value)).To(o.BeNumerically(">", 1), "Prometheus result missing value for CPU query on node %s", node)
	cpuUsage := response.Data.Result[0].Value[1].(string)
	cpu_usage, err := strconv.ParseFloat(cpuUsage, 64)
	o.Expect(err).NotTo(o.HaveOccurred())
	return cpu_usage
}

func getClusterUptime(oc *exutil.CLI) (int, error) {
	layout := "2006-01-02T15:04:05Z"
	completionTime, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[*].status.history[*].completionTime}").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to query clusterversion completionTime: %v", err)
	}
	
	returnTime, perr := time.Parse(layout, completionTime)
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
	query := "query=node_memory_MemAvailable_bytes%7Binstance%3D%27" + node + "%27%7D"
	url := "http://localhost:9090/api/v1/query?" + query

	jsonString, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "--", "curl", "-s", url).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	var response Response
	unmarshalErr := json.Unmarshal([]byte(jsonString), &response)
	o.Expect(unmarshalErr).NotTo(o.HaveOccurred())
	o.Expect(len(response.Data.Result)).To(o.BeNumerically(">", 0), "Prometheus returned no results for memory query on node %s", node)
	o.Expect(len(response.Data.Result[0].Value)).To(o.BeNumerically(">", 1), "Prometheus result missing value for memory query on node %s", node)
	memUsage := response.Data.Result[0].Value[1].(string)
	memFloat, err := strconv.ParseFloat(memUsage, 64)
	o.Expect(err).NotTo(o.HaveOccurred())
	availableMem := int(memFloat)
	return availableMem
}

// make sure operator is not processing and degraded
func checkOperator(oc *exutil.CLI, operatorName string) (bool, error) {
	output, err := oc.AsAdmin().Run("get").Args("clusteroperator", operatorName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if matched, _ := regexp.MatchString("True.*False.*False", output); !matched {
		e2e.Logf("clusteroperator %s is abnormal\n", operatorName)
		return false, nil
	}
	return true, nil
}

func waitForPodNotFound(oc *exutil.CLI, podName string, nameSpace string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, true, func(context.Context) (bool, error) {
		out, err := oc.AsAdmin().Run("get").Args("-n", nameSpace, "pods", "-o=jsonpath={.items[*].metadata.name}").Output()
		if err != nil {
			return false, err
		}
		if !strings.Contains(out, podName) {
			e2e.Logf("Pod %v still exists is", podName)
			return true, nil
		}
		e2e.Logf("Pod %v exists, Trying again", podName)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "The test deployment job is running")
}

func getUserFromSecret(oc *exutil.CLI, namespace string, secretName string) string {
	userbase64, pwderr := oc.AsAdmin().Run("get").Args("secrets", "-n", namespace, secretName, "-o=jsonpath={.data.username}").Output()
	o.Expect(pwderr).ShouldNot(o.HaveOccurred())
	user, err := base64.StdEncoding.DecodeString(userbase64)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	return string(user)
}

func getPassFromSecret(oc *exutil.CLI, namespace string, secretName string) string {
	pwdbase64, pwderr := oc.AsAdmin().Run("get").Args("secrets", "-n", namespace, secretName, "-o=jsonpath={.data.password}").Output()
	o.Expect(pwderr).ShouldNot(o.HaveOccurred())
	pwd, err := base64.StdEncoding.DecodeString(pwdbase64)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	return string(pwd)
}

func CopyToFile(fromPath string, toFilename string) string {
	// check if source file is regular file
	srcFileStat, err := os.Stat(fromPath)
	if err != nil {
		e2e.Failf("get source file %s stat failed: %v", fromPath, err)
	}
	if !srcFileStat.Mode().IsRegular() {
		e2e.Failf("source file %s is not a regular file", fromPath)
	}

	// open source file
	source, err := os.Open(fromPath)
	if err != nil {
		e2e.Failf("open source file %s failed: %v", fromPath, err)
	}
	defer source.Close()

	// open dest file
	saveTo := filepath.Join(e2e.TestContext.OutputDir, toFilename)
	dest, err := os.Create(saveTo)
	if err != nil {
		e2e.Failf("open destination file %s failed: %v", saveTo, err)
	}
	defer dest.Close()

	// copy from source to dest
	_, err = io.Copy(dest, source)
	if err != nil {
		e2e.Failf("copy file from %s to %s failed: %v", fromPath, saveTo, err)
	}
	return saveTo
}

func waitForBMHState(oc *exutil.CLI, bmhName string, bmhStatus string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 30*time.Minute, true, func(context.Context) (bool, error) {
		statusOp, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "bmh", bmhName, "-o=jsonpath={.status.provisioning.state}").Output()
		if err != nil {
			return false, err
		}
		if strings.Contains(statusOp, bmhStatus) {
			e2e.Logf("BMH state %v is %v", bmhName, bmhStatus)
			return true, nil
		}
		e2e.Logf("BMH %v state is %v, Trying again", bmhName, statusOp)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("The BMH state of %v is not as expected", bmhName))
}

func waitForBMHDeletion(oc *exutil.CLI, bmhName string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
		out, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "bmh", "-o=jsonpath={.items[*].metadata.name}").Output()
		if err != nil {
			return false, err
		}
		if !strings.Contains(out, bmhName) {
			e2e.Logf("bmh %v still exists is", bmhName)
			return true, nil
		}
		e2e.Logf("bmh %v exists, Trying again", bmhName)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "The BMH was not deleted as expected")
}

func getBypathDeviceName(oc *exutil.CLI, bmhName string) string {
	byPath, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.status.hardware.storage[0].name}").Output()
	o.Expect(err).ShouldNot(o.HaveOccurred())
	return byPath
}

// clusterOperatorHealthcheck check abnormal operators
func clusterOperatorHealthcheck(oc *exutil.CLI, waitTime int, dirname string) error {
	e2e.Logf("Check the abnormal operators")
	errCo := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, time.Duration(waitTime)*time.Second, false, func(cxt context.Context) (bool, error) {
		coLogFile, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "--no-headers").OutputToFile(dirname)
		if err == nil {
			file, err := os.Open(coLogFile)
			if err != nil {
				return false, nil
			}
			defer file.Close()

			pattern := regexp.MustCompile(`\.True.*False.*False`)
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if !pattern.MatchString(line) {
					// Found abnormal operator (line doesn't match healthy pattern)
					return false, nil
				}
			}
			if err := scanner.Err(); err != nil {
				return false, nil
			}
		} else {
			return false, nil
		}
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("No abnormality found in cluster operators...")
		return true, nil
	})
	if errCo != nil {
		err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return errCo
}

// clusterNodesHealthcheck check abnormal nodes
func clusterNodesHealthcheck(oc *exutil.CLI, waitTime int) error {
	errNode := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, time.Duration(waitTime)*time.Second, false, func(cxt context.Context) (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node").Output()
		if err == nil {
			if strings.Contains(output, "NotReady") || strings.Contains(output, "SchedulingDisabled") {
				return false, nil
			}
		} else {
			return false, nil
		}
		e2e.Logf("Nodes are normal...")
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		return true, nil
	})
	if errNode != nil {
		err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return errNode
}

// checkNodeStatus
func checkNodeStatus(oc *exutil.CLI, pollIntervalSec time.Duration, pollDurationMinute time.Duration, nodeName string, nodeStatus string) error {
	e2e.Logf("Check status of node %s", nodeName)
	errNode := wait.PollUntilContextTimeout(context.Background(), pollIntervalSec, pollDurationMinute, false, func(ctx context.Context) (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.status.conditions[3].status}").Output()
		if err != nil || string(output) != nodeStatus {
			e2e.Logf("Node status: %s. Trying again", output)
			return false, nil
		}
		if string(output) == nodeStatus {
			e2e.Logf("Node status: %s", output)
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(errNode, "Node did not change state as expected")
	return errNode
}

func buildFirmwareURL(vendor, currentVersion string) (string, string) {
	var url, fileName string

	iDRAC_71070 := "https://dl.dell.com/FOLDER11965413M/1/iDRAC_7.10.70.00_A00.exe"
	iDRAC_71030 := "https://dl.dell.com/FOLDER11319105M/1/iDRAC_7.10.30.00_A00.exe"
	ilo5_305 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-ilo/p991377599/v247527/ilo5_305.fwpkg"
	ilo5_302 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-ilo/p991377599/v243854/ilo5_302.fwpkg"
	ilo6_157 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-ilo/p788720876/v243858/ilo6_157.fwpkg"
	ilo6_160 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-ilo/p788720876/v247531/ilo6_160.fwpkg"

	switch vendor {
	case "Dell Inc.":
		fileName = "firmimgFIT.d9"
		switch currentVersion {
		case "7.10.70.00":
			url = iDRAC_71030
		case "7.10.30.00":
			url = iDRAC_71070
		default:
			url = iDRAC_71070 // Default to 7.10.70.00
		}
	case "HPE":
		// Extract the iLO version and assign the file name accordingly
		if strings.Contains(currentVersion, "iLO 5") {
			switch currentVersion {
			case "iLO 5 v3.05":
				url = ilo5_302
				fileName = "ilo5_302.bin"
			case "iLO 5 v3.02":
				url = ilo5_305
				fileName = "ilo5_305.bin"
			default:
				url = ilo5_305 // Default to v3.05
				fileName = "ilo5_305.bin"
			}
		} else if strings.Contains(currentVersion, "iLO 6") {
			switch currentVersion {
			case "iLO 6 v1.57":
				url = ilo6_160
				fileName = "ilo6_160.bin"
			case "iLO 6 v1.60":
				url = ilo6_157
				fileName = "ilo6_157.bin"
			default:
				url = ilo6_157 // Default to 1.57
				fileName = "ilo6_157.bin"
			}
		} else {
			g.Skip("Unsupported HPE BMC version")
		}
	default:
		g.Skip("Unsupported vendor")
	}

	return url, fileName
}

func setProxyEnv() {
	sharedProxy := filepath.Join(os.Getenv("SHARED_DIR"), "proxy-conf.sh")
	if _, err := os.Stat(sharedProxy); err == nil {
		e2e.Logf("proxy-conf.sh exists. Proxy environment variables are already set.")
		return
	}
	proxyFilePath := filepath.Join(os.Getenv(clusterProfileDir), proxyFile)
	if _, err := os.Stat(proxyFilePath); err == nil {
		content, err := ioutil.ReadFile(proxyFilePath)
		if err != nil {
			e2e.Failf("Failed to read file: %v", err)
		}
		proxyValue := strings.TrimSpace(string(content))
		proxyVars := []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"}
		for _, proxyVar := range proxyVars {
			if err := os.Setenv(proxyVar, proxyValue); err != nil {
				e2e.Failf("Failed to set %s: %v", proxyVar, err)
			}
		}
		noProxyValue := "localhost,127.0.0.1"
		os.Setenv("NO_PROXY", noProxyValue)
		os.Setenv("no_proxy", noProxyValue)
		e2e.Logf("Proxy environment variables are set.")
	} else if os.IsNotExist(err) {
		e2e.Failf("File does not exist at path: %s\n", proxyFilePath)
	} else {
		e2e.Failf("Error checking file: %v\n", err)
	}
}

func unsetProxyEnv() {
	sharedProxy := filepath.Join(os.Getenv("SHARED_DIR"), "proxy-conf.sh")
	if _, err := os.Stat(sharedProxy); err == nil {
		e2e.Logf("proxy-conf.sh exists. Not unsetting proxy enviornment variables.")
		return
	}
	proxyVars := []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "NO_PROXY", "no_proxy"}
	for _, proxyVar := range proxyVars {
		err := os.Unsetenv(proxyVar)
		if err != nil {
			e2e.Failf("Failed to unset %s: %v", proxyVar, err)
		}
	}
	e2e.Logf("Proxy environment variables are unset.")
}

func getHfsByVendor(oc *exutil.CLI, vendor, machineAPINamespace, host string) (string, string, error) {
	var hfs, value, currStatus string
	var err error

	switch vendor {
	case "Dell Inc.":
		hfs = "LogicalProc"
	case "HPE":
		hfs = "NetworkBootRetry"
	default:
		g.Skip("Unsupported vendor")
		return "", "", nil
	}

	currStatus, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("hfs", "-n", machineAPINamespace, host, fmt.Sprintf("-o=jsonpath={.status.settings.%s}", hfs)).Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch current status for %s: %v", hfs, err)
	}

	if currStatus == "Enabled" {
		value = "Disabled"
	} else {
		value = "Enabled"
	}

	return hfs, value, nil
}

func waitForError(oc *exutil.CLI, bmhName string, errorMessage string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 30*time.Minute, true, func(context.Context) (bool, error) {
		out, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "bmh", bmhName, "-o=jsonpath={.status.errorMessage}").Output()
		if err != nil {
			return false, err
		}
		if strings.Contains(out, errorMessage) {
			e2e.Logf("Expected error message occured: %s", errorMessage)
			return true, nil
		}
		e2e.Logf("Expected error did not occur yet, Trying again")
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "The Expected error did not occur during wait time")
}

func getNicFwDetails(vendor, currentVersion string) (string, string) {
	var url, fileName string

	// Dell NIC firmware URLs
	dellNic23310 := "https://dl.dell.com/FOLDER13278949M/2/Network_Firmware_9GMFK_LN64_23.31.0.BIN"
	dellNic23224 := "https://dl.dell.com/FOLDER12941539M/1/Network_Firmware_4HVHG_LN64_23.22.4.BIN"

	// HPE NIC firmware URLs
	hpeNic16354030 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-generic/p1590118926/v253823/16_35_4030-MCX512F-ACH_Ax_Bx.pldm.fwpkg"
	hpeNic16353502 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-generic/p1590118926/v248750/16_35_3502-MCX512F-ACH_Ax_Bx.pldm.fwpkg"

	switch vendor {
	case "Dell Inc.":
		switch currentVersion {
		case "23.31.0":
			url = dellNic23224
			fileName = "Network_Firmware_4HVHG_LN64_23.22.4.BIN"
		case "23.22.4":
			url = dellNic23310
			fileName = "Network_Firmware_9GMFK_LN64_23.31.0.BIN"
		default:
			// Default to latest known (23.31.0)
			url = dellNic23310
			fileName = "Network_Firmware_9GMFK_LN64_23.31.0.BIN"
		}

	case "HPE":
		switch currentVersion {
		case "16.35.4030":
			url = hpeNic16353502
			fileName = "16_35_3502-MCX512F-ACH_Ax_Bx.pldm.fwpkg"
		case "16.35.3502":
			url = hpeNic16354030
			fileName = "16_35_4030-MCX512F-ACH_Ax_Bx.pldm.fwpkg"
		default:
			url = hpeNic16354030 // Default to latest known (16.35.4030)
			fileName = "16_35_4030-MCX512F-ACH_Ax_Bx.pldm.fwpkg"
		}

	default:
		g.Skip("Unsupported NIC vendor")
	}

	return url, fileName
}

func getNicNameByVendor(vendor string) string {
	switch vendor {
	case "Dell Inc.":
		return "NIC.Embedded.1"
	case "HPE":
		return "DE081000"
	default:
		return "NIC.Embedded.1"
	}
}

// getCertificateValidityDays calculates the validity period of a certificate in days
// Returns the number of days between notBefore and notAfter dates
func getCertificateValidityDays(certFilePath string) (int, error) {
	// Get certificate start date
	startDateOutput, err := exec.Command("openssl", "x509", "-in", certFilePath, "-noout", "-startdate").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get certificate start date: %v", err)
	}
	startDateStr := strings.TrimSpace(string(startDateOutput))
	startDate := strings.TrimPrefix(startDateStr, "notBefore=")

	// Get certificate end date
	endDateOutput, err := exec.Command("openssl", "x509", "-in", certFilePath, "-noout", "-enddate").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get certificate end date: %v", err)
	}
	endDateStr := strings.TrimSpace(string(endDateOutput))
	endDate := strings.TrimPrefix(endDateStr, "notAfter=")

	// Calculate validity in days using Python
	pythonScript := fmt.Sprintf(`
from datetime import datetime
import sys

start_str = '%s'
end_str = '%s'

try:
    start = datetime.strptime(start_str, '%%b %%d %%H:%%M:%%S %%Y %%Z')
    end = datetime.strptime(end_str, '%%b %%d %%H:%%M:%%S %%Y %%Z')
except:
    # Fallback without timezone
    start = datetime.strptime(start_str.rsplit(' ', 1)[0], '%%b %%d %%H:%%M:%%S %%Y')
    end = datetime.strptime(end_str.rsplit(' ', 1)[0], '%%b %%d %%H:%%M:%%S %%Y')

days = (end - start).days
print(days)
`, startDate, endDate)

	validityOutput, err := exec.Command("python3", "-c", pythonScript).Output()
	if err != nil {
		return 0, fmt.Errorf("failed to calculate validity days: %v", err)
	}

	validityDays, err := strconv.Atoi(strings.TrimSpace(string(validityOutput)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse validity days: %v", err)
	}

	return validityDays, nil
}

// getCertificateDaysRemaining calculates how many days remain until certificate expiration
// Returns negative value if certificate is already expired
func getCertificateDaysRemaining(certFilePath string) (int, error) {
	// Get certificate end date
	endDateOutput, err := exec.Command("openssl", "x509", "-in", certFilePath, "-noout", "-enddate").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get certificate end date: %v", err)
	}
	endDateStr := strings.TrimSpace(string(endDateOutput))
	endDate := strings.TrimPrefix(endDateStr, "notAfter=")

	// Calculate remaining days using Python
	pythonScript := fmt.Sprintf(`
from datetime import datetime
import sys

end_str = '%s'

try:
    end = datetime.strptime(end_str, '%%b %%d %%H:%%M:%%S %%Y %%Z')
except:
    end = datetime.strptime(end_str.rsplit(' ', 1)[0], '%%b %%d %%H:%%M:%%S %%Y')

now = datetime.utcnow()
remaining = (end - now).days
print(remaining)
`, endDate)

	remainingOutput, err := exec.Command("python3", "-c", pythonScript).Output()
	if err != nil {
		return 0, fmt.Errorf("failed to calculate remaining days: %v", err)
	}

	remainingDays, err := strconv.Atoi(strings.TrimSpace(string(remainingOutput)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse remaining days: %v", err)
	}

	return remainingDays, nil
}

// isIPv6 checks if a given string is a valid IPv6 address
func isIPv6(ip string) bool {
	matched, _ := regexp.MatchString(`([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}`, ip)
	return matched && strings.Contains(ip, ":")
}

// normalizeIPv6 normalizes an IPv6 address to canonical form using Python
// Returns the normalized address (lowercase, compressed) or original if normalization fails
func normalizeIPv6(ipv6 string) string {
	// Use Python to normalize IPv6 (handles all edge cases)
	pythonScript := fmt.Sprintf(`
import ipaddress
try:
    addr = ipaddress.IPv6Address('%s')
    print(str(addr))
except:
    print('%s')
`, ipv6, ipv6)

	output, err := exec.Command("python3", "-c", pythonScript).Output()
	if err != nil {
		return ipv6 // Return original if normalization fails
	}
	return strings.TrimSpace(string(output))
}

// isDualStackCluster checks if the cluster has both IPv4 and IPv6 API VIPs
func isDualStackCluster(oc *exutil.CLI) (bool, []string, []string, error) {
	apiVIPsJSON, err := oc.AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.baremetal.apiServerInternalIPs}").Output()
	if err != nil {
		return false, nil, nil, err
	}

	if apiVIPsJSON == "" || apiVIPsJSON == "[]" {
		return false, nil, nil, nil
	}

	var ipv4Addrs []string
	var ipv6Addrs []string

	// Parse JSON array
	var ips []string
	jsonErr := json.Unmarshal([]byte(apiVIPsJSON), &ips)
	if jsonErr != nil {
		return false, nil, nil, jsonErr
	}

	for _, ip := range ips {
		if isIPv6(ip) {
			ipv6Addrs = append(ipv6Addrs, ip)
		} else {
			ipv4Addrs = append(ipv4Addrs, ip)
		}
	}

	isDualStack := len(ipv4Addrs) > 0 && len(ipv6Addrs) > 0
	return isDualStack, ipv4Addrs, ipv6Addrs, nil
}
