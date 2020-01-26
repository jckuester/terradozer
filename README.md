# Terradozer

Sometimes, `terraform destroy` behaves like a diva - it just wouldn't let you the way you want. 

**YOU WANT** probably simply destroy.

**IT WANTS** you all the configuration files to still exist, be valid, dependencies still exist, ... 
the whole project be still in tact.

C'mon, we want to destroy not apply.
 
Wouldn't it be great to bulldoze a Terraform project without all the fuzz - just based on still existing state file(s)?
 
That's what Terradozer is built for!

## Quick Start

To only check what will be deleted, execute:

    terradozer -dry

To delete all resources contained in a state file:

    terradozer -state <path/to/terraform.tfstate>

Note that you need to provide credentials for the cloud account you want to destroy resources in. In AWS, for example, via [environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html):

    AWS_PROFILE=<myaccount> AWS_DEFAULT_REGION=us-west-2 terradozer

## Supported Terraform Providers

Thanks to the loose coupling of the [plugin architecture](https://github.com/hashicorp/go-plugin),
Terradozer can delete resources managed by any Terraform provider.

However, I still need to investigate a way to generically provide a default configuration for any provider.
Until then, **Terraform AWS provide is the only supported provider** (which is my use case), for which I statically added a default config.

Let me know if you need any other provider, and I will try to support it.

## Tests

This section is only relevant if you want to develop Terradozer. Terradozer is tested on many layers,
there are acceptance tests, integration tests checking against changes of behaviour in the Terraform provider API, and of course
 unit tests.

Run unit tests

    make test
    
Run acceptance and integration tests

    AWS_PROFILE=<myaccount> AWS_DEFAULT_REGION=us-west-2 make test-all