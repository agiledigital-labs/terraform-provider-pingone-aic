# `_id` on the wire is Terraform_service_desk. Access rules must reference
# internal/role/Terraform_service_desk, not the display name.
resource "pingoneaic_internal_role" "probe" {
  name        = "service_desk"
  description = "Terraform probe — copy, not a built-in"

  privilege {
    name        = "probe-users"
    path        = "managed/alpha_user"
    actions     = []
    permissions = ["VIEW"]
    filter      = "/userName eq \"does-not-exist\""

    access_flag {
      attribute = "userName"
      read_only = true
    }
  }
}
