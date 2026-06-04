# Demo: `sample` lists and exports built-in BASE I fixtures; `version` too.
# Render: make build && vhs docs/demo-sample.tape  -> docs/demo-sample.gif

Output docs/demo-sample.gif

Set Shell "bash"
Set FontSize 16
Set Width 1080
Set Height 700
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

Type "iso8583tool version"
Sleep 300ms
Enter
Sleep 1200ms

# List the built-in samples.
Type "iso8583tool sample"
Sleep 300ms
Enter
Sleep 2s

# Export one as packed hex.
Type "iso8583tool sample 0100-auth-request --format hex"
Sleep 300ms
Enter
Sleep 2s

# Or write it straight to a file.
Type "iso8583tool sample 0110-auth-response --format hex --output /tmp/0110.hex"
Sleep 300ms
Enter
Sleep 2s
