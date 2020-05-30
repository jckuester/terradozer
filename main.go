package main

//nolint:lll
//go:generate mockgen -source=pkg/resource/update.go -destination=pkg/resource/update_mock_test.go -package=resource_test
//go:generate mockgen -source=pkg/resource/destroy.go -destination=pkg/resource/destroy_mock_test.go -package=resource_test

import (
	"flag"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/fatih/color"
	"github.com/jckuester/terradozer/internal"
	"github.com/jckuester/terradozer/pkg/provider"
	"github.com/jckuester/terradozer/pkg/resource"
	"github.com/jckuester/terradozer/pkg/state"
)

func main() {
	os.Exit(mainExitCode())
}

//nolint:wsl
func mainExitCode() int {
	var dryRun bool
	var force bool
	var logDebug bool
	var parallel int
	var timeout string
	var version bool

	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	flags.Usage = func() {
		printHelp(flags)
	}

	flags.StringVar(&timeout, "timeout", "30s", "Amount of time to wait for a destroy of a resource to finish")
	flags.BoolVar(&dryRun, "dry-run", false, "Show what would be destroyed")
	flags.BoolVar(&force, "force", false, "Destroy without asking for confirmation")
	flags.BoolVar(&logDebug, "debug", false, "Enable debug logging")
	flags.IntVar(&parallel, "parallel", 10, "Limit the number of concurrent destroy operations")
	flags.BoolVar(&version, "version", false, "Show application version")

	_ = flags.Parse(os.Args[1:])
	args := flags.Args()

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
		fmt.Fprint(os.Stderr, color.RedString("Error:️ -force and -dry-run flag cannot be used together\n"))
		printHelp(flags)

		return 1
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		fmt.Fprint(os.Stderr, color.RedString("Error: failed to parse timeout flag: %s\n", err))
		printHelp(flags)

		return 1
	}

	if len(args) == 0 {
		fmt.Fprint(os.Stderr, color.RedString("Error: path to Terraform state file expected\n"))
		printHelp(flags)

		return 1
	}

	pathToState := args[0]

	tfstate, err := state.New(pathToState)
	if err != nil {
		fmt.Fprint(os.Stderr, color.RedString("Error:️ failed to read Terraform state file: %s\n", err))

		return 1
	}

	internal.LogTitle("reading state")
	log.WithField("file", pathToState).Info(internal.Pad("using state"))

	providers, err := provider.InitProviders(tfstate.ProviderNames(), "~/.terradozer", timeoutDuration)
	if err != nil {
		fmt.Fprint(os.Stderr, color.RedString("\nError:️ failed to initialize Terraform providers: %s\n", err))

		return 1
	}

	resources, err := tfstate.Resources(providers)
	if err != nil {
		fmt.Fprint(os.Stderr, color.RedString("\nError:️ failed to get resources from Terraform state: %s\n", err))

		return 1
	}

	resourcesWithUpdatedState := resource.UpdateResources(resources, parallel)

	if !force {
		internal.LogTitle("showing resources that would be deleted (dry run)")

		// always show the resources that would be affected before deleting anything
		for _, r := range resourcesWithUpdatedState {
			log.WithField("id", r.ID()).Warn(internal.Pad(r.Type()))
		}

		if len(resourcesWithUpdatedState) == 0 {
			internal.LogTitle("all resources have already been deleted")
			return 0
		}

		internal.LogTitle(fmt.Sprintf("total number of resources that would be deleted: %d",
			len(resourcesWithUpdatedState)))
	}

	if !dryRun {
		if !internal.UserConfirmedDeletion(os.Stdin, force) {
			return 0
		}

		internal.LogTitle("Starting to delete resources")

		numDeletedResources := resource.DestroyResources(
			convertToDestroyableResources(resourcesWithUpdatedState), parallel)

		internal.LogTitle(fmt.Sprintf("total number of deleted resources: %d", numDeletedResources))
	}

	return 0
}

func convertToDestroyableResources(resources []resource.UpdatableResource) []resource.DestroyableResource {
	var result []resource.DestroyableResource

	for _, r := range resources {
		result = append(result, r.(resource.DestroyableResource))
	}

	return result
}

func printHelp(fs *flag.FlagSet) {
	fmt.Fprintf(os.Stderr, "\n"+strings.TrimSpace(help)+"\n")
	fs.PrintDefaults()
	fmt.Println()
}

const help = `
Terraform destroy using only the state - no *.tf files needed.

USAGE:
  $ terradozer [flags] <path/to/terraform.tfstate>

FLAGS:
`
