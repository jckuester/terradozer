package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

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

			m.EXPECT().ApplyResourceChange(gomock.Any()).Return(providers.ApplyResourceChangeResponse{}).Times(tc.expectedTimesCalled)

			p := &TerraformProvider{m}

			p.DeleteResource("test_type", "testID", providers.ReadResourceResponse{}, tc.dryRun)
		})
	}
}

func TestTerraformProvider_InstallProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	defer os.RemoveAll(".terradozer")

	tests := []struct {
		name             string
		providerName     string
		constraint       string
		expectedFile     string
		expectedChecksum string
	}{
		{
			name:             "install Terraform AWS Provider",
			providerName:     "aws",
			constraint:       "2.43.0",
			expectedFile:     ".terradozer/terraform-provider-aws_v2.43.0_x4",
			expectedChecksum: "d8a5e7969884c03cecbfd64fb3add8c542c918c5a8c259d1b31fadbbee284fb7",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.RemoveAll(".terradozer")

			p, err := InstallProvider(tc.providerName, tc.constraint, true)
			require.NoError(t, err)

			if tc.expectedFile != "" {
				f, err := os.Open(tc.expectedFile)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()

				assert.Equal(t, tc.providerName, p.Name)
				assert.Equal(t, tc.constraint, p.Version.MustParse().String())
				assert.Equal(t, tc.expectedChecksum, checksum(t, f))
			}
		})
	}
}

func TestTerraformProvider_InstallProvider_Cache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	defer os.RemoveAll(".terradozer")

	tests := []struct {
		name          string
		providerName  string
		constraint    string
		expectedFile  string
		expectedCache string
		useCache      bool
	}{
		{
			name:          "cache Terraform AWS Provider",
			providerName:  "aws",
			constraint:    "2.43.0",
			expectedFile:  ".terradozer/terraform-provider-aws_v2.43.0_x4",
			expectedCache: ".terradozer/cache/terraform-provider-aws_v2.43.0_x4",
			useCache:      true,
		},
		{
			name:         "don't cache Terraform AWS Provider",
			providerName: "aws",
			constraint:   "2.43.0",
			expectedFile: ".terradozer/terraform-provider-aws_v2.43.0_x4",
			useCache:     false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.RemoveAll(".terradozer")

			p, err := InstallProvider(tc.providerName, tc.constraint, tc.useCache)
			require.NoError(t, err)
			assert.Equal(t, tc.providerName, p.Name)
			assert.Equal(t, tc.constraint, p.Version.MustParse().String())

			_, err = ioutil.ReadFile(tc.expectedFile)
			if err != nil {
				t.Fatal(err)
			}

			modTime := modifiedTime(t, tc.expectedFile)

			if tc.expectedCache != "" {
				_, err = ioutil.ReadFile(tc.expectedCache)
				if err != nil {
					t.Fatal(err)
				}
			}

			p2, err := InstallProvider(tc.providerName, tc.constraint, tc.useCache)
			require.NoError(t, err)
			assert.Equal(t, tc.providerName, p2.Name)
			assert.Equal(t, tc.constraint, p2.Version.MustParse().String())

			modTimeAfterSecondInstall := modifiedTime(t, tc.expectedFile)

			if tc.useCache {
				assert.True(t, modTime.Equal(modTimeAfterSecondInstall))
			} else {
				assert.True(t, modTime.Before(modTimeAfterSecondInstall))
			}
		})
	}
}

func checksum(t *testing.T, file *os.File) string {
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		t.Fatal(err)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func modifiedTime(t *testing.T, filename string) time.Time {
	file, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}

	return file.ModTime()
}
