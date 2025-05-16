output "gcp_kubeconfig" {
  value     = module.gke.kubeconfig
  sensitive = true
}

output "gcp_project" {
  value = module.gke.project
}

output "gcp_region" {
  value = module.gke.region
}

output "gcp_artifact_repository" {
  value = module.gar.artifact_repository_id
}

output "wi_iam_serviceaccount_email" {
  value = var.enable_wi ? google_service_account.test[0].email : ""
}

output "workload_identity_provider" {
  value = var.enable_wi ? google_iam_workload_identity_pool_provider.main[0].name : ""
}

output "cluster_resource" {
  value = module.gke.full_name
}

output "cluster_endpoint" {
  value = module.gke.endpoint
}

output "wi_k8s_sa_principal_direct_access" {
  value = var.enable_wi ? local.wi_k8s_sa_principal_direct_access : ""
}

output "wi_k8s_sa_principal_direct_access_federation" {
  value = var.enable_wi ? local.wi_k8s_sa_principal_direct_access_federation : ""
}
