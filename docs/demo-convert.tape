# Demo: `convert` auto-detects direction between a packed message and JSON, and
# works in pipes (stdin).
# Render: make build && vhs docs/demo-convert.tape  -> docs/demo-convert.gif

Output docs/demo-convert.gif

Set Shell "bash"
Set FontSize 14
Set Width 1080
Set Height 880
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

# A packed message piped in is unpacked to a round-trippable JSON document.
Type "iso8583tool sample 0110-auth-response --format hex | iso8583tool convert"
Sleep 400ms
Enter
Sleep 4s

Hide
Type "clear"
Enter
Show

# A JSON document is packed back to hex - direction is detected automatically.
Type "iso8583tool convert examples/basei/0100-auth-request.json"
Sleep 400ms
Enter
Sleep 3s
