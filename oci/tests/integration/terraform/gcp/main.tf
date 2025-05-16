provider "google" {
  project = var.gcp_project_id
  region  = var.gcp_region
  zone    = var.gcp_zone
}

resource "random_pet" "suffix" {}

locals {
  name               = "flux-test-${random_pet.suffix.id}"
  federation_pool_id = var.enable_wi ? google_iam_workload_identity_pool.main[0].name : ""
  gke_pool_id        = "projects/${data.google_project.project.number}/locations/global/workloadIdentityPools/${module.gke.project}.svc.id.goog"

  # This principal represents a GCP service account for testing
  # impersonation of GCP service accounts for both the built-in
  # Workload Identity Federation for GKE and also other Kubernetes
  # clusters.
  wi_gcp_sa_principal = var.enable_wi ? "serviceAccount:${google_service_account.test[0].email}" : ""

  # This principal represents a GKE service account for testing built-in
  # Workload Identity Federation for GKE with direct access.
  wi_k8s_sa_principal_direct_access = var.enable_wi ? "serviceAccount:${var.gcp_project_id}.svc.id.goog[${var.wi_k8s_sa_ns}/${var.wi_k8s_sa_name_direct_access}]" : ""

  # This principal represents a service account (that only happens to be
  # from GKE, but could be from any other type of Kubernetes cluster) for
  # testing Workload Identity Federation for other Kubernetes clusters with
  # direct access.
  wi_k8s_sa_principal_direct_access_federation = var.enable_wi ? "principal://iam.googleapis.com/${local.federation_pool_id}/subject/system:serviceaccount:${var.wi_k8s_sa_ns}:${var.wi_k8s_sa_name_federation_direct_access}" : ""

  permission_principals = var.enable_wi ? [
    local.wi_gcp_sa_principal,
    local.wi_k8s_sa_principal_direct_access,
    local.wi_k8s_sa_principal_direct_access_federation,
  ] : []
}

data "google_project" "project" {
  project_id = var.gcp_project_id
}

module "gke" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/gcp/gke"

  name      = local.name
  tags      = var.tags
  enable_wi = var.enable_wi
  oauth_scopes = var.enable_wi ? null : [
    "https://www.googleapis.com/auth/cloud-platform"
  ]
}

module "gar" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/gcp/gcr"

  name = local.name
  tags = var.tags
}

resource "google_service_account" "test" {
  count       = var.enable_wi ? 1 : 0
  account_id  = local.name
  project     = var.gcp_project_id
  description = "Service account for testing Workload integration for OCI repositories in Flux"
}

resource "google_project_iam_binding" "gar_access" {
  count   = var.enable_wi ? 1 : 0
  project = var.gcp_project_id
  role    = "roles/artifactregistry.repoAdmin"
  members = local.permission_principals
}

resource "google_project_iam_binding" "gke_access" {
  count   = var.enable_wi ? 1 : 0
  project = var.gcp_project_id
  role    = "roles/container.clusterViewer"
  members = local.permission_principals
}

resource "google_service_account_iam_binding" "main" {
  count              = var.enable_wi ? 1 : 0
  service_account_id = google_service_account.test[0].name
  role               = "roles/iam.workloadIdentityUser"
  members            = [
    # This principal represents a GKE service account for testing built-in
    # Workload Identity Federation for GKE with impersonation.
    "serviceAccount:${var.gcp_project_id}.svc.id.goog[${var.wi_k8s_sa_ns}/${var.wi_k8s_sa_name}]",

    # This principal represents a service account (that only happens to be
    # from GKE, but could be from any other type of Kubernetes cluster) for
    # testing Workload Identity Federation for other Kubernetes clusters with
    # impersonation.
    "principal://iam.googleapis.com/${local.federation_pool_id}/subject/system:serviceaccount:${var.wi_k8s_sa_ns}:${var.wi_k8s_sa_name_federation}",
  ]
}

# The Workload Identity Pool and Provider resources are for testing
# Workload Identity Federation for arbitrary types of Kubernetes
# clusters. We test it with a GKE cluster for both the setup simplicity
# (setup for a kind cluster would be more complicated, we just need
# a cluster with a public Issuer URL like the GKE cluster itself), and
# also to prove that it works for GKE clusters as well, which may be
# useful in the future in case GKE blocks JWTs without pod claims
# for the built-in Workload Identity Federation for GKE (think about
# Workload Identity Pool and Provider as AWS EKS IRSA and built-in
# Workload Identity Federation for GKE as AWS EKS Pod Identity).

resource "google_iam_workload_identity_pool" "main" {
  count                     = var.enable_wi ? 1 : 0
  workload_identity_pool_id = local.name
}

resource "google_iam_workload_identity_pool_provider" "main" {
  count                              = var.enable_wi ? 1 : 0
  workload_identity_pool_id          = google_iam_workload_identity_pool.main[0].workload_identity_pool_id
  workload_identity_pool_provider_id = local.name

  oidc {
    issuer_uri = "https://container.googleapis.com/v1/${module.gke.full_name}"
  }

  attribute_mapping = {
    "google.subject" = "assertion.sub"
  }
}
