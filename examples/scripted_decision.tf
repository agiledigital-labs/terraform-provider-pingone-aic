resource "pingoneaic_script" "probe" {
  realm   = "alpha"
  name    = "Example Probe"
  context = "AUTHENTICATION_TREE_DECISION_NODE"
  source  = <<-JS
    outcome = "ok";
  JS
}

resource "pingoneaic_scripted_decision_node" "probe" {
  realm     = "alpha"
  script_id = pingoneaic_script.probe.id
  outcomes  = ["ok", "error"]
}

resource "pingoneaic_journey" "probe" {
  realm       = "alpha"
  name        = "ExampleProbe"
  description = "Minimal scripted-decision journey."
  entry_node  = pingoneaic_scripted_decision_node.probe.id

  node {
    id           = pingoneaic_scripted_decision_node.probe.id
    type         = "ScriptedDecisionNode"
    display_name = "Example Probe"
    connections = {
      ok    = "success"
      error = "failure"
    }
  }
}
