// Command mock is a deterministic local TCP server used only by the send
// end-to-end tests. It accepts a single connection, reads one framed request,
// and replies with a fixed framed response using the same framing as the
// client. It never touches the network beyond 127.0.0.1 and reuses the
// production framing code so the test exercises the real wire format.
package main

import (
	"context"
	"encoding/hex"
	"errors"
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

	if err := run(*addr, *framingName, *replyHex, *readyFile, *noReply); err != nil {
		// When stderr is gone too, the exit code is all that is left to report.
		if _, werr := fmt.Fprintln(os.Stderr, "mock:", err); werr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

func run(addr, framingName, replyHex, readyFile string, noReply bool) (err error) {
	framing, err := service.ParseFraming(framingName)
	if err != nil {
		return err
	}
	reply, err := hex.DecodeString(strings.TrimSpace(replyHex))
	if err != nil {
		return fmt.Errorf("decode --reply-hex: %w", err)
	}

	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ln.Close()) }()

	// Publish the actual address (the port is ephemeral) so the test can connect
	// without guessing a port. The ready file appearing is the test's signal.
	if readyFile != "" {
		if err := os.WriteFile(readyFile, []byte(ln.Addr().String()), 0o600); err != nil {
			return err
		}
	}
	if _, err := fmt.Println(ln.Addr().String()); err != nil {
		return err
	}

	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, conn.Close()) }()
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}

	if _, err := framing.ReadResponse(conn); err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	if noReply {
		// Hold the connection open so the client blocks on its read deadline.
		time.Sleep(20 * time.Second)
		return nil
	}

	framed, err := framing.Encode(reply)
	if err != nil {
		return err
	}
	// A single Write may be short; loop so the whole frame reaches the client and
	// the e2e tests stay deterministic.
	for len(framed) > 0 {
		n, err := conn.Write(framed)
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("short write: wrote 0 bytes")
		}
		framed = framed[n:]
	}
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return nil
}
