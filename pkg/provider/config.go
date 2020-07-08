package provider

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
)

// config returns a configuration for the Terraform Provider given by name (e.g. "aws")
// with the given attributes set.
func config(name string, attrs map[string]cty.Value) (cty.Value, string, error) {
	switch name {
	case "aws":
		return awsProviderConfig(attrs), "2.68.0", nil
	default:
		return cty.NilVal, "", fmt.Errorf("provider config not found: %s", name)
	}
}

// awsProviderConfig returns a configuration for the Terraform AWS Provider with the given attributes set.
func awsProviderConfig(attrs map[string]cty.Value) cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"profile":                     attrs["profile"],
		"region":                      attrs["region"],
		"access_key":                  attrs["access_key"],
		"allowed_account_ids":         cty.UnknownVal(cty.DynamicPseudoType),
		"assume_role":                 cty.UnknownVal(cty.DynamicPseudoType),
		"endpoints":                   cty.UnknownVal(cty.DynamicPseudoType),
		"forbidden_account_ids":       cty.UnknownVal(cty.DynamicPseudoType),
		"insecure":                    cty.UnknownVal(cty.DynamicPseudoType),
		"max_retries":                 cty.UnknownVal(cty.DynamicPseudoType),
		"s3_force_path_style":         cty.UnknownVal(cty.DynamicPseudoType),
		"secret_key":                  attrs["secret_key"],
		"shared_credentials_file":     attrs["shared_credentials_file"],
		"skip_credentials_validation": cty.UnknownVal(cty.DynamicPseudoType),
		"skip_get_ec2_platforms":      cty.UnknownVal(cty.DynamicPseudoType),
		"skip_metadata_api_check":     cty.UnknownVal(cty.DynamicPseudoType),
		"skip_region_validation":      cty.UnknownVal(cty.DynamicPseudoType),
		"skip_requesting_account_id":  cty.UnknownVal(cty.DynamicPseudoType),
		"token":                       attrs["token"],
		"ignore_tag_prefixes":         cty.UnknownVal(cty.DynamicPseudoType),
		"ignore_tags":                 cty.UnknownVal(cty.DynamicPseudoType),
	})
}
