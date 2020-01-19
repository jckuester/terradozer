package main

//go:generate mockgen -source=provider.go -destination=provider_mock_test.go -package=main
//go:generate mockgen -source=main.go -destination=main_mock_test.go -package=main

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
		logrus.WithError(err).Error("failed to initialize Terraform providers needed for deletion of resources")
		return 1
	}

	resources, err := state.Resources()
	if err != nil {
		logrus.WithError(err).Error("failed to get resources from state")
		return 1
	}

	logrus.Infof("total number of resources deleted: %d\n", delete(resources, providers))

	return 0
}

type ResourceDeleter interface {
	Delete(r Resource, dryRun bool) bool
}

func delete(resources []Resource, providers map[string]ResourceDeleter) int {
	deletionCount := 0
	var resFailed []Resource

	for _, r := range resources {
		p, ok := providers[r.Provider]
		if !ok {
			logrus.Debugf("Terraform provider not found in provider list: %s", r.Provider)
			continue
		}

		deleted := p.(ResourceDeleter).Delete(r, dryRun)
		if deleted {
			logrus.Debugf("resource deleted (type=%s, ID=%s)", r.Type, r.ID)
			deletionCount++
		} else {
			resFailed = append(resFailed, r)
		}
	}

	if len(resFailed) > 0 && deletionCount > 0 {
		logrus.Debugf("retrying to delete resources that possibly were dependencies before: %+v", resFailed)

		deletionCount += delete(resFailed, providers)
	}

	return deletionCount
}
