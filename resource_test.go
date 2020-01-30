package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform/providers"
	"github.com/zclconf/go-cty/cty"
)

func TestResource_Delete(t *testing.T) {
	tests := []struct {
		name                string
		dryRun              bool
		expectedTimesCalled int
	}{
		{
			name:                "with dry-run flag",
			dryRun:              true,
			expectedTimesCalled: 0,
		},
		{
			name:                "without dry-run flag",
			dryRun:              false,
			expectedTimesCalled: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			m := NewMockProvider(ctrl)

			m.EXPECT().ImportResourceState(gomock.Any()).Return(providers.ImportResourceStateResponse{
				ImportedResources: []providers.ImportedResource{{TypeName: "foo"}}}).Times(tc.expectedTimesCalled)

			m.EXPECT().ReadResource(gomock.Any()).Return(providers.ReadResourceResponse{
				NewState: cty.ObjectVal(map[string]cty.Value{}),
			}).Times(tc.expectedTimesCalled)

			m.EXPECT().ApplyResourceChange(gomock.Any()).
				Return(providers.ApplyResourceChangeResponse{}).Times(tc.expectedTimesCalled)

			p := &TerraformProvider{m}

			r := Resource{
				TerraformType: "aws_vpc",
				id:            "testID",
				Provider:      p,
			}

			err := r.Delete(tc.dryRun)
			require.NoError(t, err)

			ctrl.Finish()
		})
	}
}

func Test_Delete(t *testing.T) {
	tests := []struct {
		name                  string
		expectedDeletionCount int
		failedDeletions       map[string]int
	}{
		{
			name:                  "no resources to delete",
			expectedDeletionCount: 0,
		},
		{
			name: "single resource deleted in first run",
			failedDeletions: map[string]int{
				"aws_vpc": 0,
			},
			expectedDeletionCount: 1,
		},
		{
			name: "single resources failed in first run",
			failedDeletions: map[string]int{
				"aws_vpc": 1,
			},
			expectedDeletionCount: 0,
		},
		{
			name: "multiple resources deleted during two runs",
			failedDeletions: map[string]int{
				"aws_vpc":    1,
				"aws_subnet": 0,
			},
			expectedDeletionCount: 2,
		},
		{
			name: "multiple resources deleted during three runs",
			failedDeletions: map[string]int{
				"aws_vpc":      2,
				"aws_subnet":   1,
				"aws_instance": 0,
			},
			expectedDeletionCount: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var resources []DeletableResource
			for rType, numOfFailedDeletions := range tc.failedDeletions {
				m := NewMockDeletableResource(ctrl)

				resFailedDeletions := m.EXPECT().Delete(gomock.Any()).
					Return(RetryableError).MaxTimes(numOfFailedDeletions)

				m.EXPECT().Delete(gomock.Any()).
					Return(nil).After(resFailedDeletions).AnyTimes()

				m.EXPECT().ID().Return("1234").AnyTimes()
				m.EXPECT().Type().Return(rType).AnyTimes()

				resources = append(resources, m)
			}

			actualDeletionCount := Delete(resources, 3)
			assert.Equal(t, tc.expectedDeletionCount, actualDeletionCount)

			ctrl.Finish()
		})
	}
}
