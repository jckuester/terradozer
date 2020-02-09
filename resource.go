package main

import (
	"fmt"

	"github.com/apex/log"
)

type Resource struct {
	// TerraformType is the Terraform type of a resource
	TerraformType string
	// Provider must be able to delete a resource
	Provider *TerraformProvider
	// id is used by the provider to import and delete the resource
	id string
}

func (r Resource) Type() string {
	return r.TerraformType
}

func (r Resource) ID() string {
	return r.id
}

// Delete deletes a Terraform resource via the corresponding Terraform Provider
func (r Resource) Delete(dryRun bool) error {
	importResp := r.Provider.importResource(r.TerraformType, r.id)
	if importResp.Diagnostics.HasErrors() {
		return fmt.Errorf("failed to import resource: %s", importResp.Diagnostics.Err())
	}

	for _, resImp := range importResp.ImportedResources {
		log.WithField("state", resImp.State.GoString()).Debug(Pad("imported resource state"))

		readResp := r.Provider.readResource(resImp)
		if readResp.Diagnostics.HasErrors() {
			return fmt.Errorf("failed to read current state of resource: %s", readResp.Diagnostics.Err())
		}

		log.WithField("state", readResp.NewState.GoString()).Debug(Pad("read resource state"))

		resourceNotFound := readResp.NewState.IsNull()
		if resourceNotFound {
			return fmt.Errorf("resource found in state doesn't exist anymore")
		}

		if dryRun {
			log.WithField("id", r.id).Warn(Pad(r.TerraformType))

			return nil
		}

		respApply := r.Provider.destroy(r.TerraformType, readResp.NewState)
		if respApply.Diagnostics.HasErrors() {
			log.WithError(respApply.Diagnostics.Err()).Debug(Pad("failed to delete resource"))

			return RetryableError(respApply.Diagnostics.Err(), r)
		}

		log.WithField("state", respApply.NewState.GoString()).Debug(Pad("new resource state after apply"))

		log.WithFields(log.Fields{"type": r.TerraformType, "id": r.id}).Error(Pad("resource deleted"))
	}

	return nil
}

type DeletableResource interface {
	Delete(bool) error
	Type() string
	ID() string
}

// Delete erases a given list of resources, which might depend on each other.
// If at least one resource is successfully deleted per run (iteration of the list), the remaining,
// failed resources will be retried in a next run (until all resources are deleted or
// some deletions have permanently failed).
func Delete(resources []DeletableResource, dryRun bool, parallel int) int {
	numOfResourcesToDelete := len(resources)
	numOfDeletedResources := 0

	var retryableResourceErrors []RetryResourceError

	jobQueue := make(chan DeletableResource, numOfResourcesToDelete)

	workerResults := make(chan workerResult, numOfResourcesToDelete)

	for workerID := 1; workerID <= parallel; workerID++ {
		go worker(workerID, dryRun, jobQueue, workerResults)
	}

	log.Debug("start distributing resources to workers")

	for _, r := range resources {
		jobQueue <- r
	}

	close(jobQueue)

	for i := 1; i <= numOfResourcesToDelete; i++ {
		result := <-workerResults

		if result.resourceHasBeenDeleted {
			numOfDeletedResources++

			continue
		}

		if result.Err != nil {
			retryableResourceErrors = append(retryableResourceErrors, *result.Err)
		}
	}

	if len(retryableResourceErrors) > 0 && numOfDeletedResources > 0 {
		var resourcesToRetry []DeletableResource
		for _, retryErr := range retryableResourceErrors {
			resourcesToRetry = append(resourcesToRetry, retryErr.Resource)
		}

		numOfDeletedResources += Delete(resourcesToRetry, dryRun, parallel)
	}

	if len(retryableResourceErrors) > 0 && numOfDeletedResources == 0 {
		LogTitle("failed to delete the following resources (retries exceeded)")

		for _, err := range retryableResourceErrors {
			log.WithError(err).WithField("id", err.Resource.ID()).Warn(Pad(err.Resource.Type()))
		}
	}

	return numOfDeletedResources
}

type workerResult struct {
	resourceHasBeenDeleted bool
	// if set, it is worth retrying to delete this resource
	Err *RetryResourceError
}

func worker(id int, dryRun bool, resources <-chan DeletableResource, result chan<- workerResult) {
	for r := range resources {
		log.WithFields(log.Fields{
			"worker_id": id,
			"type":      r.Type(),
			"id":        r.ID(),
		}).Debug(Pad("worker starts deleting resource"))

		err := r.Delete(dryRun)
		if err != nil {
			switch err := err.(type) {
			case *RetryResourceError:
				log.WithFields(log.Fields{
					"type": r.Type(),
					"id":   r.ID(),
				}).Info(Pad("will retry to delete resource"))

				result <- workerResult{
					Err: err,
				}

			default:
				log.WithError(err).WithFields(log.Fields{
					"type": r.Type(),
					"id":   r.ID(),
				}).Debug(Pad("unable to delete resource"))

				result <- workerResult{}
			}

			continue
		}

		result <- workerResult{
			resourceHasBeenDeleted: true,
		}
	}
}
