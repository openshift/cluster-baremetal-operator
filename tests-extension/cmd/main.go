package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/component-base/logs"

	"github.com/openshift-eng/openshift-tests-extension/pkg/cmd"
	e "github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	g "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	"github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	framework "k8s.io/kubernetes/test/e2e/framework"

	// Import testdata package from this module
	_ "github.com/openshift/cluster-baremetal-operator-tests-extension/openshift/test/e2e/testdata"

	// Import test packages from this module
	_ "github.com/openshift/cluster-baremetal-operator-tests-extension/openshift/test/e2e"
)

func main() {
	// Initialize test framework flags (required for kubeconfig, provider, etc.)
	util.InitStandardFlags()
	framework.AfterReadingAllFlags(&framework.TestContext)

	logs.InitLogs()
	defer logs.FlushLogs()

	registry := e.NewRegistry()
	ext := e.NewExtension("openshift", "payload", "cluster-baremetal-operator")

	// Register test suites (parallel, serial, disruptive, all)
	registerSuites(ext)

	// Build test specs from Ginkgo
	// Note: ModuleTestsOnly() is applied by default, which filters out /vendor/ and k8s.io/kubernetes tests
	allSpecs, err := g.BuildExtensionTestSpecsFromOpenShiftGinkgoSuite()
	if err != nil {
		panic(fmt.Sprintf("couldn't build extension test specs from ginkgo: %+v", err.Error()))
	}

	// Filter to only include tests from this module's test directory
	// Excludes tests from /go/pkg/mod/ (module cache) and /vendor/
	componentSpecs := allSpecs.Select(func(spec *et.ExtensionTestSpec) bool {
		for _, loc := range spec.CodeLocations {
			// Include tests from local test directory (not from module cache or vendor)
			if strings.Contains(loc, "/openshift/test/e2e/") && !strings.Contains(loc, "/go/pkg/mod/") && !strings.Contains(loc, "/vendor/") {
				return true
			}
		}
		return false
	})

	// Initialize test framework before all tests
	componentSpecs.AddBeforeAll(func() {
		if err := compat_otp.InitTest(false); err != nil {
			fmt.Printf("ERROR: compat_otp.InitTest failed: %v\n", err)
			panic(err)
		}
		fmt.Println("DEBUG: compat_otp.InitTest completed successfully")

		// Set testsStarted = true to allow OTP functions like oc.Run() to work
		// WithCleanup sets this flag and it remains true for all subsequent tests
		fmt.Println("DEBUG: Calling util.WithCleanup")
		util.WithCleanup(func() {
			// Empty function - we just need WithCleanup to set testsStarted = true
		})
		fmt.Println("DEBUG: BeforeAll completed successfully")
	})

	// Process all specs
	componentSpecs.Walk(func(spec *et.ExtensionTestSpec) {
		// CRITICAL: All cluster-baremetal-operator tests ONLY run on baremetal platform
		// Note: Platform name is case-sensitive - cluster reports "BareMetal"
		spec.Include(et.PlatformEquals("BareMetal"))

		// Apply platform filters based on Platform: labels
		for label := range spec.Labels {
			if strings.HasPrefix(label, "Platform:") {
				platformName := strings.TrimPrefix(label, "Platform:")
				spec.Include(et.PlatformEquals(platformName))
			}
		}

		// Apply platform filters based on [platform:xxx] in test names
		re := regexp.MustCompile(`\[platform:([a-z]+)\]`)
		if match := re.FindStringSubmatch(spec.Name); match != nil {
			platform := match[1]
			spec.Include(et.PlatformEquals(platform))
		}

		// Set lifecycle to Informing
		spec.Lifecycle = et.LifecycleInforming
	})

	// Add filtered component specs to extension
	ext.AddSpecs(componentSpecs)

	registry.Register(ext)

	root := &cobra.Command{
		Long: "Cluster Baremetal Operator Tests",
	}

	root.AddCommand(cmd.DefaultExtensionCommands(registry)...)

	if err := func() error {
		return root.Execute()
	}(); err != nil {
		os.Exit(1)
	}
}

// registerSuites registers test suites with proper categorization
func registerSuites(ext *e.Extension) {
	suites := []e.Suite{
		{
			Name: "cluster-baremetal-operator/conformance/parallel",
			Parents: []string{
				"openshift/conformance/parallel",
			},
			Description: "Parallel conformance tests (Level0, non-serial, non-disruptive)",
			Qualifiers: []string{
				`name.contains("[Level0]") && !(name.contains("[Serial]") || name.contains("[Disruptive]"))`,
			},
		},
		{
			Name: "cluster-baremetal-operator/conformance/serial",
			Parents: []string{
				"openshift/conformance/serial",
			},
			Description: "Serial conformance tests (must run sequentially)",
			Qualifiers: []string{
				`name.contains("[Level0]") && name.contains("[Serial]") && !name.contains("[Disruptive]")`,
			},
		},
		{
			Name:        "cluster-baremetal-operator/disruptive",
			Parents:     []string{"openshift/disruptive"},
			Description: "Disruptive tests (may affect cluster state)",
			Qualifiers: []string{
				`name.contains("[Disruptive]")`,
			},
		},
		{
			Name:        "cluster-baremetal-operator/non-disruptive",
			Description: "All non-disruptive tests (safe for development clusters)",
			Qualifiers: []string{
				`!name.contains("[Disruptive]")`,
			},
		},
		{
			Name:        "cluster-baremetal-operator/all",
			Description: "All cluster-baremetal-operator tests",
			// No qualifiers means all tests from this extension will be included
		},
	}

	for _, suite := range suites {
		ext.AddSuite(suite)
	}
}
