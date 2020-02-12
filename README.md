<p align="center">
  <img alt="TerraDozer Logo" src="https://github.com/jckuester/terradozer/blob/master/img/logo.png" height="180" />
  <h3 align="center">TerraDozer</h3>
  <p align="center">Terraform destroy without configuration files.</p>
</p>

---
[![Release](https://img.shields.io/github/release/terradozer/terradozer.svg?style=for-the-badge)](https://github.com/jckuester/terradozer/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge)](/LICENSE.md)
[![Travis](https://img.shields.io/travis/jckuester/terradozer/master.svg?style=for-the-badge)](https://travis-ci.org/jckuester/terradozer)
[![Codecov branch](https://img.shields.io/codecov/c/github/jckuester/terradozer/master.svg?style=for-the-badge)](https://codecov.io/gh/jckuester/terradozer)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=for-the-badge)](http://godoc.org/github.com/jckuester/terradozer)

Occasionally, `terraform destroy` behaves like a stubborn donkey and doesn't want the way you want. What **YOU WANT** is
probably simply to destroy a Terraform configuration. What **TERRAFORM WANTS** is all the configuration files to exist,
be valid, dependencies in remote states be accessible, ... everything still be in perfect shape--yes, even if you don't want to
apply, but just destroy.

Terraform may have [valid reasons](https://github.com/hashicorp/terraform/issues/18994#issuecomment-427082789) to
be so pedantic, but wouldn't it be convenient to have a way to bulldoze all resources managed by Terraform configuration no matter what,
simply based on its state file? That's what Terradozer does for you.

Happy (terra)dozing!

## Example

TODO

## Features

* Terradozer first scans a given Terraform state file (read-only) to find all resources (excluding data sources),
then downloads the necessary Terraform Provider Plugins to call the destroy function of the providers' CRUD API via GRPC
(e.g., calling the Terraform AWS Provider to destroy a `aws_instance` resource)---this can be done without needing the configuration files.
 * Terradozer cannot infer the dependency graph from the state, as this information is stored in the configuration files.
 However, Terradozer retries smartly until all resources are destroyed.
* Terradozer shows all resources first and asks to confirm with `yes` before proceeding wit a destroy (same as Terraform does)
* Terradozer has a force mode to use Terradozer in an automated way, for example, in your CI pipeline

## Installation

It's recommended to install a specific version of terradozer available on the
[releases page](https://github.com/jckuester/terradozer/releases).

Here is the recommended way to install terradozer v0.1.0:

```bash
# install it into ./bin/
curl -sSfL https://raw.githubusercontent.com/jckuester/terradozer/master/install.sh | sh -s v0.1.0
```

## Usage

To delete all resources in a Terraform state file:

    AWS_PROFILE=<myaccount> AWS_DEFAULT_REGION=<myregion> terradozer -state <path/to/terraform.tfstate>

Note that you need to provide credentials for the AWS account you want to destroy resources in
 via [environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html). The region
 is needed too, as the region information is not stored in the state. Having multiple regions in a state file is not
 yet supported.
 
To only see what would be deleted, add the dry run flag above `-dry`.

## Limitations

*By making use of Terraform's  [Provider Plugin Architecture](https://github.com/hashicorp/go-plugin), Terradozer
would be able destroy any resource in a Terraform state file. However, the Terraform Provider configuration is not stored in
 the state and therefore I still need to investigate a way to generically provide a default configuration for any provider.
Until now, **Terraform AWS provide is the only supported provider**, for which I statically added a
[default config](https://github.com/jckuester/terradozer/blob/master/pkg/provider/config.go#L21).
If you need any other provider, let me know, and I will help supporting it.

* Terradozer cannot know the region (as it is part of the Terraform configuration) in which your resources live. So, you need
to provide the region via `AWS_DEFAULT_REGION=<myregion>` when running Terradozer.

## Tests

This section is only relevant if you want to contribute to Terradozer and therefore run the tests. Terradozer has
acceptance tests, integration tests checking against changes of behaviour in the Terraform Provider API, and of course
 unit tests.

Run unit tests

    make test
    
Run acceptance and integration tests

    AWS_PROFILE=<myaccount> AWS_DEFAULT_REGION=<myregion> make test-all
