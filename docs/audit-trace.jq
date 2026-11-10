def attr($k): (.attributes // []) | map(select(.key == $k) | .value | (.stringValue // .intValue // .boolValue)) | first;
def tally(f): group_by(f) | map({key: (.[0] | f | tostring), value: length}) | from_entries;

[.result.resourceSpans[]?.scopeSpans[]?.spans[]?] as $spans
| ($spans | map(select(.name == "run.transition"))) as $transitions
| select($transitions | length > 0)
| {
    denies: {
      by_rule: ($spans | map(select(.name == "plugin.call" and attr("harness.verdict") == "deny")) | tally(attr("harness.plugin"))),
      hook_denies: ($spans | map(select(.name == "hook.handle" and attr("harness.verdict") == "deny")) | length)
    },
    reworks: ($transitions | map(select(attr("harness.event") == "request-rework")) | length),
    amends: ($transitions | map(select(attr("harness.event") == "amend")) | length),
    stop_blocks: ($spans | map(select(.name == "run.decide_stop" and attr("harness.stop.block") == true)) | length),
    tool_outcomes: ($spans | map(select(.name == "tool.call")) | tally(attr("harness.tool.outcome")))
  }
