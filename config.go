package main

import (
	"os"

	"github.com/zclconf/go-cty/cty"
)

func awsProviderConfig() cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"profile":                     cty.StringVal(os.Getenv("AWS_PROFILE")),
		"region":                      cty.StringVal(os.Getenv("AWS_DEFAULT_REGION")),
		"access_key":                  cty.StringVal(os.Getenv("AWS_ACCESS_KEY_ID")),
		"allowed_account_ids":         cty.UnknownVal(cty.DynamicPseudoType),
		"assume_role":                 cty.UnknownVal(cty.DynamicPseudoType),
		"endpoints":                   cty.UnknownVal(cty.DynamicPseudoType),
		"forbidden_account_ids":       cty.UnknownVal(cty.DynamicPseudoType),
		"insecure":                    cty.UnknownVal(cty.DynamicPseudoType),
		"max_retries":                 cty.UnknownVal(cty.DynamicPseudoType),
		"s3_force_path_style":         cty.UnknownVal(cty.DynamicPseudoType),
		"secret_key":                  cty.StringVal(os.Getenv("AWS_SECRET_ACCESS_KEY")),
		"shared_credentials_file":     cty.StringVal(os.Getenv("AWS_SHARED_CREDENTIALS_FILE")),
		"skip_credentials_validation": cty.UnknownVal(cty.DynamicPseudoType),
		"skip_get_ec2_platforms":      cty.UnknownVal(cty.DynamicPseudoType),
		"skip_metadata_api_check":     cty.UnknownVal(cty.DynamicPseudoType),
		"skip_region_validation":      cty.UnknownVal(cty.DynamicPseudoType),
		"skip_requesting_account_id":  cty.UnknownVal(cty.DynamicPseudoType),
		"token":                       cty.UnknownVal(cty.DynamicPseudoType),
		"ignore_tag_prefixes":         cty.UnknownVal(cty.DynamicPseudoType),
		"ignore_tags":                 cty.UnknownVal(cty.DynamicPseudoType),
	})
}
