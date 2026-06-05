// Command mock is a deterministic local TCP server used only by the send
// end-to-end tests. It accepts a single connection, reads one framed request,
// and replies with a fixed framed response using the same framing as the
// client. It never touches the network beyond 127.0.0.1 and reuses the
// production framing code so the test exercises the real wire format.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/nao1215/iso8583tool/internal/service"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address")
	framingName := flag.String("framing", "2byte-binary", "framing: 2byte-binary, 4digit-ascii, or none")
	replyHex := flag.String("reply-hex", "", "hex-encoded response payload to send back")
	readyFile := flag.String("ready-file", "", "file to write the chosen listen address to once ready")
	noReply := flag.Bool("no-reply", false, "read the request but never reply (to exercise client timeouts)")
	flag.Parse()

	framing, err := service.ParseFraming(*framingName)
	if err != nil {
		fail(err)
	}
	reply, err := hex.DecodeString(strings.TrimSpace(*replyHex))
	if err != nil {
		fail(fmt.Errorf("decode --reply-hex: %w", err))
	}

	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", *addr)
	if err != nil {
		fail(err)
	}
	defer func() { _ = ln.Close() }()

	// Publish the actual address (the port is ephemeral) so the test can connect
	// without guessing a port. The ready file appearing is the test's signal.
	if *readyFile != "" {
		if err := os.WriteFile(*readyFile, []byte(ln.Addr().String()), 0o600); err != nil {
			fail(err)
		}
	}
	fmt.Println(ln.Addr().String())

	conn, err := ln.Accept()
	if err != nil {
		fail(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	if _, err := framing.ReadResponse(conn); err != nil {
		fail(fmt.Errorf("read request: %w", err))
	}

	if *noReply {
		// Hold the connection open so the client blocks on its read deadline.
		time.Sleep(20 * time.Second)
		return
	}

	framed, err := framing.Encode(reply)
	if err != nil {
		fail(err)
	}
	if _, err := conn.Write(framed); err != nil {
		fail(err)
	}
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mock:", err)
	os.Exit(1)
}
