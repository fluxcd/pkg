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

  name = local.name
  tags = var.tags
}

resource "aws_ecrpublic_repository" "test_ecr_public" {
  repository_name = local.name
  tags            = var.tags
  force_destroy   = true
}

module "test_ecr_cross_reg" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "${local.name}-cross-reg"
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

resource "aws_iam_role" "assume_role" {
  count       = var.enable_wi ? 1 : 0
  name        = local.name
  description = "IAM role used for testing Workload integration for OCI repositories in Flux"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = module.eks.cluster_oidc_arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "${replace(module.eks.cluster_oidc_url, "https://", "")}:aud" = "sts.amazonaws.com"
          "${replace(module.eks.cluster_oidc_url, "https://", "")}:sub" = [
            "system:serviceaccount:${var.wi_k8s_sa_ns}:${var.wi_k8s_sa_name}",
            "system:serviceaccount:${var.wi_k8s_sa_ns}:${var.wi_k8s_sa_name_assume_role}",
          ]
        }
      }
    }]
  })

  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "wi_read_ecr" {
  count = var.enable_wi ? 1 : 0

  role       = aws_iam_role.assume_role[0].name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "wi_role_policy" {
  count = var.enable_wi ? 1 : 0

  role       = aws_iam_role.assume_role[0].name
  policy_arn = aws_iam_policy.wi_role_policy[0].arn
}

resource "aws_iam_policy" "wi_role_policy" {
  count = var.enable_wi ? 1 : 0

  name = "${local.name}-wi-policy"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr-public:GetAuthorizationToken",
          "sts:GetServiceBearerToken",
          "eks:DescribeCluster",
        ]
        Resource = "*"
      },
    ],
  })
}

resource "aws_eks_access_entry" "wi_access_entry" {
  count = var.enable_wi ? 1 : 0

  depends_on = [ module.eks ]

  cluster_name  = local.name
  principal_arn = aws_iam_role.assume_role[0].arn
  user_name     = aws_iam_role.assume_role[0].arn
}

# --- Impersonation (AssumeRole) testing resources ---

# Target IAM role that will be assumed via impersonation.
resource "aws_iam_role" "assume_role_target" {
  count       = var.enable_wi ? 1 : 0
  name        = "${local.name}-target"
  description = "Target IAM role for AssumeRole impersonation testing"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Principal = { AWS = [
          aws_iam_role.assume_role[0].arn,
          aws_iam_role.controller_irsa[0].arn,
          aws_iam_role.controller_pod_identity[0].arn,
        ]}
        Action = ["sts:AssumeRole", "sts:TagSession"]
      }
    ]
  })
  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "target_read_ecr" {
  count = var.enable_wi ? 1 : 0

  role       = aws_iam_role.assume_role_target[0].name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "target_role_policy" {
  count = var.enable_wi ? 1 : 0

  role       = aws_iam_role.assume_role_target[0].name
  policy_arn = aws_iam_policy.wi_role_policy[0].arn
}

resource "aws_eks_access_entry" "target_access_entry" {
  count = var.enable_wi ? 1 : 0

  depends_on = [module.eks]

  cluster_name  = local.name
  principal_arn = aws_iam_role.assume_role_target[0].arn
  user_name     = aws_iam_role.assume_role_target[0].arn
}

# Managed policy granting sts:AssumeRole on the target role.
# Shared by all source roles that need to assume the target.
resource "aws_iam_policy" "assume_target_policy" {
  count = var.enable_wi ? 1 : 0

  name = "${local.name}-assume-target"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "sts:AssumeRole"
      Resource = aws_iam_role.assume_role_target[0].arn
    }]
  })
}

# Allow the existing IRSA role to AssumeRole into the target.
resource "aws_iam_role_policy_attachment" "wi_assume_target" {
  count = var.enable_wi ? 1 : 0

  role       = aws_iam_role.assume_role[0].name
  policy_arn = aws_iam_policy.assume_target_policy[0].arn
}

# Controller IRSA role — trusted by OIDC, only has sts:AssumeRole on target.
resource "aws_iam_role" "controller_irsa" {
  count       = var.enable_wi ? 1 : 0
  name        = "${local.name}-ctrl-irsa"
  description = "Controller IRSA role for AssumeRole impersonation testing"
  assume_role_policy = templatefile("oidc_assume_role_policy.json", {
    OIDC_ARN  = module.eks.cluster_oidc_arn,
    OIDC_URL  = replace(module.eks.cluster_oidc_url, "https://", ""),
    NAMESPACE = var.wi_k8s_sa_ns,
    SA_NAME   = var.wi_k8s_sa_name_controller_irsa
  })
  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "controller_irsa_assume" {
  count = var.enable_wi ? 1 : 0

  role       = aws_iam_role.controller_irsa[0].name
  policy_arn = aws_iam_policy.assume_target_policy[0].arn
}

# Controller Pod Identity role — trusted by EKS Pod Identity, only has sts:AssumeRole on target.
resource "aws_iam_role" "controller_pod_identity" {
  count       = var.enable_wi ? 1 : 0
  name        = "${local.name}-ctrl-podid"
  description = "Controller Pod Identity role for AssumeRole impersonation testing"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "pods.eks.amazonaws.com" }
      Action    = ["sts:AssumeRole", "sts:TagSession"]
    }]
  })
  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "controller_pod_identity_assume" {
  count = var.enable_wi ? 1 : 0

  role       = aws_iam_role.controller_pod_identity[0].name
  policy_arn = aws_iam_policy.assume_target_policy[0].arn
}

# Pod Identity association for the controller SA.
resource "aws_eks_pod_identity_association" "controller" {
  count = var.enable_wi ? 1 : 0

  depends_on = [module.eks]

  cluster_name    = local.name
  namespace       = var.wi_k8s_sa_ns
  service_account = var.wi_k8s_sa_name_controller_pod_identity
  role_arn        = aws_iam_role.controller_pod_identity[0].arn
}
