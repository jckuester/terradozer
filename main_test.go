package main

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform/addrs"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_deleteResources(t *testing.T) {
	tests := []struct {
		name                  string
		resources             []Resource
		providers             map[string]*TerraformProvider
		expectedDeletionCount int
		failedDeletions       map[string]int
	}{
		{
			name:                  "no resources to delete",
			expectedDeletionCount: 0,
		},
		{
			name: "single resources deleted",
			resources: []Resource{
				{
					Type:     "aws_vpc",
					Provider: "aws",
					Mode:     addrs.ManagedResourceMode,
					ID:       "vpc-1234",
				},
			},
			expectedDeletionCount: 1,
		},
		{
			name: "single resources failed to delete",
			resources: []Resource{
				{
					Type:     "aws_vpc",
					Provider: "aws",
					Mode:     addrs.ManagedResourceMode,
					ID:       "vpc-1234",
				},
			},
			failedDeletions: map[string]int{
				"aws_vpc": 1,
			},
			expectedDeletionCount: 0,
		},
		{
			name: "multiple resources deleted",
			resources: []Resource{
				{
					Type:     "aws_vpc",
					Provider: "aws",
					Mode:     addrs.ManagedResourceMode,
					ID:       "vpc-1234",
				},
				{
					Type:     "aws_subnet",
					Provider: "aws",
					Mode:     addrs.ManagedResourceMode,
					ID:       "subnet-1234",
				},
			},
			failedDeletions: map[string]int{
				"aws_vpc": 1,
			},
			expectedDeletionCount: 2,
		},
		{
			name: "multiple resources failed to delete",
			resources: []Resource{
				{
					Type:     "aws_vpc",
					Provider: "aws",
					Mode:     addrs.ManagedResourceMode,
					ID:       "vpc-1234",
				},
				{
					Type:     "aws_subnet",
					Provider: "aws",
					Mode:     addrs.ManagedResourceMode,
					ID:       "subnet-1234",
				},
				{
					Type:     "aws_foo",
					Provider: "aws",
					Mode:     addrs.ManagedResourceMode,
					ID:       "foo-1234",
				},
			},
			failedDeletions: map[string]int{
				"aws_vpc":    1,
				"aws_subnet": 2,
			},
			expectedDeletionCount: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)

			ctrl := gomock.NewController(t)

			m := NewMockResourceDeleter(ctrl)

			vpcDeletionFailed := m.EXPECT().Delete(Resource{
				Type:     "aws_vpc",
				Provider: "aws",
				Mode:     addrs.ManagedResourceMode,
				ID:       "vpc-1234",
			}, gomock.Any()).Return(false).MaxTimes(tc.failedDeletions["aws_vpc"])

			m.EXPECT().Delete(Resource{
				Type:     "aws_vpc",
				Provider: "aws",
				Mode:     addrs.ManagedResourceMode,
				ID:       "vpc-1234",
			}, gomock.Any()).Return(true).After(vpcDeletionFailed).AnyTimes()

			subnetDeletionFailed := m.EXPECT().Delete(Resource{
				Type:     "aws_subnet",
				Provider: "aws",
				Mode:     addrs.ManagedResourceMode,
				ID:       "subnet-1234",
			}, gomock.Any()).Return(false).MaxTimes(tc.failedDeletions["aws_subnet"])

			m.EXPECT().Delete(Resource{
				Type:     "aws_subnet",
				Provider: "aws",
				Mode:     addrs.ManagedResourceMode,
				ID:       "subnet-1234",
			}, gomock.Any()).Return(true).After(subnetDeletionFailed).AnyTimes()

			m.EXPECT().Delete(Resource{
				Type:     "aws_foo",
				Provider: "aws",
				Mode:     addrs.ManagedResourceMode,
				ID:       "foo-1234",
			}, gomock.Any()).Return(true).AnyTimes()

			actualDeletionCount := delete(tc.resources, map[string]ResourceDeleter{"aws": m})
			assert.Equal(t, actualDeletionCount, tc.expectedDeletionCount, "resources deleted")

			ctrl.Finish()
			logrus.SetLevel(logrus.InfoLevel)

		})
	}
}
