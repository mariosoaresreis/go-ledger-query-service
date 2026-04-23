locals {
  gcp_services = [
    "container.googleapis.com",
    "sqladmin.googleapis.com",
    "artifactregistry.googleapis.com",
    "servicenetworking.googleapis.com",
    "compute.googleapis.com",
    "iam.googleapis.com",
  ]
}

resource "google_project_service" "apis" {
  for_each           = toset(local.gcp_services)
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# ── VPC (separate from command VPC) ──────────────────────────────────────────
module "network" {
  source        = "../../modules/network"
  project_id    = var.project_id
  environment   = "${var.environment}-query"
  region        = var.region
  subnet_cidr   = var.subnet_cidr
  pods_cidr     = var.pods_cidr
  services_cidr = var.services_cidr
  depends_on    = [google_project_service.apis]
}

# ── GKE Cluster (query region) ────────────────────────────────────────────────
module "gke" {
  source              = "../../modules/gke"
  project_id          = var.project_id
  environment         = "${var.environment}-query"
  region              = var.region
  zone_suffix         = "b"
  vpc_id              = module.network.vpc_id
  subnet_name         = module.network.gke_subnet_name
  pods_range_name     = module.network.pods_range_name
  services_range_name = module.network.services_range_name
  node_count          = var.node_count
  machine_type        = var.machine_type
  min_nodes           = var.min_nodes
  max_nodes           = var.max_nodes
}

# ── Cloud SQL – read-side DB (separate instance) ──────────────────────────────
module "cloud_sql" {
  source        = "../../modules/cloud-sql"
  project_id    = var.project_id
  environment   = "${var.environment}-query"
  region        = var.region
  vpc_self_link = module.network.vpc_self_link
  tier          = var.db_tier
  db_username   = var.db_username
  db_password   = var.db_password
  db_name       = "ledger_query"
  # Public IP needed: query GKE and command Kafka are in separate VPCs with no private peering.
  # The query service connects to Kafka via external LoadBalancer (34.134.143.95:9095).
  # For production, use VPC peering + internal LB to avoid public exposure.
  enable_public_ip = true
  authorized_networks = [
    { name = "gke-query-nodes", value = "0.0.0.0/0" }
  ]
  depends_on    = [module.network]
}

# ── Kubernetes workloads for the query service ────────────────────────────────
module "kubernetes_query" {
  source = "../../modules/kubernetes-query"

  query_db_host     = module.cloud_sql.db_host
  query_db_username = var.db_username
  query_db_password = var.db_password
  # Kafka bootstrap is in the command VPC; use the external Kafka address or VPC peering address
  kafka_bootstrap_servers = var.kafka_bootstrap_servers
  app_image               = "${var.artifact_registry_url}/ledger-query-service:${var.app_image_tag}"
  app_replicas            = var.app_replicas

  depends_on = [module.gke, module.cloud_sql]
}

