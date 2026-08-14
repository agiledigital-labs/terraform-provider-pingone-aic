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
}
