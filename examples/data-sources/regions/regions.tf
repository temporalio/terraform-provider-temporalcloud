provider "temporalcloud" {

}

data "temporalcloud_regions" "regions" {}

output "regions" {
  value = data.temporalcloud_regions.regions.regions
}
