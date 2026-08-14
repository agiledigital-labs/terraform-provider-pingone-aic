terraform {
  required_providers {
    pingoneaic = {
      source  = "agiledigital-labs/pingone-aic"
      version = "0.1.0"
    }
  }
}

provider "pingoneaic" {
  # tenant_url / credentials: PINGONEAIC_* environment variables.
  resource_prefix = "Terraform_"
}
