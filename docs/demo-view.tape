# Demo: `view` decodes a message (PAN masked, EMV tag names, summary).
# Render: make demo

Output docs/demo-view.gif

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

Type "iso8583tool view examples/basei/0110-auth-response.hex"
Sleep 300ms
Enter
Sleep 5s
