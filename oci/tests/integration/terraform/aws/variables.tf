variable "rand" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "cross_region" {
  type        = string
  description = "different region for testing cross region resources"
}

variable "wi_k8s_sa_name" {
  type        = string
  default     = "test"
  description = "Name of kubernetes service account that can assume the IAM role (For workload identity)"
}

variable "wi_k8s_sa_ns" {
  type        = string
  default     = "default"
  description = "Namespace of kubernetes service account that can assume the IAM role (For workload identity)"
}

variable "enable_wi" {
  type        = bool
  default     = false
  description = "If set to true, will create IAM role and policy for workload identity"
}
