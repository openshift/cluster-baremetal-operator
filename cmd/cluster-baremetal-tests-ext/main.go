package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	extcmd "github.com/openshift-eng/openshift-tests-extension/pkg/cmd"
	e "github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	g "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	// Import test packages to register Ginkgo specs
	_ "github.com/openshift/cluster-baremetal-tests-ext/test/e2e/openshift/baremetal"
)

func main() {
	// Extension registry
	registry := e.NewRegistry()

	// Create extension
	ext := e.NewExtension(
		"openshift",           // product
		"payload",             // type
		"cluster-baremetal",   // component name
	)

	// Add suites to the extension
	ext.AddSuite(e.Suite{
		Name:    "cluster-baremetal/all",
		Parents: []string{"openshift/conformance/parallel"},
	})

	// Build test specs from Ginkgo tests automatically
	specs, err := g.BuildExtensionTestSpecsFromOpenShiftGinkgoSuite()
	if err != nil {
		fmt.Fprintf(os.Stderr, "couldn't build extension test specs from ginkgo: %v\n", err)
		os.Exit(1)
	}

	// Apply environment selectors - baremetal platform only
	// All tests should only run on baremetal platform
	specs.Select(et.NameContains("")).
		Include(et.PlatformEquals("baremetal"))

	// Add specs to extension
	ext.AddSpecs(specs)
	registry.Register(ext)

	// Create root command
	rootCmd := &cobra.Command{
		Use:   "cluster-baremetal-tests-ext",
		Short: "Cluster BareMetal Operator Test Extension",
		Long:  "OpenShift Tests Extension for Cluster BareMetal Operator E2E Tests",
	}

	// Register OTE subcommands (info, list, run-test, run-suite, etc.)
	rootCmd.AddCommand(extcmd.DefaultExtensionCommands(registry)...)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
