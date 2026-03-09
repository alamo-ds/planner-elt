terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
  }
  backend "azurerm" {
    resource_group_name  = "rg-terraform-state"
    storage_account_name = "tfstate32734"
    container_name       = "tfstate"
    key                  = "planner-elt.tfstate"
  }
}

provider "azurerm" {
  features {}
  skip_provider_registration = true
}
