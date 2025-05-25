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
  value = module.gcr.artifact_repository_id
}

output "wi_iam_serviceaccount_email" {
  value = var.enable_wi ? google_service_account.test[0].email : ""
}

output "workload_identity_provider" {
  value = var.enable_wi ? google_iam_workload_identity_pool_provider.main[0].name : ""
}
