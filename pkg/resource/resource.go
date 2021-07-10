// Package resource manages the state update and deletion of Terraform resources.
package resource

import (
	"github.com/jckuester/awstools-lib/terraform/provider"
	"github.com/zclconf/go-cty/cty"
)

// Resource represents a Terraform resource that can be destroyed.
type Resource struct {
	// terraformType is the Terraform type of a resource
	terraformType string
	// id is used by the provider to import and delete the resource
	id string
	// provider is able to delete a resource
	provider *provider.TerraformProvider
	// internal Terraform state of the resource
	state *cty.Value
	attrs map[string]cty.Value
}

// New creates a destroyable Terraform resource.
//
// To destroy a resource, its Terraform Type and ID (both together uniquely identify a resource),
// plus a provider that will handle the destroy is needed. Note: if a resource's internal state
// representation is known, use NewWithState() instead.
//
// For some resources, additionally to the ID a list of attributes needs to be populated to destroy it.
func New(terraformType, id string, attrs map[string]cty.Value, provider *provider.TerraformProvider) *Resource {
	return &Resource{
		terraformType: terraformType,
		id:            id,
		provider:      provider,
		attrs:         attrs,
	}
}

// NewWithState creates a destroyable Terraform resource.
//
// This constructor is used if a resource's internal state representation is known
// based on a present Terraform state file. A resource created with this constructor can be destroyed more reliable
// than with New(), which is used when the state is not known.
func NewWithState(terraformType, id string, provider *provider.TerraformProvider, state *cty.Value) *Resource {
	return &Resource{
		terraformType: terraformType,
		id:            id,
		provider:      provider,
		state:         state,
	}
}

// Type returns the Terraform type of a resource.
func (r Resource) Type() string {
	return r.terraformType
}

// ID returns the Terraform ID of a resource.
func (r Resource) ID() string {
	return r.id
}

// State returns the internal Terraform state representation of a resource.
func (r Resource) State() *cty.Value {
	return r.state
}
