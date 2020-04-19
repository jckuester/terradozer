// Package resource manages the deletion of Terraform resources.
package resource

import (
	"fmt"

	"github.com/hashicorp/terraform/configs/configschema"

	"github.com/zclconf/go-cty/cty"

	"github.com/apex/log"
	"github.com/jckuester/terradozer/internal"
	"github.com/jckuester/terradozer/pkg/provider"
)

// DestroyableResource implementations can destroy a Terraform resource.
type DestroyableResource interface {
	Destroy(bool) error
	Type() string
	ID() string
}

// Resource represents a Terraform resource that can be destroyed.
type Resource struct {
	// terraformType is the Terraform type of a resource
	terraformType string
	// id is used by the provider to import and delete the resource
	id string
	// provider is able to delete a resource
	provider *provider.TerraformProvider
}

// New creates a destroyable Terraform resource.
//
// To destroy a resource, its Terraform Type and ID
// (which both together uniquely identify a resource), plus a provider that
// will handle the destroy is needed.
func New(terraformType, id string, provider *provider.TerraformProvider) *Resource {
	return &Resource{
		terraformType: terraformType,
		id:            id,
		provider:      provider,
	}
}

// Type returns the type a Terraform resource.
func (r Resource) Type() string {
	return r.terraformType
}

// Type returns the ID a Terraform resource.
func (r Resource) ID() string {
	return r.id
}

// Destroy destroys a Terraform resource.
func (r Resource) Destroy(dryRun bool) error {
	resourceSchema, ok := r.provider.GetSchema().ResourceTypes[r.Type()]
	if !ok {
		return fmt.Errorf("failed to get schema for resource")
	}

	currentResourceState, err := r.provider.ReadResource(r.Type(), emptyValueWitID(r.ID(), resourceSchema.Block))
	if err != nil {
		return fmt.Errorf("failed to read current state of resource: %s", err)
	}

	resourceNotFound := currentResourceState.IsNull()
	if resourceNotFound {
		return fmt.Errorf("resource found in state doesn't exist anymore")
	}

	if dryRun {
		log.WithField("id", r.id).Warn(internal.Pad(r.terraformType))

		return nil
	}

	err = r.provider.DestroyResource(r.terraformType, currentResourceState)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"id": r.id, "type": r.terraformType}).Debug(internal.Pad("failed to delete resource"))

		return NewRetryDestroyError(err, r)
	}

	log.WithField("id", r.id).Error(internal.Pad(r.terraformType))

	return nil
}

// emptyValueWitID returns a non-null object for the configuration block
// where all of the attribute values are set to empty values except the ID attribute.
//
// see also github.com/hashicorp/terraform/configs/configschema/empty_value.go
func emptyValueWitID(id string, block *configschema.Block) cty.Value {
	vals := make(map[string]cty.Value)

	for name, attrS := range block.Attributes {
		vals[name] = attrS.EmptyValue()
	}

	for name, blockS := range block.BlockTypes {
		vals[name] = blockS.EmptyValue()
	}

	vals["id"] = cty.StringVal(id)

	return cty.ObjectVal(vals)
}

// DestroyResources destroys a given list of resources, which may depend on each other.
//
// If at least one resource is successfully destroyed per run (iteration through the list of given resources),
// the remaining, failed resources will be retried in a next run (until all resources are destroyed or
// some destroys have permanently failed).
func DestroyResources(resources []DestroyableResource, dryRun bool, parallel int) int {
	numOfResourcesToDelete := len(resources)
	numOfDeletedResources := 0

	var retryableResourceErrors []RetryDestroyError

	jobQueue := make(chan DestroyableResource, numOfResourcesToDelete)

	workerResults := make(chan workerResult, numOfResourcesToDelete)

	for workerID := 1; workerID <= parallel; workerID++ {
		go worker(workerID, dryRun, jobQueue, workerResults)
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

		numOfDeletedResources += DestroyResources(resourcesToRetry, dryRun, parallel)
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

func worker(id int, dryRun bool, resources <-chan DestroyableResource, result chan<- workerResult) {
	for r := range resources {
		log.WithFields(log.Fields{
			"worker_id": id,
			"type":      r.Type(),
			"id":        r.ID(),
		}).Debug(internal.Pad("worker starts deleting resource"))

		err := r.Destroy(dryRun)
		if err != nil {
			switch err := err.(type) {
			case *RetryDestroyError:
				log.WithFields(log.Fields{
					"type": r.Type(),
					"id":   r.ID(),
				}).Info(internal.Pad("will retry to delete resource"))

				result <- workerResult{
					Err: err,
				}

			default:
				log.WithError(err).WithFields(log.Fields{
					"type": r.Type(),
					"id":   r.ID(),
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
