# Demo: `redact` masks cardholder data for safe sharing.
# Render: make build && vhs docs/demo-redact.tape  -> docs/demo-redact.gif

Output docs/demo-redact.gif

Set Shell "bash"
Set FontSize 16
Set Width 1060
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

Sleep 600ms

# PAN keeps its BIN and last four; track data, PIN, and the EMV cryptogram are
# masked. Output is safe to paste into chat.
Type "iso8583tool redact examples/basei/0100-auth-request.hex --format text"
Sleep 400ms
Enter
Sleep 4s

Hide
Type "clear"
Enter
Show

# JSON is the default, ready for jq or an LLM.
Type "iso8583tool redact examples/basei/0100-auth-request.hex | jq .fields"
Sleep 400ms
Enter
Sleep 3s
