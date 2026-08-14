resource "pingoneaic_managed_object" "probe" {
  name  = "example_probe"
  title = "Example Probe"
  icon  = "fa-database"

  property {
    name     = "label"
    type     = "string"
    title    = "Label"
    required = true
  }

  # Copies of the stock alpha_user hooks. They only fire for records of this
  # type — they do not attach to alpha_user.
  hook {
    event  = "onCreate"
    source = file("${path.module}/hooks/onCreate.js")
  }

  hook {
    event  = "onUpdate"
    source = file("${path.module}/hooks/onUpdate.js")
  }
}
