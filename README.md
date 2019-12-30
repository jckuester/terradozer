# Terradozer

**This project is WIP and not yet officially released.**

Have you also been facing the problem of trying to delete dangling resources of an (old or broken) Terraform configuration but
 [Terraform](https://www.terraform.io) didn't let you? Are you still having the Terraform state file(s) for this project?

If the answer to both question is YES, then I have good news
for you: Terradozer might be able help you. Terradozer force-destroys every Terraform
managed resource without the need of having the matching Terraform configuration (*.tf files) - just the Terraform state file is needed.

Not being able to destroy a Terraform project properly via `terraform destroy` can have several reasons.
Here are some that I came across:

* A team member applied some Terraform configuration to a testing account and later switched projects or
  left the company. Anyway, the Terraform code is left on the coworker's device (ie. is inaccessible) and the resources
  cannot be cleaned up by anyone else anymore. Even worse if the stale resources still add to the monthly bill.
  If the Terraform state is backed up in some shared storage (eg. AWS S3), which should be a default pattern, you can now
  go ahead, and run Terradozer against the state.
  
* The main branch of a co-developed Terraform configuration evolved and isn't backwards compatible anymore (due to broken or missing dependencies
  to other [Terraservices](https://www.hashicorp.com/resources/evolving-infrastructure-terraform-opencredo), upgrade to Terraform 12, renamed modules, you name it).
  At some point in the past, you applied the back then HEAD commit of the configuration to a testing account and now have checked out the latest version.
  Good luck trying to destroy the old state with the new code. Finding exactly the commit of matching configuration that was applied previously is also not easy.
  
* You probably have your own story to tell...

## Quick Start

To only check what will be deleted, execute:

    terradozer -dry
    
To delete all resources contained in a state file:

    terradozer -state <path/to/terraform.tfstate>
    
Note that you need to provide credentials for the cloud account you want to destroy resources in. In AWS, for example, via [environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html):

    AWS_PROFILE=<myaccount> AWS_DEFAULT_REGION=us-west-2 terradozer

## Supported Terraform Providers

In general, Terradozer can delete resources managed by any Terraform provider (because of the loose dependency via the [provider plugin 
architecture](https://github.com/hashicorp/go-plugin)). However, I still need to find a way to generically create a default configuration for any provider. So far,
I hard-coded a provider configuration the Terraform AWS provider (which is my use case).

Let me know if you need any other provider, and I will try to support it.

## Tests

Run unit tests

    make test
    
Run acceptance and integration tests

    AWS_PROFILE=<myaccount> AWS_DEFAULT_REGION=us-west-2 make test-all