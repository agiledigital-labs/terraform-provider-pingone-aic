resource "pingoneaic_idm_endpoint" "probe" {
  name   = "example-probe"
  source = <<-JS
    (function () {
      return { ok: true };
    })();
  JS
}

resource "pingoneaic_idm_schedule" "probe" {
  name           = "example-probe"
  invoke_service = "script"
  type           = "cron"
  schedule       = "0 0 3 1 1 ? 2099"
  enabled        = false
  persisted      = true
  source         = <<-JS
    logger.info("example probe schedule");
  JS
}
