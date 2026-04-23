provider "google" {
  project = var.project_id
  region  = var.region
}

data "google_client_config" "default" {}

data "google_container_cluster" "query_gke" {
  name     = "${var.project_id}-${var.environment}-query-gke"
  location = "${var.region}-b"
  project  = var.project_id
}

provider "kubernetes" {
  host                   = "https://${data.google_container_cluster.query_gke.endpoint}"
  token                  = data.google_client_config.default.access_token
  cluster_ca_certificate = base64decode(data.google_container_cluster.query_gke.master_auth[0].cluster_ca_certificate)
}

provider "helm" {
  kubernetes {
    host                   = "https://${data.google_container_cluster.query_gke.endpoint}"
    token                  = data.google_client_config.default.access_token
    cluster_ca_certificate = base64decode(data.google_container_cluster.query_gke.master_auth[0].cluster_ca_certificate)
  }
}

