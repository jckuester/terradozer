package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/terraform/states"

	"github.com/stretchr/testify/require"
)

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
			pathToState: "test-fixtures/tfstates/terraform-0.11.14-stateversion-3.tfstate",
			expectedID:  "vpc-003104c0d87e7a9f4",
		},
		{
			name:        "state version 4",
			pathToState: "test-fixtures/tfstates/terraform-0.12.9-stateversion-4.tfstate",
			expectedID:  "vpc-034efaa028f36357d",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var resInstance *states.ResourceInstance

			if tc.pathToState != "" {
				state, err := getState(tc.pathToState)
				require.NoError(t, err)

				resInstances, diagnostics := lookupAllResourceInstanceAddrs(state)
				require.NoError(t, diagnostics.Err())

				require.Len(t, resInstances, 1)

				resInstance = state.ResourceInstance(resInstances[0])
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
