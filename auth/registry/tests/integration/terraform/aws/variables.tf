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
