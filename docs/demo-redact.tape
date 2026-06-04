# Demo: `redact` masks the PAN, track data, and EMV cryptogram.
# Render: make demo

Output docs/demo-redact.gif

Set Shell "bash"
Set FontSize 14
Set Width 1080
Set Height 940
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

Type "iso8583tool redact examples/basei/0100-auth-request.hex --format text"
Sleep 300ms
Enter
Sleep 5s
