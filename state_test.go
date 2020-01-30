package main

import (
	"testing"

	"github.com/hashicorp/terraform/builtin/providers/terraform"

	"github.com/hashicorp/terraform/states"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func Test_NewState(t *testing.T) {
	tests := []struct {
		name           string
		pathToState    string
		expectedErrMsg string
	}{
		{
			name:        "state version 3",
			pathToState: "test-fixtures/tfstates/version3.tfstate",
		},
		{
			name:        "state version 4",
			pathToState: "test-fixtures/tfstates/version4.tfstate",
		},
		{
			name:           "broken state file with malformed JSON",
			pathToState:    "test-fixtures/tfstates/malformed.tfstate",
			expectedErrMsg: "failed reading test-fixtures/tfstates/malformed.tfstate as a statefile",
		},
		{
			name:           "wrong path to state",
			pathToState:    "not/exist/terraform.tfstate",
			expectedErrMsg: "failed loading statefile: open not/exist/terraform.tfstate: no such file or directory",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualState, err := NewState(tc.pathToState)

			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, actualState)
			}
		})
	}
}

func TestState_ProviderNames(t *testing.T) {
	tests := []struct {
		name                  string
		pathToState           string
		expectedProviderNames []string
	}{
		{
			name:                  "state version 3",
			pathToState:           "test-fixtures/tfstates/version3.tfstate",
			expectedProviderNames: []string{"aws"},
		},
		{
			name:                  "state version 4",
			pathToState:           "test-fixtures/tfstates/version4.tfstate",
			expectedProviderNames: []string{"aws"},
		},
		{
			name:        "empty state",
			pathToState: "test-fixtures/tfstates/empty.tfstate",
		},
		{
			name:                  "multiple providers",
			pathToState:           "test-fixtures/tfstates/multiple-providers.tfstate",
			expectedProviderNames: []string{"aws", "random"},
		},
		{
			name:                  "duplicate provider",
			pathToState:           "test-fixtures/tfstates/duplicate-provider.tfstate",
			expectedProviderNames: []string{"aws"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, err := NewState(tc.pathToState)
			require.NoError(t, err)

			actualProviderNames := state.ProviderNames()

			assert.Equal(t, tc.expectedProviderNames, actualProviderNames)
		})
	}
}

func Test_Resources(t *testing.T) {
	awsDummyProvider := terraform.NewProvider()
	awsDummyProvider.Schema.Version = 1

	randomDummyProvider := terraform.NewProvider()
	randomDummyProvider.Schema.Version = 2

	tests := []struct {
		name              string
		pathToState       string
		expectedResources []DeletableResource
		expectedErrMsg    string
		providers         map[string]*TerraformProvider
	}{
		{
			name:        "empty provider list",
			pathToState: "test-fixtures/tfstates/version3.tfstate",
		},
		{
			name:        "single AWS resource",
			pathToState: "test-fixtures/tfstates/version3.tfstate",
			providers: map[string]*TerraformProvider{
				"aws": {awsDummyProvider},
			},
			expectedResources: []DeletableResource{
				Resource{
					TerraformType: "aws_vpc",
					Provider:      &TerraformProvider{awsDummyProvider},
					id:            "vpc-003104c0d87e7a9f4",
				},
			},
		},
		{
			name:        "resources from multiple providers",
			pathToState: "test-fixtures/tfstates/multiple-providers.tfstate",
			providers: map[string]*TerraformProvider{
				"aws":    {awsDummyProvider},
				"random": {randomDummyProvider},
			},
			expectedResources: []DeletableResource{
				Resource{
					TerraformType: "aws_vpc",
					Provider:      &TerraformProvider{awsDummyProvider},

					id: "vpc-039b3d3fb4ffcf0ea",
				},
				Resource{
					TerraformType: "random_integer",
					Provider:      &TerraformProvider{randomDummyProvider},
					id:            "12375",
				},
			},
		},
		{
			name:        "data source",
			pathToState: "test-fixtures/tfstates/datasource.tfstate",
			providers:   map[string]*TerraformProvider{"aws": nil},
		},
		{
			name:        "state with missing resource ID",
			pathToState: "test-fixtures/tfstates/missing-id.tfstate",
			providers:   map[string]*TerraformProvider{"aws": nil},
			expectedResources: []DeletableResource{
				Resource{
					TerraformType: "aws_vpc",
					id:            "",
				},
			},
		},
		{
			name:        "empty state",
			pathToState: "test-fixtures/tfstates/empty.tfstate",
			providers:   map[string]*TerraformProvider{"aws": nil},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pathToState != "" {
				s, err := NewState(tc.pathToState)
				require.NoError(t, err)

				actualResources, err := s.Resources(tc.providers)
				require.NoError(t, err)

				if tc.expectedErrMsg != "" {
					require.Error(t, err)
					assert.EqualError(t, err, tc.expectedErrMsg)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedResources, actualResources)
				}
			}
		})
	}
}

func Test_getResourceID(t *testing.T) {
	tests := []struct {
		name           string
		pathToState    string
		expectedID     string
		expectedErrMsg string
	}{
		{
			name:           "resource instance is nil",
			expectedErrMsg: "resource instance has no current object",
		},
		{
			name:        "state version 3",
			pathToState: "test-fixtures/tfstates/version3.tfstate",
			expectedID:  "vpc-003104c0d87e7a9f4",
		},
		{
			name:        "state version 4",
			pathToState: "test-fixtures/tfstates/version4.tfstate",
			expectedID:  "vpc-034efaa028f36357d",
		},
		{
			name:        "state with missing resource ID",
			pathToState: "test-fixtures/tfstates/missing-id.tfstate",
			expectedID:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var resInstance *states.ResourceInstance

			if tc.pathToState != "" {
				s, err := NewState(tc.pathToState)
				require.NoError(t, err)

				resInstances := lookupAllResourceInstanceAddrs(s.state)
				require.Len(t, resInstances, 1)

				resInstance = s.state.ResourceInstance(resInstances[0])
			}

			actualID, err := getResourceID(resInstance)

			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedID, actualID)
			}
		})
	}
}
