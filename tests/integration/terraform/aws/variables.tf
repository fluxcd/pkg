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

variable "wi_k8s_sa_ns" {
  type        = string
  description = "Namespace of kubernetes service account that can assume the IAM role (For workload identity)"
}

variable "wi_k8s_sa_name" {
  type        = string
  description = "Name of kubernetes service account that can assume the IAM role (For workload identity)"
}

variable "wi_k8s_sa_name_controller_irsa" {
  type        = string
  description = "Name of controller SA with IRSA for impersonation testing"
}

variable "wi_k8s_sa_name_controller_pod_identity" {
  type        = string
  description = "Name of controller SA with Pod Identity for impersonation testing"
}

variable "wi_k8s_sa_name_assume_role" {
  type        = string
  description = "Name of SA with IRSA that will also AssumeRole into the target (object-level impersonation)"
}

variable "enable_wi" {
  type        = bool
  default     = false
  description = "If set to true, will create IAM role and policy for workload identity"
}
