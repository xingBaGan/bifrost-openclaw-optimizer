"""Patch plugins.go to register smart-classifier and classifier in loadBuiltinPlugins."""
import sys

filepath = sys.argv[1]
code = open(filepath).read()

anchor = 's.Config.SetPluginOrderInfo(maxim.PluginName, builtinPlacement, schemas.Ptr(7))'

if anchor not in code:
    print("ERROR: anchor not found in plugins.go", file=sys.stderr)
    sys.exit(1)

# Build insertion using real tab characters
t = "\t"
insert_lines = [
    "",
    f"{t}// 8. Smart Classifier (if configured in PluginConfigs)",
    f'{t}smartClassifierConfig := s.getPluginConfig("smart-classifier")',
    f"{t}if smartClassifierConfig != nil && smartClassifierConfig.Enabled {{",
    f'{t}{t}s.registerPluginWithStatus(ctx, "smart-classifier", nil, smartClassifierConfig.Config, false)',
    f"{t}}} else {{",
    f'{t}{t}s.markPluginDisabled("smart-classifier")',
    f"{t}}}",
    f'{t}s.Config.SetPluginOrderInfo("smart-classifier", builtinPlacement, schemas.Ptr(8))',
    "",
    f"{t}// 9. Classifier (if configured in PluginConfigs)",
    f'{t}classifierConfig := s.getPluginConfig("classifier")',
    f"{t}if classifierConfig != nil && classifierConfig.Enabled {{",
    f'{t}{t}s.registerPluginWithStatus(ctx, "classifier", nil, classifierConfig.Config, false)',
    f"{t}}} else {{",
    f'{t}{t}s.markPluginDisabled("classifier")',
    f"{t}}}",
    f'{t}s.Config.SetPluginOrderInfo("classifier", builtinPlacement, schemas.Ptr(9))',
]

insert = "\n".join(insert_lines)
code = code.replace(anchor, anchor + "\n" + insert)
open(filepath, 'w').write(code)
print("OK: patched loadBuiltinPlugins with smart-classifier and classifier")
