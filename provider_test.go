package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/hashicorp/terraform/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

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
			err := os.RemoveAll(".terradozer")
			require.NoError(t, err)

			p, err := installProvider(tc.providerName, tc.constraint, true)
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
			err := os.RemoveAll(".terradozer")
			require.NoError(t, err)

			p, err := installProvider(tc.providerName, tc.constraint, tc.useCache)
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

			p2, err := installProvider(tc.providerName, tc.constraint, tc.useCache)
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

	env := InitEnv(t)

	terraformDir := "./test-fixtures/single-resource"

	terraformOptions := &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    "terradozer",
		},
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcId := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcId, env.AWSRegion)

	providers, err := InitProviders([]string{"aws"})
	require.NoError(t, err)
	require.Len(t, providers, 1)

	importResp := providers["aws"].importResource("aws_vpc", actualVpcId)
	assert.NoError(t, importResp.Diagnostics.Err())
	assert.Len(t, importResp.ImportedResources, 1)

	assert.Equal(t, importResp.ImportedResources[0].TypeName, "aws_vpc")
	assert.Equal(t, importResp.ImportedResources[0].State, cty.ObjectVal(map[string]cty.Value{
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
		"id":                               cty.StringVal(actualVpcId),
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

	env := InitEnv(t)

	terraformDir := "./test-fixtures/single-resource"

	testName := "terradozer"

	terraformOptions := &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    testName,
		},
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcId := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcId, env.AWSRegion)

	p, err := InitProviders([]string{"aws"})
	require.NoError(t, err)
	require.Len(t, p, 1)

	readResp := p["aws"].readResource(providers.ImportedResource{
		TypeName: "aws_vpc",
		State: cty.ObjectVal(map[string]cty.Value{
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
			"id":                               cty.StringVal(actualVpcId),
			"instance_tenancy":                 cty.NullVal(cty.String),
			"ipv6_association_id":              cty.NullVal(cty.String),
			"ipv6_cidr_block":                  cty.NullVal(cty.String),
			"main_route_table_id":              cty.NullVal(cty.String),
			"owner_id":                         cty.NullVal(cty.String),
			"tags":                             cty.NullVal(cty.Map(cty.String)),
		}),
	})

	assert.NoError(t, readResp.Diagnostics.Err())

	assert.Equal(t, readResp.NewState.GetAttr("tags"),
		cty.MapVal(map[string]cty.Value{"Name": cty.StringVal(testName)}))

	assert.Equal(t, readResp.NewState.GetAttr("cidr_block"),
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
			actualProviders, err := InitProviders(tc.providerNames)

			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)

				for pName, p := range actualProviders {
					assert.NotNil(t, p.provider)
					assert.Contains(t, tc.expectedProviderNames, pName)
				}
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
