# Overview demo: inspect, compare, redact.
# Render: make build && vhs docs/demo.tape  -> docs/demo.gif
# (before.hex/after.hex are prepared by the Makefile `demo` target.)

Output docs/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1080
Set Height 900
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

# Inspect a message: values decoded, PAN masked.
Type "iso8583tool view examples/basei/0110-auth-response.hex"
Sleep 300ms
Enter
Sleep 4s

Hide
Type "clear"
Enter
Show

# Compare two messages.
Type "iso8583tool diff /tmp/before.hex /tmp/after.hex"
Sleep 300ms
Enter
Sleep 3500ms

Hide
Type "clear"
Enter
Show

# Mask cardholder data before sharing.
Type "iso8583tool redact examples/basei/0110-auth-response.hex --format text"
Sleep 300ms
Enter
Sleep 3500ms
