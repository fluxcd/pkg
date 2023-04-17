provider "aws" {}

locals {
  name = "flux-test-${var.rand}"
}

module "eks" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/eks"

  name = local.name
  tags = var.tags
}

module "test_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "test-repo-${local.name}"
  tags = var.tags
}

module "test_app_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "test-app-${local.name}"
  tags = var.tags
}
