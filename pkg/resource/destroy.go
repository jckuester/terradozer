package resource

import (
	"fmt"

	"github.com/apex/log"
	"github.com/jckuester/terradozer/internal"
)

// DestroyableResource implementations can destroy a Terraform resource.
type DestroyableResource interface {
	Destroy() error
	Type() string
	ID() string
}

// DestroyResources destroys a given list of resources, which may depend on each other.
//
// If at least one resource is successfully destroyed per run (iteration through the list of given resources),
// the remaining, failed resources will be retried in a next run (until all resources are destroyed or
// some destroys have permanently failed).
func DestroyResources(resources []DestroyableResource, parallel int) int {
	numOfResourcesToDelete := len(resources)
	numOfDeletedResources := 0

	var retryableResourceErrors []RetryDestroyError

	jobQueue := make(chan DestroyableResource, numOfResourcesToDelete)

	workerResults := make(chan workerResult, numOfResourcesToDelete)

	for i := 1; i <= parallel; i++ {
		go workerDestroy(jobQueue, workerResults)
	}

	log.Debug("start distributing resources to workers for this run")

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
		var resourcesToRetry []DestroyableResource
		for _, retryErr := range retryableResourceErrors {
			resourcesToRetry = append(resourcesToRetry, retryErr.Resource)
		}

		numOfDeletedResources += DestroyResources(resourcesToRetry, parallel)
	}

	if len(retryableResourceErrors) > 0 && numOfDeletedResources == 0 {
		internal.LogTitle(fmt.Sprintf("failed to delete the following resources (retries exceeded): %d",
			len(retryableResourceErrors)))

		for _, err := range retryableResourceErrors {
			log.WithError(err).WithField("id", err.Resource.ID()).Warn(internal.Pad(err.Resource.Type()))
		}
	}

	return numOfDeletedResources
}

type workerResult struct {
	resourceHasBeenDeleted bool
	// if set, it is worth retrying to delete this resource
	Err *RetryDestroyError
}

// workerDestroy is a worker that destroys a resource.
func workerDestroy(resources <-chan DestroyableResource, result chan<- workerResult) {
	for r := range resources {
		err := r.Destroy()
		if err != nil {
			switch err := err.(type) {
			case *RetryDestroyError:
				log.WithFields(log.Fields{
					"type":        r.Type(),
					"resource_id": r.ID(),
				}).Info(internal.Pad("will retry to delete resource"))

				result <- workerResult{
					Err: err,
				}

			default:
				log.WithError(err).WithFields(log.Fields{
					"type":        r.Type(),
					"resource_id": r.ID(),
				}).Debug(internal.Pad("unable to delete resource"))

				result <- workerResult{}
			}

			continue
		}

		result <- workerResult{
			resourceHasBeenDeleted: true,
		}
	}
}

// Destroy destroys a Terraform resource.
func (r Resource) Destroy() error {
	if r.State() == nil {
		return fmt.Errorf("resource state is nil; need to call update first")
	}

	err := r.Provider.DestroyResource(r.Type(), *r.State())
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"id": r.ID(), "type": r.Type()}).Debug(internal.Pad("failed to delete resource"))

		return NewRetryDestroyError(err, &r)
	}

	log.WithField("id", r.ID()).Error(internal.Pad(r.Type()))

	return nil
}
