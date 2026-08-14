# New grants. Applying this does not touch the tenant's existing configs[]
# entries — the provider appends, and identifies the new row by its content hash.
resource "pingoneaic_access_rule" "probe" {
  pattern = "endpoint/Terraform_validateQueryFilter"
  roles   = "internal/role/openidm-authorized"
  methods = "read,query"
  actions = "*"
}

# Authentication mappings live in a different document and use an array for roles.
resource "pingoneaic_authentication_mapping" "probe" {
  subject    = "Terraform_auth_probe"
  local_user = "internal/user/anonymous"
  roles      = ["internal/role/Terraform_auth_probe"]
}
