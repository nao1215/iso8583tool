# Demo: `diff` compares two messages field by field.
# Render: make build && vhs docs/demo-diff.tape  -> docs/demo-diff.gif

Output docs/demo-diff.gif

Set Shell "bash"
Set FontSize 16
Set Width 1060
Set Height 720
Set Padding 14
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"

Hide
Type "export PATH=$PWD:$PATH PS1='$ '"
Enter
Type "cp examples/basei/0100-auth-request.hex /tmp/before.hex"
Enter
Type "iso8583tool convert examples/basei/0100-auth-request.hex | sed 's/000000005000/000000009999/' | iso8583tool convert > /tmp/after.hex"
Enter
Type "clear"
Enter
Show

Sleep 600ms

# Compare two messages: amount and the EMV amount tag changed.
Type "iso8583tool diff /tmp/before.hex /tmp/after.hex"
Sleep 400ms
Enter
Sleep 4s

Hide
Type "clear"
Enter
Show

# JSON output is jq-friendly.
Type "iso8583tool diff /tmp/before.hex /tmp/after.hex --format json | jq '.changes[0]'"
Sleep 400ms
Enter
Sleep 3s
