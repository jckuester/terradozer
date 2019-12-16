package main

import (
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/hashicorp/terraform/providers"
)

func TestTerraformProvider_DeleteResource_DryRun(t *testing.T) {
	tests := []struct {
		name                string
		dryRun              bool
		expectedTimesCalled int
	}{
		{
			name:                "with dry-run flag set",
			dryRun:              true,
			expectedTimesCalled: 0,
		},
		{
			name:                "without dry-run flag set",
			dryRun:              false,
			expectedTimesCalled: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			m := NewMockProvider(ctrl)

			m.EXPECT().ApplyResourceChange(gomock.Any()).Return(providers.ApplyResourceChangeResponse{}).Times(tc.expectedTimesCalled)

			p := &TerraformProvider{m}

			p.DeleteResource("test_type", "testID", providers.ReadResourceResponse{}, tc.dryRun)
		})
	}
}
