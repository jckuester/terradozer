package resource_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/apex/log"
	"github.com/golang/mock/gomock"
	"github.com/jckuester/terradozer/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func TestUpdateResources(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	tests := []struct {
		name                     string
		resourceToUpdate         []*resource.Resource
		expectedUpdatedResources []*resource.Resource
		parallel                 int
	}{
		{
			name: "empty list of resources to update",
		},
		{
			name: "single resource to update",
			resourceToUpdate: []*resource.Resource{
				resource.New("aws_vpc", "id-1234", nil),
			},
			expectedUpdatedResources: []*resource.Resource{
				resource.New("aws_vpc", "id-1234", nil),
			},
			parallel: 1,
		},
		{
			name: "multiple resources to update, one worker",
			resourceToUpdate: []*resource.Resource{
				resource.New("aws_vpc", "id-1234", nil),
				resource.New("aws_vpc", "id-3456", nil),
				resource.New("aws_subnet", "id-1234", nil),
				resource.New("aws_subnet", "id-3456", nil),
			},
			expectedUpdatedResources: []*resource.Resource{
				resource.New("aws_vpc", "id-1234", nil),
				resource.New("aws_vpc", "id-3456", nil),
				resource.New("aws_subnet", "id-1234", nil),
				resource.New("aws_subnet", "id-3456", nil)},
			parallel: 1,
		},
		{
			name: "multiple resources to update, some workers",
			resourceToUpdate: []*resource.Resource{
				resource.New("aws_vpc", "id-1234", nil),
				resource.New("aws_vpc", "id-3456", nil),
				resource.New("aws_subnet", "id-1234", nil),
				resource.New("aws_subnet", "id-3456", nil),
			},
			expectedUpdatedResources: []*resource.Resource{
				resource.New("aws_vpc", "id-1234", nil),
				resource.New("aws_vpc", "id-3456", nil),
				resource.New("aws_subnet", "id-1234", nil),
				resource.New("aws_subnet", "id-3456", nil)},
			parallel: 3,
		},
		{
			name: "multiple resources to update, more workers than resources",
			resourceToUpdate: []*resource.Resource{
				resource.New("aws_vpc", "id-1234", nil),
				resource.New("aws_subnet", "id-1234", nil),
			},
			expectedUpdatedResources: []*resource.Resource{
				resource.New("aws_vpc", "id-1234", nil),
				resource.New("aws_subnet", "id-3456", nil)},
			parallel: 10,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var resources []resource.DestroyableResource
			for _, r := range tc.resourceToUpdate {
				m := NewMockDestroyableResource(ctrl)

				m.EXPECT().UpdateState().Return(nil).Times(1)
				m.EXPECT().Destroy().Return(nil).Times(0)
				m.EXPECT().ID().Return(r.ID()).AnyTimes()
				m.EXPECT().Type().Return(r.Type()).AnyTimes()
				m.EXPECT().State().Return(&cty.DynamicVal).AnyTimes()

				resources = append(resources, m)
			}

			actualUpdatedResources := resource.UpdateResources(resources, tc.parallel)

			assert.Equal(t, len(tc.expectedUpdatedResources), len(actualUpdatedResources))

			ctrl.Finish()
		})
	}
}

func TestUpdateResources_UpdateError(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	ctrl := gomock.NewController(t)

	m := NewMockDestroyableResource(ctrl)

	m.EXPECT().UpdateState().Return(nil).Times(1)
	m.EXPECT().Destroy().Return(nil).Times(0)
	m.EXPECT().ID().Return("id-1234").AnyTimes()
	m.EXPECT().Type().Return("aws_vpc").AnyTimes()

	mUpdateError := NewMockDestroyableResource(ctrl)

	mUpdateError.EXPECT().UpdateState().Return(fmt.Errorf("some error")).Times(1)
	mUpdateError.EXPECT().Destroy().Return(nil).Times(0)
	mUpdateError.EXPECT().ID().Return("id-3456").AnyTimes()
	mUpdateError.EXPECT().Type().Return("aws_subnet").AnyTimes()

	actualUpdatedResources := resource.UpdateResources([]resource.DestroyableResource{m, mUpdateError}, 3)
	require.Len(t, actualUpdatedResources, 1)

	assert.Equal(t, "aws_vpc", actualUpdatedResources[0].Type())
	assert.Equal(t, "id-1234", actualUpdatedResources[0].ID())

	ctrl.Finish()
}

func TestUpdateResources_StateIsNil(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	ctrl := gomock.NewController(t)

	m := NewMockDestroyableResource(ctrl)

	m.EXPECT().UpdateState().Return(nil).Times(1)
	m.EXPECT().Destroy().Return(nil).Times(0)
	m.EXPECT().ID().Return("id-1234").AnyTimes()
	m.EXPECT().Type().Return("aws_vpc").AnyTimes()
	m.EXPECT().State().Return(&cty.DynamicVal).AnyTimes()

	mNilState := NewMockDestroyableResource(ctrl)

	mNilState.EXPECT().UpdateState().Return(nil).Times(1)
	mNilState.EXPECT().Destroy().Return(nil).Times(0)
	mNilState.EXPECT().ID().Return("id-3456").AnyTimes()
	mNilState.EXPECT().Type().Return("aws_subnet").AnyTimes()
	mNilState.EXPECT().State().Return(&cty.NilVal).AnyTimes()

	actualUpdatedResources := resource.UpdateResources([]resource.DestroyableResource{m, mNilState}, 2)
	require.Len(t, actualUpdatedResources, 1)

	assert.Equal(t, "aws_vpc", actualUpdatedResources[0].Type())
	assert.Equal(t, "id-1234", actualUpdatedResources[0].ID())

	ctrl.Finish()
}
