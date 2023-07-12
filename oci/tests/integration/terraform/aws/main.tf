provider "aws" {}

provider "aws" {
  alias  = "cross_region"
  region = var.cross_region
}

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

module "test_ecr_cross_reg" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "test-repo-${local.name}-cross-reg"
  tags = var.tags
  providers = {
    aws = aws.cross_region
  }
}

module "test_app_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "test-app-${local.name}"
  tags = var.tags
}
