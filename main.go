package main

//go:generate mockgen -source=pkg/provider/provider.go -destination=pkg/resource/provider_mock_test.go -package=resource
//go:generate mockgen -source=pkg/resource/resource.go -destination=pkg/resource/resource_mock_test.go -package=resource

import (
	"flag"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/jckuester/terradozer/internal"
	"github.com/jckuester/terradozer/pkg/provider"
	"github.com/jckuester/terradozer/pkg/resource"
	"github.com/jckuester/terradozer/pkg/state"
)

var (
	dryRun      bool
	force       bool
	logDebug    bool
	pathToState string
	parallel    int
	timeout     string
	version     bool
)

//nolint:gochecknoinits
func init() {
	flag.StringVar(&timeout, "timeout", "30s", "Amount of time to wait for a destroy of a resource to finish")
	flag.BoolVar(&dryRun, "dry", false, "Don't delete anything")
	flag.BoolVar(&force, "force", false, "Delete without asking for confirmation")
	flag.BoolVar(&logDebug, "debug", false, "Enable debug logging")
	flag.StringVar(&pathToState, "state", "terraform.tfstate", "Path to a Terraform state file")
	flag.IntVar(&parallel, "parallel", 10, "Limit the number of concurrent delete operations")
	flag.BoolVar(&version, "version", false, "Show application version")
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	flag.Parse()

	log.SetHandler(cli.Default)

	fmt.Println()
	defer fmt.Println()

	if logDebug {
		log.SetLevel(log.DebugLevel)
	}

	// discard TRACE logs of GRPCProvider
	stdlog.SetOutput(ioutil.Discard)

	if version {
		fmt.Println(internal.BuildVersionString())
		return 0
	}

	if force && dryRun {
		log.Error("-force and -dry flag cannot be used together")
		return 1
	}

	tfstate, err := state.New(pathToState)
	if err != nil {
		log.WithError(err).Error("failed to get Terraform state")
		return 1
	}

	internal.LogTitle("reading state")
	log.WithField("file", pathToState).Info(internal.Pad("using state"))

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		log.WithError(err).Error("failed to parse timeout")
		return 1
	}

	providers, err := provider.InitProviders(tfstate.ProviderNames(), timeoutDuration)
	if err != nil {
		log.WithError(err).Error("failed to initialize Terraform providers")
		return 1
	}

	resources, err := tfstate.Resources(providers)
	if err != nil {
		log.WithError(err).Error("failed to get resources from Terraform state")
		return 1
	}

	if !force {
		internal.LogTitle("showing resources that would be deleted (dry run)")

		// always show the resources that would be affected before deleting anything
		numDeletedResources := resource.DestroyResources(resources, true, parallel)

		if numDeletedResources == 0 {
			internal.LogTitle("all resources have already been deleted")
			return 0
		}

		internal.LogTitle(fmt.Sprintf("total number of resources that would be deleted: %d", numDeletedResources))
	}

	if !dryRun {
		if !internal.UserConfirmedDeletion(os.Stdin, force) {
			return 0
		}

		internal.LogTitle("Starting to delete resources")

		numDeletedResources := resource.DestroyResources(resources, false, parallel)

		internal.LogTitle(fmt.Sprintf("total number of deleted resources: %d", numDeletedResources))
	}

	return 0
}
