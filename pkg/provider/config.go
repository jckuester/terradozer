package provider

import (
	"fmt"
	"os"

	"github.com/zclconf/go-cty/cty"
)

// config returns a default configuration for the Terraform Provider given by name (e.g. "aws").
func config(name string) (cty.Value, string, error) {
	switch name {
	case "aws":
		return awsProviderConfig(), "2.43.0", nil
	case "google":
		return googleProviderConfig(), "3.11.0", nil
	default:
		return cty.NilVal, "", fmt.Errorf("provider config not found: %s", name)
	}
}

// awsProviderConfig returns a default configuration for the Terraform AWS Provider.
func awsProviderConfig() cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"access_key":                  cty.StringVal(os.Getenv("AWS_ACCESS_KEY_ID")),
		"allowed_account_ids":         cty.UnknownVal(cty.DynamicPseudoType),
		"assume_role":                 cty.UnknownVal(cty.DynamicPseudoType),
		"endpoints":                   cty.UnknownVal(cty.DynamicPseudoType),
		"forbidden_account_ids":       cty.UnknownVal(cty.DynamicPseudoType),
		"ignore_tag_prefixes":         cty.UnknownVal(cty.DynamicPseudoType),
		"ignore_tags":                 cty.UnknownVal(cty.DynamicPseudoType),
		"insecure":                    cty.UnknownVal(cty.DynamicPseudoType),
		"max_retries":                 cty.UnknownVal(cty.DynamicPseudoType),
		"profile":                     cty.StringVal(os.Getenv("AWS_PROFILE")),
		"region":                      cty.StringVal(os.Getenv("AWS_DEFAULT_REGION")),
		"s3_force_path_style":         cty.UnknownVal(cty.DynamicPseudoType),
		"secret_key":                  cty.StringVal(os.Getenv("AWS_SECRET_ACCESS_KEY")),
		"shared_credentials_file":     cty.StringVal(os.Getenv("AWS_SHARED_CREDENTIALS_FILE")),
		"skip_credentials_validation": cty.UnknownVal(cty.DynamicPseudoType),
		"skip_get_ec2_platforms":      cty.UnknownVal(cty.DynamicPseudoType),
		"skip_metadata_api_check":     cty.UnknownVal(cty.DynamicPseudoType),
		"skip_region_validation":      cty.UnknownVal(cty.DynamicPseudoType),
		"skip_requesting_account_id":  cty.UnknownVal(cty.DynamicPseudoType),
		"token":                       cty.StringVal(os.Getenv("AWS_SESSION_TOKEN")),
	})
}

// awsProviderConfig returns a default configuration for the Terraform AWS Provider.
func googleProviderConfig() cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"access_context_manager_custom_endpoint":   cty.UnknownVal(cty.DynamicPseudoType),
		"access_token":                             cty.UnknownVal(cty.DynamicPseudoType),
		"app_engine_custom_endpoint":               cty.UnknownVal(cty.DynamicPseudoType),
		"batching":                                 cty.UnknownVal(cty.DynamicPseudoType),
		"big_query_custom_endpoint":                cty.UnknownVal(cty.DynamicPseudoType),
		"bigquery_data_transfer_custom_endpoint":   cty.UnknownVal(cty.DynamicPseudoType),
		"bigtable_custom_endpoint":                 cty.UnknownVal(cty.DynamicPseudoType),
		"binary_authorization_custom_endpoint":     cty.UnknownVal(cty.DynamicPseudoType),
		"cloud_billing_custom_endpoint":            cty.UnknownVal(cty.DynamicPseudoType),
		"cloud_build_custom_endpoint":              cty.UnknownVal(cty.DynamicPseudoType),
		"cloud_functions_custom_endpoint":          cty.UnknownVal(cty.DynamicPseudoType),
		"cloud_iot_custom_endpoint":                cty.UnknownVal(cty.DynamicPseudoType),
		"cloud_run_custom_endpoint":                cty.UnknownVal(cty.DynamicPseudoType),
		"cloud_scheduler_custom_endpoint":          cty.UnknownVal(cty.DynamicPseudoType),
		"cloud_tasks_custom_endpoint":              cty.UnknownVal(cty.DynamicPseudoType),
		"composer_custom_endpoint":                 cty.UnknownVal(cty.DynamicPseudoType),
		"compute_beta_custom_endpoint":             cty.UnknownVal(cty.DynamicPseudoType),
		"compute_custom_endpoint":                  cty.UnknownVal(cty.DynamicPseudoType),
		"container_analysis_custom_endpoint":       cty.UnknownVal(cty.DynamicPseudoType),
		"container_beta_custom_endpoint":           cty.UnknownVal(cty.DynamicPseudoType),
		"container_custom_endpoint":                cty.UnknownVal(cty.DynamicPseudoType),
		"credentials":                              cty.StringVal(os.Getenv("GOOGLE_CREDENTIALS")),
		"dataflow_custom_endpoint":                 cty.UnknownVal(cty.DynamicPseudoType),
		"dataproc_beta_custom_endpoint":            cty.UnknownVal(cty.DynamicPseudoType),
		"dataproc_custom_endpoint":                 cty.UnknownVal(cty.DynamicPseudoType),
		"datastore_custom_endpoint":                cty.UnknownVal(cty.DynamicPseudoType),
		"deployment_manager_custom_endpoint":       cty.UnknownVal(cty.DynamicPseudoType),
		"dialogflow_custom_endpoint":               cty.UnknownVal(cty.DynamicPseudoType),
		"dns_beta_custom_endpoint":                 cty.UnknownVal(cty.DynamicPseudoType),
		"dns_custom_endpoint":                      cty.UnknownVal(cty.DynamicPseudoType),
		"filestore_custom_endpoint":                cty.UnknownVal(cty.DynamicPseudoType),
		"firestore_custom_endpoint":                cty.UnknownVal(cty.DynamicPseudoType),
		"iam_credentials_custom_endpoint":          cty.UnknownVal(cty.DynamicPseudoType),
		"iam_custom_endpoint":                      cty.UnknownVal(cty.DynamicPseudoType),
		"iap_custom_endpoint":                      cty.UnknownVal(cty.DynamicPseudoType),
		"identity_platform_custom_endpoint":        cty.UnknownVal(cty.DynamicPseudoType),
		"kms_custom_endpoint":                      cty.UnknownVal(cty.DynamicPseudoType),
		"logging_custom_endpoint":                  cty.UnknownVal(cty.DynamicPseudoType),
		"ml_engine_custom_endpoint":                cty.UnknownVal(cty.DynamicPseudoType),
		"monitoring_custom_endpoint":               cty.UnknownVal(cty.DynamicPseudoType),
		"project":                                  cty.StringVal(os.Getenv("GOOGLE_PROJECT")),
		"pubsub_custom_endpoint":                   cty.UnknownVal(cty.DynamicPseudoType),
		"redis_custom_endpoint":                    cty.UnknownVal(cty.DynamicPseudoType),
		"region":                                   cty.StringVal(os.Getenv("GOOGLE_REGION")),
		"request_timeout":                          cty.UnknownVal(cty.DynamicPseudoType),
		"resource_manager_custom_endpoint":         cty.UnknownVal(cty.DynamicPseudoType),
		"resource_manager_v2beta1_custom_endpoint": cty.UnknownVal(cty.DynamicPseudoType),
		"runtime_config_custom_endpoint":           cty.UnknownVal(cty.DynamicPseudoType),
		"runtimeconfig_custom_endpoint":            cty.UnknownVal(cty.DynamicPseudoType),
		"scopes":                                   cty.UnknownVal(cty.DynamicPseudoType),
		"security_center_custom_endpoint":          cty.UnknownVal(cty.DynamicPseudoType),
		"service_management_custom_endpoint":       cty.UnknownVal(cty.DynamicPseudoType),
		"service_networking_custom_endpoint":       cty.UnknownVal(cty.DynamicPseudoType),
		"service_usage_custom_endpoint":            cty.UnknownVal(cty.DynamicPseudoType),
		"source_repo_custom_endpoint":              cty.UnknownVal(cty.DynamicPseudoType),
		"spanner_custom_endpoint":                  cty.UnknownVal(cty.DynamicPseudoType),
		"sql_custom_endpoint":                      cty.UnknownVal(cty.DynamicPseudoType),
		"storage_custom_endpoint":                  cty.UnknownVal(cty.DynamicPseudoType),
		"storage_transfer_custom_endpoint":         cty.UnknownVal(cty.DynamicPseudoType),
		"tpu_custom_endpoint":                      cty.UnknownVal(cty.DynamicPseudoType),
		"user_project_override":                    cty.UnknownVal(cty.DynamicPseudoType),
		"vpc_access_custom_endpoint":               cty.UnknownVal(cty.DynamicPseudoType),
		"zone":                                     cty.StringVal(os.Getenv("GOOGLE_ZONE")),
	})
}
