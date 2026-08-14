resource "pingoneaic_oauth2_client" "probe" {
  realm = "alpha"
  name  = "ExampleProbe"

  core = {
    client_type            = "Confidential"
    scopes                 = ["openid"]
    access_token_lifetime  = 3600
    refresh_token_lifetime = 604800
  }

  advanced = {
    grant_types                = ["client_credentials"]
    response_types             = ["token"]
    is_consent_implied         = true
    token_endpoint_auth_method = "client_secret_post"
  }
}
