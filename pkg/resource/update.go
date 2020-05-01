package resource

import (
	"fmt"

	"github.com/apex/log"
	"github.com/hashicorp/terraform/configs/configschema"
	"github.com/jckuester/terradozer/internal"
	"github.com/zclconf/go-cty/cty"
)

// UpdateResources updates the state of a given list of resources in parallel.
// Only updated resources are returned which still exist remotely (e.g., in AWS).
func UpdateResources(resources []DestroyableResource, parallel int) []DestroyableResource {
	numOfResourcesToUpdate := len(resources)
	var updatedResources []DestroyableResource

	jobQueue := make(chan DestroyableResource, numOfResourcesToUpdate)

	workerResults := make(chan *DestroyableResource, numOfResourcesToUpdate)

	for workerID := 1; workerID <= parallel; workerID++ {
		go updateWorker(workerID, jobQueue, workerResults)
	}

	for _, r := range resources {
		jobQueue <- r
	}

	close(jobQueue)

	for i := 1; i <= numOfResourcesToUpdate; i++ {
		r := <-workerResults

		if r != nil {
			updatedResources = append(updatedResources, *r)
		}
	}

	return updatedResources
}

// updateWorker is a worker that updates the state of a resource.
func updateWorker(id int, resources <-chan DestroyableResource, result chan<- *DestroyableResource) {
	for r := range resources {
		err := r.UpdateState()
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"worker_id":   id,
				"type":        r.Type(),
				"resource_id": r.ID(),
			}).Debug(internal.Pad("failed to update resource"))

			result <- nil

			continue
		}

		resourceNotFound := r.State().IsNull()
		if resourceNotFound {
			log.WithFields(log.Fields{
				"worker_id": id,
				"type":      r.Type(),
				"id":        r.ID(),
			}).Debug(internal.Pad("resource doesn't exist anymore"))

			result <- nil

			continue
		}

		result <- &r
	}
}

// UpdateState updates the state of the resource (i.e., refreshes all its attributes).
// If the resource is already gone, the updated state will be nil.
func (r *Resource) UpdateState() error {
	if r.state != nil {
		// if the resource stores already a state representation, refresh that state
		result, err := r.provider.ReadResource(r.terraformType, *r.state)
		if err != nil {
			return err
		}

		r.state = &result

		return nil
	}

	result, err := r.importAndReadResource()
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"id": r.id, "type": r.terraformType}).Debug(internal.Pad("failed to import resource; " +
			"trying to read resource without import"))

		result, err = r.readResource()
		if err != nil {
			return fmt.Errorf("failed to read current state of resource: %s", err)
		}
	}

	r.state = &result

	return nil
}

func (r Resource) importAndReadResource() (cty.Value, error) {
	importedResources, err := r.provider.ImportResource(r.terraformType, r.id)
	if err != nil {
		return cty.NilVal, err
	}

	for _, rImported := range importedResources {
		currentResourceState, err := r.provider.ReadResource(rImported.TypeName, rImported.State)
		if err != nil {
			return cty.NilVal, err
		}

		if rImported.TypeName == r.terraformType {
			return currentResourceState, nil
		}

		log.WithError(err).WithFields(log.Fields{
			"type": rImported.TypeName,
		}).Debug(internal.Pad("found multiple resources during import"))
	}

	return cty.NilVal, fmt.Errorf("no resource found to be imported")
}

// readResource fetches the current state of a resource based on its ID attribute.
func (r Resource) readResource() (cty.Value, error) {
	schema, err := r.provider.GetSchemaForResource(r.terraformType)
	if err != nil {
		return cty.NilVal, err
	}

	currentResourceState, err := r.provider.ReadResource(r.terraformType, emptyValueWitID(r.id, schema.Block))
	if err != nil {
		return cty.NilVal, err
	}

	return currentResourceState, nil
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
