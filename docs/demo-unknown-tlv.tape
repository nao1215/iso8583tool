# Demo: Field 55 unknown TLV tag is preserved through a JSON -> pack -> view round-trip.
# Render: make build && vhs docs/demo-unknown-tlv.tape  -> docs/demo-unknown-tlv.gif

Output docs/demo-unknown-tlv.gif

Set Shell "bash"
Set FontSize 14
Set Width 1060
Set Height 900
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

# The document feeds Field 55 per tag, including an unknown proprietary tag
# (DF8129) alongside known EMV tags.
Type "cat examples/basei/0100-auth-request-unknown-tlv.json"
Sleep 300ms
Enter
Sleep 2500ms

# Pack it...
Type "iso8583tool convert examples/basei/0100-auth-request-unknown-tlv.json --output /tmp/unknown.hex"
Sleep 300ms
Enter
Sleep 1500ms

# ...and view it: known tags decode, the unknown tag survives and is flagged.
Type "iso8583tool view /tmp/unknown.hex"
Sleep 400ms
Enter
Sleep 5s
