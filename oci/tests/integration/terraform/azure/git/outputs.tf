output "git_repo_url" {
  value = module.devops.repo_url
}

output azure_devops_project_id {
    value = module.devops.project_id
}

output "azure_devops_organization" {
  value     = var.devops_org
}

output "azure_devops_access_token" {
  sensitive = true
  value     = var.pat
}

output "aks_kubeconfig" {
  value     = module.aks.kubeconfig
  sensitive = true
}

output "workload_identity_client_id" {
  value = var.enable_wi ? azurerm_user_assigned_identity.wi-id[0].client_id : ""
}

output "workload_identity_object_id" {
  value = var.enable_wi ? azurerm_user_assigned_identity.wi-id[0].principal_id : ""
}

output "acr_registry_url" {
  value = module.acr.registry_url
}

output "acr_registry_id" {
  value = module.acr.registry_id
}