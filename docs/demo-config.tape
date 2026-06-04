# Demo: a single --config file bundles the spec selection and extension catalog.
# Render: make build && vhs docs/demo-config.tape  -> docs/demo-config.gif

Output docs/demo-config.gif

Set Shell "bash"
Set FontSize 15
Set Width 1060
Set Height 860
Set Padding 14
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"

Hide
Type "export PATH=$PWD:$PATH PS1='$ '"
Enter
Type "clear"
Enter
Show

Sleep 600ms

# One JSON file picks the spec and overlays the extension-field catalog.
Type "cat examples/iso8583tool.config.json"
Sleep 300ms
Enter
Sleep 3s

# Pass it with --config; no workspace or init step is needed.
Type "iso8583tool validate examples/basei/0110-auth-response.hex --config examples/iso8583tool.config.json"
Sleep 400ms
Enter
Sleep 4s
