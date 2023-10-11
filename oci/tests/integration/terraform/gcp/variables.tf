variable "gcp_project_id" {
  type = string
}

variable "gcp_region" {
  type    = string
  default = "us-central1"
}

variable "gcp_zone" {
  type    = string
  default = "us-central1-c"
}

variable "gcr_region" {
  type    = string
  default = "" // Empty default to use gcr.io.
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "wi_k8s_sa_name" {
  type        = string
  default     = "test"
  description = "Name of kubernetes service account to be bound to GCP IAM service account (For workload identity)"
}

variable "wi_k8s_sa_ns" {
  type        = string
  default     = "default"
  description = "Namespace of kubernetes service account to be bound to GCP IAM service account (For workload identity)"
}

variable "enable_wi" {
  type        = bool
  default     = false
  description = "Enable workload identity on cluster and create a federated identity with service account"
}
