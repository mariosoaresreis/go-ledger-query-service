variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "db_password" {
  type        = string
  description = "Password for the query-side Cloud SQL user"
  sensitive   = true
}

variable "kafka_bootstrap_servers" {
  type        = string
  description = "Kafka bootstrap address – must be reachable from the query GKE cluster (e.g. external IP:9092 or VPC-peered address)"
}

variable "artifact_registry_url" {
  type        = string
  description = "Artifact Registry repository URL (shared with command side)"
  default     = "us-central1-docker.pkg.dev/ledger-493222/ledger"
}

variable "environment" {
  type    = string
  default = "dev"
}

variable "region" {
  type        = string
  description = "GCP region for query service"
  default     = "us-east1"
}

variable "subnet_cidr" {
  type    = string
  default = "10.40.0.0/24"
}

variable "pods_cidr" {
  type    = string
  default = "10.50.0.0/16"
}

variable "services_cidr" {
  type    = string
  default = "10.60.0.0/16"
}

variable "node_count" {
  type    = number
  default = 1
}

variable "machine_type" {
  type    = string
  default = "e2-small"
}

variable "min_nodes" {
  type    = number
  default = 1
}

variable "max_nodes" {
  type    = number
  default = 2
}

variable "db_tier" {
  type    = string
  default = "db-f1-micro"
}

variable "db_username" {
  type    = string
  default = "ledger_query"
}

variable "app_image_tag" {
  type    = string
  default = "latest"
}

variable "app_replicas" {
  type    = number
  default = 1
}

