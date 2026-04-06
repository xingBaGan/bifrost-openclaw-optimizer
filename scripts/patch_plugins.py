"""Patch plugins.go to register smart-classifier and classifier in loadBuiltinPlugins."""
import sys

filepath = sys.argv[1]
code = open(filepath).read()

# 我们通过将 anchor 设为 telemetry (Order 1)，将 classifier 插入它之前（Order 0）
anchor = 's.Config.SetPluginOrderInfo(telemetry.PluginName, builtinPlacement, schemas.Ptr(1))'

if anchor not in code:
    print("ERROR: anchor not found in plugins.go", file=sys.stderr)
    sys.exit(1)

# Build insertion using real tab characters
t = "\t"
insert_lines = [
    "",
    f"{t}// 0. Classifier (if configured in PluginConfigs)",
    f'{t}classifierConfig := s.getPluginConfig("classifier")',
    f"{t}if classifierConfig != nil && classifierConfig.Enabled {{",
    f'{t}{t}s.registerPluginWithStatus(ctx, "classifier", nil, classifierConfig.Config, false)',
    f"{t}}} else {{",
    f'{t}{t}s.markPluginDisabled("classifier")',
    f"{t}}}",
    f'{t}s.Config.SetPluginOrderInfo("classifier", builtinPlacement, schemas.Ptr(0))',
]

insert = "\n".join(insert_lines)
code = code.replace(anchor, anchor + "\n" + insert)

# 同时也把 Smart Classifier 挂在最后 (Order 8)
anchor2 = 's.Config.SetPluginOrderInfo(maxim.PluginName, builtinPlacement, schemas.Ptr(7))'
if anchor2 in code:
    insert_lines2 = [
        "",
        f"{t}// 8. Smart Classifier",
        f'{t}smartClassifierConfig := s.getPluginConfig("smart-classifier")',
        f"{t}if smartClassifierConfig != nil && smartClassifierConfig.Enabled {{",
        f'{t}{t}s.registerPluginWithStatus(ctx, "smart-classifier", nil, smartClassifierConfig.Config, false)',
        f"{t}}} else {{",
        f'{t}{t}s.markPluginDisabled("smart-classifier")',
        f"{t}}}",
        f'{t}s.Config.SetPluginOrderInfo("smart-classifier", builtinPlacement, schemas.Ptr(8))',
    ]
    insert2 = "\n".join(insert_lines2)
    code = code.replace(anchor2, anchor2 + "\n" + insert2)

open(filepath, 'w').write(code)
print("OK: patched loadBuiltinPlugins with classifier (Order 0) and smart-classifier")
