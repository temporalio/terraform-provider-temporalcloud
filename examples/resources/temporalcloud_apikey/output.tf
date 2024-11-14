output "apikey_token" {
  value     = temporalcloud_apikey.global_apikey.token
  sensitive = true
}