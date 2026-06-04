# Demo: `diff` compares two messages and masks cardholder data by default.
# Render: make demo   (prepares /tmp/before.hex and /tmp/after.hex)

Output docs/demo-diff.gif

Set Shell "bash"
Set FontSize 16
Set Width 1080
Set Height 720
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

# Safe by default: the PAN and track are masked like view.
Type "iso8583tool diff /tmp/before.hex /tmp/after.hex"
Sleep 300ms
Enter
Sleep 3500ms

Hide
Type "clear"
Enter
Show

Sleep 500ms

# --unsafe opts in to the raw values for local debugging.
Type "iso8583tool diff /tmp/before.hex /tmp/after.hex --unsafe"
Sleep 300ms
Enter
Sleep 3500ms
