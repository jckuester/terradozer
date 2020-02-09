package main

//go:generate mockgen -source=provider.go -destination=provider_mock_test.go -package=main
//go:generate mockgen -source=resource.go -destination=resource_mock_test.go -package=main

import (
	"flag"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"

	"github.com/apex/log/handlers/cli"

	"github.com/apex/log"
)

var (
	dryRun      bool
	force       bool
	logDebug    bool
	pathToState string
	parallel    int
)

//nolint:gochecknoinits
func init() {
	flag.BoolVar(&dryRun, "dry", false, "Don't delete anything")
	flag.BoolVar(&force, "force", false, "Delete without asking for confirmation")
	flag.BoolVar(&logDebug, "debug", false, "Enable debug logging")
	flag.StringVar(&pathToState, "state", "terraform.tfstate", "Path to a Terraform state file")
	flag.IntVar(&parallel, "parallel", 10, "Limit the number of concurrent delete operations")
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	flag.Parse()

	log.SetHandler(cli.Default)

	if logDebug {
		log.SetLevel(log.DebugLevel)
	}

	// discard TRACE logs of GRPCProvider
	stdlog.SetOutput(ioutil.Discard)

	if force && dryRun {
		log.Error("-force and -dry flag cannot be used together")
		return 1
	}

	state, err := NewState(pathToState)
	if err != nil {
		log.WithError(err).Error("failed to get Terraform state")
		return 1
	}

	LogTitle(Pad("reading state"))
	log.WithField("file", pathToState).Info(Pad("using state"))

	providers, err := InitProviders(state.ProviderNames())
	if err != nil {
		log.WithError(err).Error("failed to initialize Terraform providers")
		return 1
	}

	resources, err := state.Resources(providers)
	if err != nil {
		log.WithError(err).Error("failed to get resources from Terraform state")
		return 1
	}

	if !force {
		LogTitle("showing resources that would be deleted (dry run)")

		// always show the resources that would be affected before deleting anything
		numDeletedResources := Delete(resources, true, parallel)

		if numDeletedResources == 0 {
			LogTitle("all resources have already been deleted")
			return 0
		}

		LogTitle(fmt.Sprintf("total number of resources that would be deleted: %d", numDeletedResources))
	}

	if !dryRun {
		if !userConfirmedDeletion(force) {
			return 0
		}

		LogTitle("Starting to delete resources")

		numDeletedResources := Delete(resources, false, parallel)

		LogTitle(fmt.Sprintf("total number of deleted resources: %d", numDeletedResources))
	}

	return 0
}

// userConfirmedDeletion asks the user to confirm deletion of resources
func userConfirmedDeletion(force bool) bool {
	if force {
		LogTitle("user will not be asked for confirmation (force mode)")
		return true
	}

	log.Info("Are you sure you want to delete these resources (cannot be undone)? Only YES will be accepted.")
	fmt.Print(fmt.Sprintf("%23v", "Enter a value: "))

	var response string

	_, err := fmt.Scanln(&response)
	if err != nil {
		log.Fatal(err.Error())
	}

	if response == "YES" {
		return true
	}

	return false
}
