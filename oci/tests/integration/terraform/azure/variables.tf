variable "azure_location" {
  type    = string
  default = "eastus"
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "wi_k8s_sa_name" {
  type        = string
  default     = "test"
  description = "Name of kubernetes service account to establish federated identity with (For workload identity)"
}

variable "wi_k8s_sa_ns" {
  type        = string
  default     = "default"
  description = "Namespace of kubernetes service account to establish federated identity with (For workload identity)"
}

variable "enable_wi" {
  type        = bool
  default     = false
  description = "Enable workload identity on cluster and create federated identity"
}
