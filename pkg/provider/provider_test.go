package provider_test

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/jckuester/terradozer/pkg/provider"
	"github.com/jckuester/terradozer/test"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestInstall(t *testing.T) {
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
			err := os.RemoveAll(".terradozer")
			require.NoError(t, err)

			p, err := provider.Install(tc.providerName, tc.constraint, true)
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

func TestInstall_Cache(t *testing.T) {
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
			err := os.RemoveAll(".terradozer")
			require.NoError(t, err)

			p, err := provider.Install(tc.providerName, tc.constraint, tc.useCache)
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

			p2, err := provider.Install(tc.providerName, tc.constraint, tc.useCache)
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

func TestTerraformProvider_ImportResource(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	env := test.InitEnv(t)

	terraformDir := "../../test/test-fixtures/single-resource/aws-vpc"

	terraformOptions := test.GetTerraformOptions(terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion)

	provider, err := provider.Init("aws", 15)
	require.NoError(t, err)

	importedResources, err := provider.ImportResource("aws_vpc", actualVpcID)
	require.NoError(t, err)
	assert.Len(t, importedResources, 1)

	assert.Equal(t, importedResources[0].TypeName, "aws_vpc")
	assert.Equal(t, importedResources[0].State, cty.ObjectVal(map[string]cty.Value{
		"arn":                              cty.NullVal(cty.String),
		"assign_generated_ipv6_cidr_block": cty.False,
		"cidr_block":                       cty.NullVal(cty.String),
		"default_network_acl_id":           cty.NullVal(cty.String),
		"default_route_table_id":           cty.NullVal(cty.String),
		"default_security_group_id":        cty.NullVal(cty.String),
		"dhcp_options_id":                  cty.NullVal(cty.String),
		"enable_classiclink":               cty.NullVal(cty.Bool),
		"enable_classiclink_dns_support":   cty.NullVal(cty.Bool),
		"enable_dns_hostnames":             cty.NullVal(cty.Bool),
		"enable_dns_support":               cty.NullVal(cty.Bool),
		"id":                               cty.StringVal(actualVpcID),
		"instance_tenancy":                 cty.NullVal(cty.String),
		"ipv6_association_id":              cty.NullVal(cty.String),
		"ipv6_cidr_block":                  cty.NullVal(cty.String),
		"main_route_table_id":              cty.NullVal(cty.String),
		"owner_id":                         cty.NullVal(cty.String),
		"tags":                             cty.NullVal(cty.Map(cty.String)),
	}))
}

func TestTerraformProvider_ReadResource(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	env := test.InitEnv(t)

	terraformDir := "../../test/test-fixtures/single-resource/aws-vpc"

	terraformOptions := test.GetTerraformOptions(terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion)

	p, err := provider.Init("aws", 15)
	require.NoError(t, err)

	currentResourceState, err := p.ReadResource("aws_vpc",
		cty.ObjectVal(map[string]cty.Value{
			"arn":                              cty.NullVal(cty.String),
			"assign_generated_ipv6_cidr_block": cty.False,
			"cidr_block":                       cty.NullVal(cty.String),
			"default_network_acl_id":           cty.NullVal(cty.String),
			"default_route_table_id":           cty.NullVal(cty.String),
			"default_security_group_id":        cty.NullVal(cty.String),
			"dhcp_options_id":                  cty.NullVal(cty.String),
			"enable_classiclink":               cty.NullVal(cty.Bool),
			"enable_classiclink_dns_support":   cty.NullVal(cty.Bool),
			"enable_dns_hostnames":             cty.NullVal(cty.Bool),
			"enable_dns_support":               cty.NullVal(cty.Bool),
			"id":                               cty.StringVal(actualVpcID),
			"instance_tenancy":                 cty.NullVal(cty.String),
			"ipv6_association_id":              cty.NullVal(cty.String),
			"ipv6_cidr_block":                  cty.NullVal(cty.String),
			"main_route_table_id":              cty.NullVal(cty.String),
			"owner_id":                         cty.NullVal(cty.String),
			"tags":                             cty.NullVal(cty.Map(cty.String)),
		}))

	require.NoError(t, err)

	assert.Equal(t, currentResourceState.GetAttr("tags"),
		cty.MapVal(map[string]cty.Value{"Name": cty.StringVal(terraformOptions.Vars["name"].(string))}))

	assert.Equal(t, currentResourceState.GetAttr("cidr_block"),
		cty.StringVal("10.0.0.0/16"))
}

func TestInitProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	tests := []struct {
		name                  string
		providerNames         []string
		expectedProviderNames []string
		expectedErrMsg        string
	}{
		{
			name: "empty provider list input",
		},
		{
			name:                  "single provider",
			providerNames:         []string{"aws"},
			expectedProviderNames: []string{"aws"},
		},
		{
			name:          "unknown provider",
			providerNames: []string{"foo"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualProviders, err := provider.InitProviders(tc.providerNames, 15)

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)

				for pName, p := range actualProviders {
					assert.NotNil(t, p)
					assert.Contains(t, tc.expectedProviderNames, pName)
				}
			}
		})
	}
}

func checksum(t *testing.T, file io.Reader) string {
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
