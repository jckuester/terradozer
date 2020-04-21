// Package resource manages the deletion of Terraform resources.
package resource

import (
	"fmt"

	"github.com/apex/log"
	"github.com/hashicorp/terraform/configs/configschema"
	"github.com/jckuester/terradozer/internal"
	"github.com/jckuester/terradozer/pkg/provider"
	"github.com/zclconf/go-cty/cty"
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
	var resourcesAfterRead []resourceAfterRead

	resourcesAfterRead, err := importAndReadResource(r)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"id": r.id, "type": r.terraformType}).Debug(internal.Pad("failed to import resource; " +
			"trying to read resource without import"))

		resourcesAfterRead, err = readResource(r)
		if err != nil {
			return fmt.Errorf("failed to read current state of resource: %s", err)
		}
	}

	for _, rToRead := range resourcesAfterRead {
		resourceNotFound := rToRead.State.IsNull()
		if resourceNotFound {
			return fmt.Errorf("resource found in state doesn't exist anymore")
		}

		if dryRun {
			log.WithField("id", r.id).Warn(internal.Pad(r.terraformType))

			return nil
		}

		err = r.provider.DestroyResource(r.terraformType, rToRead.State)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"id": r.id, "type": r.terraformType}).Debug(internal.Pad("failed to delete resource"))

			return NewRetryDestroyError(err, r)
		}

		log.WithField("id", r.id).Error(internal.Pad(r.terraformType))
	}

	return nil
}

type resourceAfterRead struct {
	TerraformType string
	State         cty.Value
}

func importAndReadResource(r Resource) ([]resourceAfterRead, error) {
	var resourcesAfterRead []resourceAfterRead

	importedResources, err := r.provider.ImportResource(r.terraformType, r.id)
	if err != nil {
		return nil, err
	}

	for _, rImported := range importedResources {
		currentResourceState, err := r.provider.ReadResource(rImported.TypeName, rImported.State)
		if err != nil {
			return nil, err
		}

		resourcesAfterRead = append(resourcesAfterRead, resourceAfterRead{
			TerraformType: rImported.TypeName,
			State:         currentResourceState,
		})
	}

	return resourcesAfterRead, nil
}

func readResource(r Resource) ([]resourceAfterRead, error) {
	var resourcesAfterRead []resourceAfterRead

	schema, err := r.provider.GetSchemaForResource(r.terraformType)
	if err != nil {
		return nil, err
	}

	currentResourceState, err := r.provider.ReadResource(r.terraformType, emptyValueWitID(r.id, schema.Block))
	if err != nil {
		return nil, err
	}

	resourcesAfterRead = append(resourcesAfterRead, resourceAfterRead{
		TerraformType: r.terraformType,
		State:         currentResourceState,
	})

	return resourcesAfterRead, nil
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
