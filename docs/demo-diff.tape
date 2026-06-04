# Demo: `diff` compares two messages field by field.
# Render: make demo   (prepares /tmp/before.hex and /tmp/after.hex)

Output docs/demo-diff.gif

Set Shell "bash"
Set FontSize 16
Set Width 1080
Set Height 600
Set Padding 14
Set TypingSpeed 35ms
Set Theme "Catppuccin Mocha"

Hide
Type "export PATH=$PWD:$PATH PS1='$ '"
Enter
Type "clear"
Enter
Show

Sleep 500ms

Type "iso8583tool diff /tmp/before.hex /tmp/after.hex"
Sleep 300ms
Enter
Sleep 4s
