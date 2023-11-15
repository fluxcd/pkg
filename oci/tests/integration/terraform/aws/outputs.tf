output "eks_cluster_name" {
  value = module.eks.cluster_id
}

output "eks_cluster_ca_certificate" {
  value     = module.eks.cluster_ca_data
  sensitive = true
}

output "eks_cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "eks_cluster_arn" {
  value = module.eks.cluster_arn
}

output "region" {
  value = module.eks.region
}

output "cross_region" {
  value = var.cross_region
}


output "ecr_repository_url" {
  value = module.test_ecr.repository_url
}

output "ecr_cross_region_repository_url" {
  value = module.test_ecr_cross_reg.repository_url
}

output "ecr_registry_id" {
  value = module.test_ecr.registry_id
}

output "ecr_test_app_repo_url" {
  value = module.test_app_ecr.repository_url
}

output "aws_wi_iam_arn" {
  value = var.enable_wi ? aws_iam_role.assume_role[0].arn : ""
}
