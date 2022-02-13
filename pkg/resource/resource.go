// Package resource manages the state update and deletion of Terraform resources.
package resource

import (
	"github.com/jckuester/awstools-lib/terraform"
	"github.com/jckuester/awstools-lib/terraform/provider"
	"github.com/zclconf/go-cty/cty"
)

// Resource represents a Terraform resource that can be destroyed.
type Resource struct {
	terraform.Resource
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
		terraform.Resource{
			Type:     terraformType,
			ID:       id,
			Provider: provider,
			Attrs:    attrs,
		},
	}
}

// NewWithState creates a destroyable Terraform resource.
//
// This constructor is used if a resource's internal state representation is known
// based on a present Terraform state file. A resource created with this constructor can be destroyed more reliable
// than with New(), which is used when the state is not known.
func NewWithState(terraformType, id string, provider *provider.TerraformProvider, state *cty.Value) *Resource {
	return &Resource{
		terraform.Resource{
			Type:     terraformType,
			ID:       id,
			Provider: provider,
			State:    state,
		},
	}
}

// Type returns the Terraform type of a resource.
func (r Resource) Type() string {
	return r.Resource.Type
}

// ID returns the Terraform ID of a resource.
func (r Resource) ID() string {
	return r.Resource.ID
}

// State returns the internal Terraform state representation of a resource.
func (r Resource) State() *cty.Value {
	return r.Resource.State
}
