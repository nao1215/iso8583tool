# Demo: colorized `view` with numeric-to-meaning decoding.
# Render: make build && vhs docs/demo-view.tape  -> docs/demo-view.gif

Output docs/demo-view.gif

Set Shell "bash"
Set FontSize 14
Set Width 1060
Set Height 940
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

# An auth response: F39 response code, F49 currency and the EMV ARC are all
# translated into human meanings, with the structure shown in color.
Type "iso8583tool view examples/basei/0110-auth-response.hex"
Sleep 400ms
Enter
Sleep 5s

Hide
Type "clear"
Enter
Show

# Pull out just the fields you care about by dot-path (pipe-friendly).
Type "iso8583tool view examples/basei/0110-auth-response.hex --filter 39 --filter 49 --filter 55.8A"
Sleep 400ms
Enter
Sleep 3s

Hide
Type "clear"
Enter
Show

# Same data as machine-readable JSON (with a `decoded` array; never colorized).
Type "iso8583tool view examples/basei/0110-auth-response.hex --format json | tail -n 18"
Sleep 400ms
Enter
Sleep 4s
