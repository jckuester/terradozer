package main

//go:generate mockgen -source=provider.go -destination=provider_mock_test.go -package=main
//go:generate mockgen -source=resource.go -destination=resource_mock_test.go -package=main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"
)

var (
	dryRun      bool
	logDebug    bool
	pathToState string
)

func init() {
	flag.BoolVar(&dryRun, "dry", false, "Don't delete anything")
	flag.BoolVar(&logDebug, "debug", false, "Enable debug logging")
	flag.StringVar(&pathToState, "state", "terraform.tfstate", "Path to a Terraform state file")
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	flag.Parse()

	// discard TRACE logs of GRPCProvider
	log.SetOutput(ioutil.Discard)

	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	if logDebug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	state, err := NewState(pathToState)
	if err != nil {
		logrus.WithError(err).Error("failed to get Terraform state")
		return 1
	}
	logrus.Infof("using state: %s", pathToState)

	providers, err := InitProviders(state.ProviderNames())
	if err != nil {
		logrus.WithError(err).Error("failed to initialize Terraform providers")
		return 1
	}

	resources, err := state.Resources(providers)
	if err != nil {
		logrus.WithError(err).Error("failed to get resources from Terraform state")
		return 1
	}

	numDeletedResources := Delete(resources, dryRun)

	if dryRun {
		logrus.Infof("total number of resources that would be deleted: %d", numDeletedResources)
	} else {
		logrus.Infof("total number of deleted resources: %d", numDeletedResources)
	}

	return 0
}
