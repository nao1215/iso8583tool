# Demo: switch from the BASE I starter defaults to plain ISO 8583:1987 ASCII.
# Render: make build && vhs docs/demo-spec87ascii.tape  -> docs/demo-spec87ascii.gif

Output docs/demo-spec87ascii.gif

Set Shell "bash"
Set FontSize 15
Set Width 1060
Set Height 840
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

# A tiny config switches the active spec away from BASE I.
Type "cat examples/spec87ascii.config.json"
Sleep 300ms
Enter
Sleep 1800ms

# Validate a plain ISO 8583:1987 ASCII sample with no BASE I private overlays.
Type "iso8583tool validate examples/spec87ascii/0800-network-echo.hex --config examples/spec87ascii.config.json"
Sleep 300ms
Enter
Sleep 2500ms

Hide
Type "clear"
Enter
Show

# The same message can be viewed and converted with the alternate spec.
Type "iso8583tool view examples/spec87ascii/0800-network-echo.hex --config examples/spec87ascii.config.json"
Sleep 300ms
Enter
Sleep 3500ms
