# Demo: `send` ships an 0800 over TCP and decodes the 0810 reply.
# Render: make demo  (needs vhs; uses the in-repo mock server under spec/mock)

Output docs/demo-send.gif

Set Shell "bash"
Set FontSize 14
Set Width 1080
Set Height 960
Set Padding 14
Set TypingSpeed 35ms
Set Theme "Catppuccin Mocha"

Hide
Type "export PATH=$PWD:$PATH PS1='$ '"
Enter
# Build and start the deterministic local mock server (single-shot, 127.0.0.1).
Type "go build -o /tmp/iso8583-mock ./spec/mock"
Enter
Sleep 1s
Type "/tmp/iso8583-mock --addr 127.0.0.1:18583 --framing 2byte-binary --reply-hex $(tr -d ' \n\r\t' < examples/basei/0810-network-echo-response.hex) >/tmp/iso8583-mock.log 2>&1 &"
Enter
Sleep 800ms
Type "clear"
Enter
Show

Sleep 500ms

Type "iso8583tool send 127.0.0.1:18583 examples/basei/0800-network-echo.hex"
Sleep 300ms
Enter
Sleep 5s
