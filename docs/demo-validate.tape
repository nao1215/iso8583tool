# Demo: `validate` reports decoded fields, warnings, and exit codes.
# Render: make build && vhs docs/demo-validate.tape  -> docs/demo-validate.gif

Output docs/demo-validate.gif

Set Shell "bash"
Set FontSize 15
Set Width 1080
Set Height 760
Set Padding 14
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"

Hide
Type "export PATH=$PWD:$PATH PS1='$ '"
Enter
Type "clear"
Enter
Show

Sleep 500ms

# A good message: decoded fields, an opaque-field warning -> still exit 0.
Type "iso8583tool validate examples/basei/0110-auth-response.hex"
Sleep 300ms
Enter
Sleep 3500ms

Type "echo exit=$?"
Sleep 200ms
Enter
Sleep 1500ms

Hide
Type "clear"
Enter
Show

# A broken message: unpack fails -> error and exit 1.
Type "iso8583tool validate --raw 01007220"
Sleep 300ms
Enter
Sleep 2s

Type "echo exit=$?"
Sleep 200ms
Enter
Sleep 2s
