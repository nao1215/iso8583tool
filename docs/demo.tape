# VHS tape for iso8583tool demo.
# Build the binary first:  make build
# Render:                  vhs docs/demo.tape
#
# Output: docs/demo.gif

Output docs/demo.gif

Set Shell "bash"
Set FontSize 18
Set Width 1100
Set Height 820
Set Padding 16
Set TypingSpeed 45ms
Set Theme "Catppuccin Mocha"

# Put the freshly built binary on PATH so the typed commands stay clean.
Hide
Type "export PATH=$PWD:$PATH PS1='$ '"
Enter
Type "clear"
Enter
Show

Sleep 600ms

# 1. List the built-in BASE I samples.
Type "iso8583tool sample"
Sleep 400ms
Enter
Sleep 2s

# 2. Inspect a 0100 auth request whose Field 55 carries an unknown EMV tag.
Type "iso8583tool view --file examples/basei/0100-auth-request-unknown-tlv.hex"
Sleep 400ms
Enter
Sleep 3s

# 3. Validate it: the unknown TLV tag is reported as a warning, not an error.
Type "iso8583tool validate --file examples/basei/0100-auth-request-unknown-tlv.hex"
Sleep 400ms
Enter
Sleep 3s

# 4. Build a packed message back from a JSON document.
Type "iso8583tool write --input examples/basei/0100-auth-request.json"
Sleep 400ms
Enter
Sleep 2500ms

Sleep 1s
