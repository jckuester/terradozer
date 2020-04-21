provider "google" {
  credentials = file("~/.gcp/terradozer-acc-test-e478b28f1de2.json")
  project = "terradozer-acc-test"
  region  = "us-central1"
}


resource "google_compute_network" "vpc_network" {
  name                    = "terraform-network"
  auto_create_subnetworks = "true"
}

