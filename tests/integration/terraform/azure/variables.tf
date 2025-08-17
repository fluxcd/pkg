variable "azure_location" {
  type    = string
  default = "eastus"
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "wi_k8s_sa_ns" {
  type        = string
  description = "Namespace of kubernetes service account to establish federated identity with (For workload identity)"
}

variable "wi_k8s_sa_name" {
  type        = string
  description = "Name of kubernetes service account to establish federated identity with (For workload identity)"
}

variable "enable_wi" {
  type        = bool
  default     = false
  description = "Enable workload identity on cluster and create federated identity"
}

variable "azuredevops_org" {
  type        = string
  description = "Azure Devops organization to create project and git repository"
  default     = ""
}

variable "azuredevops_pat" {
  type        = string
  description = "Personal access token to create project and repository in azure devops"
  default     = ""
}
