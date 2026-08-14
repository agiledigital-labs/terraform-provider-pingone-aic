resource "pingoneaic_esv_variable" "probe" {
  name            = "esv-example-probe"
  expression_type = "string"
  value           = "hello"
  description     = "Example ESV. Apply leaves loaded=false until a tenant restart."
}
