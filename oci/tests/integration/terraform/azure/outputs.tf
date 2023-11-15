output "aks_kubeconfig" {
  value     = module.aks.kubeconfig
  sensitive = true
}

output "acr_registry_url" {
  value = module.acr.registry_url
}

output "acr_registry_id" {
  value = module.acr.registry_id
}

output "workload_identity_client_id" {
  value = var.enable_wi ? azurerm_user_assigned_identity.wi-id[0].client_id : ""
}
