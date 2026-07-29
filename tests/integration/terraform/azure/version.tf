terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">=3.20.0, <5.0.0"
    }
    azuredevops = {
      source = "microsoft/azuredevops"
    }
  }
}
