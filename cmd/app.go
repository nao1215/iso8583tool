package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/messagespec"
	"github.com/nao1215/iso8583tool/internal/render"
	"github.com/nao1215/iso8583tool/internal/service"
)

const devVersion = "dev"

// Version is overridden at build time via ldflags.
var Version = devVersion

func resolveVersion() string {
	if Version != devVersion {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return devVersion
}

type App struct {
	stdout  io.Writer
	stderr  io.Writer
	stdin   io.Reader
	workDir string
}

type resolvedContext struct {
	specLabel string
	spec      *messagespec.Spec
	catalog   basei.ExtensionCatalog
}

func NewApp(stdout, stderr io.Writer, stdin io.Reader, workDir string) *App {
	return &App{
		stdout:  stdout,
		stderr:  stderr,
		stdin:   stdin,
		workDir: workDir,
	}
}

func (a *App) Run(args []string) int {
	if len(args) == 0 {
		a.printRootHelp(a.stdout)
		return 0
	}

	switch args[0] {
	case "help":
		// "help" alone prints root help; "help <command>" describes a command.
		if len(args) > 1 {
			return a.runHelp(args[1:])
		}
		a.printRootHelp(a.stdout)
		return 0
	case "-h", "--help":
		if len(args) > 1 {
			writef(a.stderr, "%q takes no arguments; use \"iso8583tool help <command>\"\n\n", args[0])
			a.printRootHelp(a.stderr)
			return 1
		}
		a.printRootHelp(a.stdout)
		return 0
	case "view":
		return a.runView(args[1:])
	case "diff":
		return a.runDiff(args[1:])
	case "redact":
		return a.runRedact(args[1:])
	case "convert":
		return a.runConvert(args[1:])
	case "send":
		return a.runSend(args[1:])
	case "validate":
		return a.runValidate(args[1:])
	case "doctor":
		return a.runDoctor(args[1:])
	case "specs":
		return a.runSpecs(args[1:])
	case "sample":
		return a.runSample(args[1:])
	case "version", "-v", "--version":
		if len(args) > 1 {
			writef(a.stderr, "%q takes no arguments\n\n", args[0])
			a.printRootHelp(a.stderr)
			return 1
		}
		writef(a.stdout, "iso8583tool %s\n", resolveVersion())
		return 0
	default:
		writef(a.stderr, "unknown command: %s\n\n", args[0])
		a.printRootHelp(a.stderr)
		return 1
	}
}

func (a *App) runHelp(args []string) int {
	name := args[0]
	rest := args[1:]

	switch name {
	case "view", "diff", "redact", "convert", "send", "validate", "doctor", "specs", "sample":
		forwarded := make([]string, 0, len(args)+1)
		forwarded = append(forwarded, args...)
		forwarded = append(forwarded, "--help")
		return a.Run(forwarded)
	case "help", "version":
		if len(rest) > 0 {
			writef(a.stderr, "%q takes no arguments\n\n", name)
			a.printRootHelp(a.stderr)
			return 1
		}
		a.printRootHelp(a.stdout)
		return 0
	default:
		writef(a.stderr, "unknown command: %s\n\n", name)
		a.printRootHelp(a.stderr)
		return 1
	}
}

func (a *App) runView(args []string) int {
	flagSet := newFlagSet("view", a.stderr)
	specName := flagSet.String("spec", "", "spec preset or JSON spec path")
	configPath := flagSet.String("config", "", "path to a JSON config (defaults + extension catalog)")
	raw := flagSet.String("raw", "", "inline input message instead of a file argument")
	encoding := flagSet.String("encoding", "auto", "input encoding: auto, hex, or raw")
	format := flagSet.String("format", "describe", "output format: describe or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	unsafe := flagSet.Bool("unsafe", false, "show raw PAN, track, PIN, and private-field data (default: masked)")
	var filters multiFlag
	flagSet.Var(&filters, "filter", "only show this field path (repeatable, e.g. --filter 39 --filter 55.9F02)")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "Inspect an ISO8583 message with the configured spec.")
		writeLine(flagSet.Output(), "Usage: iso8583tool view [MESSAGE|-] [--filter PATH ...] [--unsafe] [--encoding auto|hex|raw] [--format describe|json] [--spec NAME|PATH] [--config PATH] [--color auto|always|never]")
		writeLine(flagSet.Output(), "Reads from stdin when MESSAGE is '-' or omitted.")
		writeLine(flagSet.Output(), "Cardholder data is masked by default; pass --unsafe to show raw values.")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	target := flagSet.Arg(0)
	if flagSet.NArg() > 1 {
		flagSet.Usage()
		return 1
	}

	mode, err := resolveColorMode(*color, *noColor)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	ctx, err := a.loadContext(*specName, *configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, err := a.readMessageInput(target, *raw, *encoding)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	pal := a.palette(mode, *format)
	result, err := service.ViewMessage(input, ctx.spec.MessageSpec, ctx.catalog, *format, filters, pal, *unsafe)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	// Filtered output is just the matched fields, kept terse for piping.
	if len(filters) > 0 {
		_, _ = io.WriteString(a.stdout, result.Body)
		if !strings.HasSuffix(result.Body, "\n") {
			_, _ = io.WriteString(a.stdout, "\n")
		}
		return 0
	}

	if *format == "describe" {
		writef(a.stdout, "%s %s\n", pal.Dim("Spec:"), pal.Bold(ctx.specLabel))
		if result.Summary != "" {
			writef(a.stdout, "%s %s\n", pal.Dim("Summary:"), pal.Cyan(result.Summary))
		}
		_, _ = io.WriteString(a.stdout, "\n")
	}
	_, _ = io.WriteString(a.stdout, result.Body)
	if !strings.HasSuffix(result.Body, "\n") {
		_, _ = io.WriteString(a.stdout, "\n")
	}
	if *format == "describe" && len(result.Extensions) > 0 {
		writef(a.stdout, "\n%s\n", pal.BoldCyan("Extension Field Strategy:"))
		for _, ext := range result.Extensions {
			writef(a.stdout, "- %s %s [%s]: %s\n",
				pal.Green(fmt.Sprintf("F%d", ext.ID)), ext.Name,
				strategyColor(pal, string(ext.Strategy)), pal.Dim(ext.Description))
		}
	}
	if *format == "describe" && len(result.UnknownTags) > 0 {
		writef(a.stdout, "\n%s\n", pal.Yellow("Unknown TLV Tags:"))
		for _, tag := range result.UnknownTags {
			writef(a.stdout, "- %s: %s\n", pal.Yellow(tag.Path), tag.Raw)
		}
	}
	return 0
}

func (a *App) runDiff(args []string) int {
	flagSet := newFlagSet("diff", a.stderr)
	specName := flagSet.String("spec", "", "spec preset or JSON spec path")
	configPath := flagSet.String("config", "", "path to a JSON config (defaults + extension catalog)")
	encoding := flagSet.String("encoding", "auto", "input encoding: auto, hex, or raw")
	format := flagSet.String("format", "text", "output format: text or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	unsafe := flagSet.Bool("unsafe", false, "show raw PAN, track, and unknown TLV values (default: masked like view)")
	var filters multiFlag
	flagSet.Var(&filters, "filter", "only compare this field path (repeatable)")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "Compare two ISO8583 messages field by field.")
		writeLine(flagSet.Output(), "Usage: iso8583tool diff BEFORE AFTER [--filter PATH ...] [--encoding auto|hex|raw] [--format text|json] [--unsafe] [--spec NAME|PATH] [--config PATH] [--color auto|always|never]")
		writeLine(flagSet.Output(), "Either BEFORE or AFTER may be '-' to read that side from stdin.")
		writeLine(flagSet.Output(), "Values are masked like view by default; pass --unsafe to show raw cardholder data.")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	if flagSet.NArg() != 2 {
		flagSet.Usage()
		return 1
	}
	beforeArg, afterArg := flagSet.Arg(0), flagSet.Arg(1)
	if beforeArg == "-" && afterArg == "-" {
		writeLine(a.stderr, "only one of BEFORE or AFTER can be stdin")
		return 1
	}
	if *format != "text" && *format != "json" {
		writef(a.stderr, "unsupported format %q\n", *format)
		return 1
	}

	mode, err := resolveColorMode(*color, *noColor)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	ctx, err := a.loadContext(*specName, *configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	before, err := a.readSideInput(beforeArg, *encoding)
	if err != nil {
		writeLine(a.stderr, fmt.Errorf("before: %w", err))
		return 1
	}
	after, err := a.readSideInput(afterArg, *encoding)
	if err != nil {
		writeLine(a.stderr, fmt.Errorf("after: %w", err))
		return 1
	}

	result, err := service.DiffMessages(ctx.spec.MessageSpec, before, after, filters, *unsafe)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	if *format == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		writef(a.stdout, "%s\n", data)
		return 0
	}

	a.printDiff(result, a.palette(mode, *format))
	return 0
}

// readSideInput reads one side of a diff, auto-detecting the input encoding when
// --encoding is "auto". Only the side explicitly given as "-" reads stdin.
func (a *App) readSideInput(target, encoding string) ([]byte, error) {
	source, err := messageio.ReadSource(target, "", a.sideStdin(target))
	if err != nil {
		return nil, err
	}
	decoded, _, err := resolveInput(source, encoding)
	return decoded, err
}

// sideStdin returns stdin only for the side explicitly given as "-".
func (a *App) sideStdin(target string) io.Reader {
	if target == "-" {
		return a.stdin
	}
	return nil
}

func (a *App) printDiff(result service.DiffResult, pal render.Palette) {
	if len(result.Changes) == 0 {
		writeLine(a.stdout, "No differences.")
		a.printMissingFilters(result.MissingFilters, pal)
		return
	}
	for i, c := range result.Changes {
		if i > 0 {
			_, _ = io.WriteString(a.stdout, "\n")
		}
		label := "Field " + c.Path
		if c.Path == "mti" {
			label = "MTI"
		}
		before := render.SanitizeControl(c.Before)
		after := render.SanitizeControl(c.After)
		writef(a.stdout, "%s %s\n", pal.Bold(label), string(c.Kind))
		switch c.Kind {
		case service.DiffChanged:
			writef(a.stdout, "%s\n", pal.Red("- "+before))
			writef(a.stdout, "%s\n", pal.Green("+ "+after))
		case service.DiffRemoved:
			writef(a.stdout, "%s\n", pal.Red("- "+before))
		case service.DiffAdded:
			writef(a.stdout, "%s\n", pal.Green("+ "+after))
		}
	}
	a.printMissingFilters(result.MissingFilters, pal)
}

// printMissingFilters notes any filters that matched no field in either message,
// so an unmatched filter (typo) is distinguishable from a real no-change result.
func (a *App) printMissingFilters(missing []string, pal render.Palette) {
	if len(missing) == 0 {
		return
	}
	writef(a.stdout, "%s %s\n", pal.Yellow("No field matched filter:"), strings.Join(missing, ", "))
}

func (a *App) runRedact(args []string) int {
	flagSet := newFlagSet("redact", a.stderr)
	specName := flagSet.String("spec", "", "spec preset or JSON spec path")
	configPath := flagSet.String("config", "", "path to a JSON config (defaults + extension catalog)")
	raw := flagSet.String("raw", "", "inline input message instead of a file argument")
	encoding := flagSet.String("encoding", "auto", "input encoding: auto, hex, or raw")
	format := flagSet.String("format", "json", "output format: json or text")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "Mask cardholder data and secrets so a message can be shared safely.")
		writeLine(flagSet.Output(), "Usage: iso8583tool redact [MESSAGE|-] [--raw MESSAGE] [--format json|text] [--spec NAME|PATH] [--config PATH] [--encoding auto|hex|raw] [--color auto|always|never]")
		writeLine(flagSet.Output(), "Reads from stdin when MESSAGE is '-' or omitted; --raw takes an inline message instead.")
		writeLine(flagSet.Output(), "Output is a sanitized document for sharing, not a re-packable message.")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	target := flagSet.Arg(0)
	if flagSet.NArg() > 1 {
		flagSet.Usage()
		return 1
	}

	mode, err := resolveColorMode(*color, *noColor)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	ctx, err := a.loadContext(*specName, *configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, err := a.readMessageInput(target, *raw, *encoding)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	doc, paths, err := service.RedactMessage(ctx.spec.MessageSpec, input)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	switch *format {
	case "json":
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		writef(a.stdout, "%s\n", data)
	case "text":
		a.printRedacted(doc, paths, a.palette(mode, *format))
	default:
		writef(a.stderr, "unsupported format %q\n", *format)
		return 1
	}
	return 0
}

func (a *App) printRedacted(doc messageio.Document, paths []string, pal render.Palette) {
	redactedSet := map[string]bool{}
	for _, p := range paths {
		redactedSet[p] = true
	}
	writef(a.stdout, "%s %s\n", pal.Dim("MTI:"), pal.Bold(doc.MTI))

	keys := make([]string, 0, len(doc.Fields)+len(doc.BinaryFields))
	for k := range doc.Fields {
		keys = append(keys, k)
	}
	for k := range doc.BinaryFields {
		keys = append(keys, k)
	}
	// Numeric field order (F2 before F11), MTI-style first, subfields natural —
	// the same ordering diff and filtered view use.
	service.SortPaths(keys)
	for _, k := range keys {
		v, ok := doc.Fields[k]
		if !ok {
			v = doc.BinaryFields[k]
		}
		v = render.SanitizeControl(v)
		if redactedSet[k] {
			writef(a.stdout, "%s = %s\n", pal.Green("F"+k), pal.Yellow(v))
			continue
		}
		writef(a.stdout, "%s = %s\n", pal.Green("F"+k), v)
	}
	if len(paths) > 0 {
		writef(a.stdout, "%s %s\n", pal.Dim("Redacted:"), strings.Join(paths, ", "))
	}
}

func (a *App) runConvert(args []string) int {
	flagSet := newFlagSet("convert", a.stderr)
	specName := flagSet.String("spec", "", "spec preset or JSON spec path")
	configPath := flagSet.String("config", "", "path to a JSON config (defaults + extension catalog)")
	outputPath := flagSet.String("output", "", "path to write the result")
	encoding := flagSet.String("encoding", "auto", "message-side encoding: auto, hex, or raw")
	to := flagSet.String("to", "", "force output direction: json or hex (default: auto-detect)")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "Convert between a packed message and a JSON document (direction auto-detected).")
		writeLine(flagSet.Output(), "Usage: iso8583tool convert [INPUT|-] [--to json|hex] [--output PATH] [--spec NAME|PATH] [--config PATH] [--encoding auto|hex|raw]")
		writeLine(flagSet.Output(), "JSON input is packed to a message; a message is unpacked to a JSON document.")
		writeLine(flagSet.Output(), "Defaults to the BASE I starter spec; use --spec to pick a preset/JSON spec, and --config for extension catalogs or default overrides.")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	target := flagSet.Arg(0)
	if flagSet.NArg() > 1 {
		flagSet.Usage()
		return 1
	}

	ctx, err := a.loadContext(*specName, *configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	source, err := messageio.ReadSource(target, "", a.inputStdin(target, ""))
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	direction, err := convertDirection(*to, source)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	var out []byte
	var summary string
	switch direction {
	case "hex": // JSON document -> packed message
		doc, err := messageio.ParseDocument(source)
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		result, err := service.WriteMessage(doc, ctx.spec.MessageSpec)
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		// "auto" has no meaning for output; default the written message to hex.
		outEnc := *encoding
		if strings.EqualFold(strings.TrimSpace(outEnc), "auto") {
			outEnc = "hex"
		}
		encoded, err := messageio.EncodeOutput(result.Raw, outEnc)
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		if outEnc == "hex" {
			encoded = append(encoded, '\n')
		}
		out = encoded
		summary = fmt.Sprintf("packed %d fields to %s", result.FieldCount, outEnc)
	case "json": // packed message -> JSON document
		msgRaw, _, err := resolveInput(source, *encoding)
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		doc, err := service.MessageToDocument(ctx.spec.MessageSpec, msgRaw)
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		data = append(data, '\n')
		out = data
		summary = fmt.Sprintf("unpacked message %s to a JSON document", doc.MTI)
	}

	if strings.TrimSpace(*outputPath) != "" {
		if err := os.WriteFile(filepath.Clean(*outputPath), out, 0o600); err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		writef(a.stdout, "Converted with %s (%s).\n", ctx.specLabel, summary)
		writef(a.stdout, "Output: %s\n", *outputPath)
		return 0
	}

	_, _ = a.stdout.Write(out)
	return 0
}

// convertDirection returns the output format: "hex" (pack a document) or
// "json" (unpack a message). An empty `to` auto-detects from the input.
func convertDirection(to string, source []byte) (string, error) {
	switch strings.ToLower(strings.TrimSpace(to)) {
	case "hex":
		return "hex", nil
	case "json":
		return "json", nil
	case "":
		if messageio.LooksLikeJSON(source) {
			return "hex", nil
		}
		return "json", nil
	default:
		return "", fmt.Errorf("unsupported --to %q (use json or hex)", to)
	}
}

func (a *App) runSend(args []string) int {
	flagSet := newFlagSet("send", a.stderr)
	specName := flagSet.String("spec", "", "spec preset or JSON spec path")
	configPath := flagSet.String("config", "", "path to a JSON config (defaults + extension catalog)")
	raw := flagSet.String("raw", "", "inline input message instead of a file argument")
	encoding := flagSet.String("encoding", "auto", "input encoding: auto, hex, or raw")
	framingName := flagSet.String("framing", "2byte-binary", "length framing: 2byte-binary, 4digit-ascii, or none")
	timeout := flagSet.Duration("timeout", 5*time.Second, "connect/read deadline (e.g. 5s, 500ms)")
	format := flagSet.String("format", "describe", "output format: describe or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	unsafe := flagSet.Bool("unsafe", false, "show raw PAN, track, PIN, and private-field data (default: masked)")
	expectMTI := flagSet.String("expect-mti", "", "assert the response MTI equals VALUE; mismatch exits non-zero")
	var expectFields multiFlag
	flagSet.Var(&expectFields, "expect-field", "assert a response field equals a value, PATH=VALUE (repeatable, e.g. --expect-field 39=00)")
	dryRun := flagSet.Bool("dry-run", false, "build and show the framed request without connecting or sending")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "Send an ISO8583 message over TCP and decode the single response.")
		writeLine(flagSet.Output(), "Usage: iso8583tool send HOST:PORT [MESSAGE|-] [--raw MESSAGE] [--framing 2byte-binary|4digit-ascii|none] [--timeout DURATION] [--dry-run] [--expect-mti VALUE] [--expect-field PATH=VALUE ...] [--encoding auto|hex|raw] [--format describe|json] [--unsafe] [--spec NAME|PATH] [--config PATH] [--color auto|always|never]")
		writeLine(flagSet.Output(), "MESSAGE is a JSON document (packed with the active spec) or a packed hex/raw message; reads from stdin when MESSAGE is '-' or omitted. --raw takes an inline message (JSON or hex/raw) instead of a file argument.")
		writeLine(flagSet.Output(), "--dry-run packs and frames the request, prints what would be sent, and exits without opening a connection (useful for verifying a message before a real run).")
		writeLine(flagSet.Output(), "--expect-mti / --expect-field assert against the decoded, unmasked canonical response values; any mismatch prints a deterministic error and exits non-zero.")
		writeLine(flagSet.Output(), "Cardholder data is masked by default; pass --unsafe to show raw values.")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	address := strings.TrimSpace(flagSet.Arg(0))
	if address == "" || flagSet.NArg() > 2 {
		flagSet.Usage()
		return 1
	}
	messageTarget := flagSet.Arg(1)

	if *format != "describe" && *format != "json" {
		writef(a.stderr, "unsupported format %q\n", *format)
		return 1
	}
	framing, err := service.ParseFraming(*framingName)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}
	expectations, err := parseFieldExpectations(expectFields)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}
	mode, err := resolveColorMode(*color, *noColor)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	ctx, err := a.loadContext(*specName, *configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	payload, err := a.readSendPayload(messageTarget, *raw, *encoding, ctx)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	// Build the request view before sending so a message that does not decode
	// under the active spec is reported as a decode failure, not silently sent.
	reqView, err := service.ViewMessage(payload, ctx.spec.MessageSpec, ctx.catalog, "json", nil, render.NewPalette(false), *unsafe)
	if err != nil {
		writeLine(a.stderr, fmt.Errorf("decode request with %s: %w", ctx.specLabel, err))
		return 1
	}

	// Validate the address before any connection so a malformed HOST:PORT fails
	// the same way whether or not --dry-run is set.
	if err := service.ValidateAddress(address); err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	if *dryRun {
		return a.runSendDryRun(address, framing, *timeout, *format, ctx.specLabel, payload, reqView, *expectMTI, expectations, *unsafe, a.palette(mode, *format))
	}

	result, err := service.SendMessage(service.SendRequest{
		Address: address,
		Payload: payload,
		Framing: framing,
		Timeout: *timeout,
	})
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	respView, err := service.ViewMessage(result.Response, ctx.spec.MessageSpec, ctx.catalog, "json", nil, render.NewPalette(false), *unsafe)
	if err != nil {
		writeLine(a.stderr, fmt.Errorf("decode response from %s with %s: %w", result.RemoteAddr, ctx.specLabel, err))
		return 1
	}

	// Assertions run against the decoded, unmasked canonical response values, so
	// an expectation on a sensitive field still matches its real value even though
	// the printed output below masks it.
	failures, err := service.CheckExpectations(ctx.spec.MessageSpec, result.Response, *expectMTI, expectations)
	if err != nil {
		writeLine(a.stderr, fmt.Errorf("evaluate expectations against %s: %w", result.RemoteAddr, err))
		return 1
	}

	if *format == "json" {
		if code := a.printSendJSON(result, string(framing), *timeout, payload, reqView, respView, *unsafe); code != 0 {
			return code
		}
	} else {
		a.printSendDescribe(result, string(framing), *timeout, ctx.specLabel, reqView, respView, a.palette(mode, *format))
	}

	// Print the exchange first (so a failing run still shows the response), then
	// report any unmet expectations on stderr and exit non-zero.
	if len(failures) > 0 {
		writeLine(a.stderr, "send expectation failed:")
		for _, f := range failures {
			writef(a.stderr, "  %s\n", f.String())
		}
		return 1
	}
	return 0
}

// parseFieldExpectations turns repeated --expect-field PATH=VALUE flags into
// structured expectations, rejecting any entry without a "=" or an empty path.
func parseFieldExpectations(raw []string) ([]service.FieldExpectation, error) {
	expectations := make([]service.FieldExpectation, 0, len(raw))
	for _, entry := range raw {
		pathPart, valuePart, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --expect-field %q (want PATH=VALUE)", entry)
		}
		path := strings.TrimSpace(pathPart)
		if path == "" {
			return nil, fmt.Errorf("invalid --expect-field %q (empty field path)", entry)
		}
		expectations = append(expectations, service.FieldExpectation{
			Path:  path,
			Value: strings.TrimSpace(valuePart),
		})
	}
	return expectations, nil
}

// readSendPayload resolves the message bytes to put on the wire. A JSON document
// is packed with the active spec (like convert); a packed hex/raw message is
// sent verbatim after decoding the input encoding.
func (a *App) readSendPayload(target, raw, encoding string, ctx resolvedContext) ([]byte, error) {
	source, err := messageio.ReadSource(target, raw, a.inputStdin(target, raw))
	if err != nil {
		return nil, err
	}
	if messageio.LooksLikeJSON(source) {
		doc, err := messageio.ParseDocument(source)
		if err != nil {
			return nil, err
		}
		result, err := service.WriteMessage(doc, ctx.spec.MessageSpec)
		if err != nil {
			return nil, err
		}
		return result.Raw, nil
	}
	decoded, _, err := resolveInput(source, encoding)
	return decoded, err
}

func (a *App) printSendDescribe(result service.SendResult, framing string, timeout time.Duration, specLabel string, req, resp service.ViewResult, pal render.Palette) {
	writef(a.stdout, "%s %s\n", pal.Dim("Sent to:"), pal.Bold(result.RemoteAddr))
	writef(a.stdout, "%s %s\n", pal.Dim("Framing:"), pal.Cyan(framing))
	writef(a.stdout, "%s %s\n", pal.Dim("Spec:"), pal.Bold(specLabel))
	writef(a.stdout, "%s %s\n", pal.Dim("Timeout:"), pal.Cyan(timeout.String()))
	writef(a.stdout, "%s %d  %s %d  %s %s\n",
		pal.Dim("Sent bytes:"), result.SentBytes,
		pal.Dim("Received bytes:"), result.ReceivedBytes,
		pal.Dim("RTT:"), pal.Cyan(result.RTT.Round(time.Microsecond).String()))

	a.printSendSide("Request", req, pal)
	a.printSendSide("Response", resp, pal)
}

// printSendSide prints the one-line summary and decoded fields for one side of
// the exchange, mirroring the safe (masked) view the view command produces.
func (a *App) printSendSide(title string, view service.ViewResult, pal render.Palette) {
	writef(a.stdout, "\n%s\n", pal.BoldCyan(title+":"))
	if view.Summary != "" {
		writef(a.stdout, "  %s %s\n", pal.Dim("Summary:"), pal.Cyan(view.Summary))
	}
	if len(view.Decoded) == 0 {
		return
	}
	for _, d := range view.Decoded {
		writef(a.stdout, "  %s = %s  %s\n",
			pal.Green(d.Path), pal.Yellow(render.SanitizeControl(d.Value)), pal.Cyan("→ "+d.Meaning))
	}
}

func (a *App) printSendJSON(result service.SendResult, framing string, timeout time.Duration, payload []byte, req, resp service.ViewResult, unsafe bool) int {
	// The packed wire bytes carry the PAN, track, and PIN in the clear, so the
	// raw hex is withheld by default (it would defeat the masking applied to the
	// request_view/response_view) and only included with --unsafe. The byte
	// counts are not sensitive and always stay.
	type payloadJSON struct {
		Hex   string `json:"hex,omitempty"`
		Bytes int    `json:"bytes"`
	}
	requestPayload := payloadJSON{Bytes: len(payload)}
	responsePayload := payloadJSON{Bytes: len(result.Response)}
	if unsafe {
		requestHex, err := messageio.EncodeOutput(payload, "hex")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		responseHex, err := messageio.EncodeOutput(result.Response, "hex")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		requestPayload.Hex = string(requestHex)
		responsePayload.Hex = string(responseHex)
	}
	out := struct {
		RemoteAddr    string          `json:"remote_addr"`
		Framing       string          `json:"framing"`
		Timeout       string          `json:"timeout"`
		RTTms         float64         `json:"rtt_ms"`
		SentBytes     int             `json:"sent_bytes"`
		ReceivedBytes int             `json:"received_bytes"`
		Request       payloadJSON     `json:"request"`
		Response      payloadJSON     `json:"response"`
		RequestView   json.RawMessage `json:"request_view"`
		ResponseView  json.RawMessage `json:"response_view"`
	}{
		RemoteAddr:    result.RemoteAddr,
		Framing:       framing,
		Timeout:       timeout.String(),
		RTTms:         float64(result.RTT.Microseconds()) / 1000.0,
		SentBytes:     result.SentBytes,
		ReceivedBytes: result.ReceivedBytes,
		Request:       requestPayload,
		Response:      responsePayload,
		RequestView:   json.RawMessage(req.Body),
		ResponseView:  json.RawMessage(resp.Body),
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}
	writef(a.stdout, "%s\n", data)
	return 0
}

// runSendDryRun packs and frames the request, reports what would be sent, and
// returns without opening a connection. Expectations cannot be evaluated because
// no response is received, so combining them with --dry-run is rejected rather
// than silently ignored.
func (a *App) runSendDryRun(address string, framing service.Framing, timeout time.Duration, format, specLabel string, payload []byte, reqView service.ViewResult, expectMTI string, expectations []service.FieldExpectation, unsafe bool, pal render.Palette) int {
	if strings.TrimSpace(expectMTI) != "" || len(expectations) > 0 {
		writeLine(a.stderr, errors.New("--expect-mti/--expect-field cannot be used with --dry-run: no response is received to assert against"))
		return 1
	}
	framed, err := framing.Encode(payload)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}
	if format == "json" {
		return a.printSendDryRunJSON(address, string(framing), timeout, payload, framed, reqView, unsafe)
	}
	a.printSendDryRunDescribe(address, string(framing), specLabel, framed, reqView, pal)
	return 0
}

func (a *App) printSendDryRunDescribe(address, framing, specLabel string, framed []byte, req service.ViewResult, pal render.Palette) {
	writef(a.stdout, "%s\n", pal.BoldCyan("Dry run (no message sent)"))
	writef(a.stdout, "%s %s\n", pal.Dim("Target:"), pal.Bold(address))
	writef(a.stdout, "%s %s\n", pal.Dim("Framing:"), pal.Cyan(framing))
	writef(a.stdout, "%s %s\n", pal.Dim("Spec:"), pal.Bold(specLabel))
	writef(a.stdout, "%s %d\n", pal.Dim("Would send bytes:"), len(framed))
	a.printSendSide("Request", req, pal)
}

func (a *App) printSendDryRunJSON(address, framing string, timeout time.Duration, payload, framed []byte, req service.ViewResult, unsafe bool) int {
	// Mirror the live JSON shape: byte counts always stay; the raw wire hex (which
	// carries PAN/track/PIN in the clear) is withheld unless --unsafe is set.
	type payloadJSON struct {
		Hex   string `json:"hex,omitempty"`
		Bytes int    `json:"bytes"`
	}
	requestPayload := payloadJSON{Bytes: len(payload)}
	if unsafe {
		requestHex, err := messageio.EncodeOutput(payload, "hex")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		requestPayload.Hex = string(requestHex)
	}
	out := struct {
		DryRun         bool            `json:"dry_run"`
		Target         string          `json:"target"`
		Framing        string          `json:"framing"`
		Timeout        string          `json:"timeout"`
		WouldSendBytes int             `json:"would_send_bytes"`
		Request        payloadJSON     `json:"request"`
		RequestView    json.RawMessage `json:"request_view"`
	}{
		DryRun:         true,
		Target:         address,
		Framing:        framing,
		Timeout:        timeout.String(),
		WouldSendBytes: len(framed),
		Request:        requestPayload,
		RequestView:    json.RawMessage(req.Body),
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}
	writef(a.stdout, "%s\n", data)
	return 0
}

func (a *App) runValidate(args []string) int {
	flagSet := newFlagSet("validate", a.stderr)
	specName := flagSet.String("spec", "", "spec preset or JSON spec path")
	configPath := flagSet.String("config", "", "path to a JSON config (defaults + extension catalog)")
	raw := flagSet.String("raw", "", "inline input message instead of a file argument")
	encoding := flagSet.String("encoding", "auto", "input encoding: auto, hex, or raw")
	format := flagSet.String("format", "text", "output format: text or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	strict := flagSet.Bool("strict", false, "apply best-effort BASE I message-class semantic checks (required/recommended fields)")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "Validate that a message can be unpacked and highlight extension-field strategy.")
		writeLine(flagSet.Output(), "Usage: iso8583tool validate [MESSAGE|-] [--raw MESSAGE] [--strict] [--spec NAME|PATH] [--config PATH] [--encoding auto|hex|raw] [--format text|json] [--color auto|always|never]")
		writeLine(flagSet.Output(), "Reads from stdin when MESSAGE is '-' or omitted; --raw takes an inline message instead.")
		writeLine(flagSet.Output(), "Without --strict, validate only checks that the message unpacks; --strict adds heuristic per-MTI field checks.")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	target := flagSet.Arg(0)
	if flagSet.NArg() > 1 {
		flagSet.Usage()
		return 1
	}

	mode, err := resolveColorMode(*color, *noColor)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	ctx, err := a.loadContext(*specName, *configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, err := a.readMessageInput(target, *raw, *encoding)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	report := service.ValidateMessage(input, ctx.spec.MessageSpec, ctx.specLabel, ctx.catalog, *strict)
	switch *format {
	case "text":
		a.printValidationReport(report, a.palette(mode, *format))
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		writef(a.stdout, "%s\n", data)
	default:
		writef(a.stderr, "unsupported format %q\n", *format)
		return 1
	}

	if report.HasErrors() {
		return 1
	}
	return 0
}

func (a *App) runDoctor(args []string) int {
	flagSet := newFlagSet("doctor", a.stderr)
	raw := flagSet.String("raw", "", "inline input message instead of a file argument")
	encoding := flagSet.String("encoding", "auto", "input encoding: auto, hex, or raw")
	format := flagSet.String("format", "text", "output format: text or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "Detect which built-in spec preset fits a message.")
		writeLine(flagSet.Output(), "Usage: iso8583tool doctor [MESSAGE|-] [--raw MESSAGE] [--encoding auto|hex|raw] [--format text|json] [--color auto|always|never]")
		writeLine(flagSet.Output(), "Reads from stdin when MESSAGE is '-' or omitted; --raw takes an inline message instead.")
		writeLine(flagSet.Output(), "The input encoding is auto-detected (hex text vs raw bytes); override with --encoding.")
		writeLine(flagSet.Output(), "Tries every preset and recommends the best fit; confirm the result with view.")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	target := flagSet.Arg(0)
	if flagSet.NArg() > 1 {
		flagSet.Usage()
		return 1
	}

	mode, err := resolveColorMode(*color, *noColor)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, usedEncoding, err := a.readDoctorInput(target, *raw, *encoding)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	diag := service.DiagnoseSpec(input)
	diag.InputEncoding = usedEncoding
	switch *format {
	case "text":
		a.printSpecDiagnosis(diag, target, a.palette(mode, *format))
	case "json":
		data, err := json.MarshalIndent(diag, "", "  ")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		writef(a.stdout, "%s\n", data)
	default:
		writef(a.stderr, "unsupported format %q\n", *format)
		return 1
	}

	// No preset unpacking the message is the actionable failure: exit non-zero
	// so a script can branch on it.
	if diag.Recommended == "" {
		return 1
	}
	return 0
}

// readDoctorInput reads the message bytes for doctor, auto-detecting the input
// encoding when --encoding is "auto" (the default).
func (a *App) readDoctorInput(target, raw, encoding string) ([]byte, string, error) {
	source, err := messageio.ReadSource(target, raw, a.inputStdin(target, raw))
	if err != nil {
		return nil, "", err
	}
	return resolveInput(source, encoding)
}

// readMessageInput reads a message from a file, stdin, or inline value and
// decodes it, auto-detecting the input encoding when --encoding is "auto" (the
// default for view/validate/diff). An explicit hex/raw is honored as-is.
func (a *App) readMessageInput(target, raw, encoding string) ([]byte, error) {
	source, err := messageio.ReadSource(target, raw, a.inputStdin(target, raw))
	if err != nil {
		return nil, err
	}
	decoded, _, err := resolveInput(source, encoding)
	return decoded, err
}

// resolveInput decodes raw source bytes into message bytes and reports the
// encoding it used. An explicit hex/raw is honored as-is; "auto" detects the
// encoding by fit so an unknown capture (a raw *.bin or an all-numeric raw ASCII
// message) no longer has to be hinted with --encoding raw.
func resolveInput(source []byte, encoding string) ([]byte, string, error) {
	enc := strings.ToLower(strings.TrimSpace(encoding))
	if enc != "auto" {
		decoded, err := messageio.DecodeInput(source, encoding)
		return decoded, enc, err
	}
	decoded, used := autoDecodeMessage(source)
	return decoded, used, nil
}

// autoDecodeMessage decides whether source is hex text or raw bytes and returns
// the message bytes plus the encoding it used. Non-hex bytes are raw. Valid hex
// is decoded, unless a built-in preset fits the raw reading strictly better — an
// all-numeric raw ASCII capture is, byte for byte, a valid even-length hex
// string, so fit, not shape, decides. Ties keep the hex reading.
func autoDecodeMessage(source []byte) ([]byte, string) {
	if !messageio.LooksLikeHex(source) {
		return source, "raw"
	}
	decoded, err := messageio.DecodeInput(source, "hex")
	if err != nil {
		return source, "raw"
	}
	if service.DiagnoseSpec(source).BestScore() > service.DiagnoseSpec(decoded).BestScore() {
		return source, "raw"
	}
	return decoded, "hex"
}

func (a *App) printSpecDiagnosis(diag service.SpecDiagnosis, target string, pal render.Palette) {
	unit := "bytes"
	if diag.Bytes == 1 {
		unit = "byte"
	}
	encNote := ""
	if diag.InputEncoding != "" {
		encNote = fmt.Sprintf(" (%s input)", diag.InputEncoding)
	}
	writef(a.stdout, "%s inspected %d %s%s\n", pal.Dim("Doctor:"), diag.Bytes, unit, encNote)
	switch {
	case diag.Recommended == "" && diag.LikelyMalformed:
		writeLine(a.stdout, pal.Red("This message appears truncated or malformed."))
		writeLine(a.stdout, pal.Dim("A field length runs past the available bytes under every preset; re-capture the full message. If the layout is custom, pass a moov-io/iso8583 JSON spec with --spec PATH."))
	case diag.Recommended == "":
		writeLine(a.stdout, pal.Red("No built-in preset could unpack this message."))
		writeLine(a.stdout, pal.Dim("It may use a custom layout; pass a moov-io/iso8583 JSON spec with --spec PATH."))
	case diag.Ambiguous:
		// More than one preset fits equally well; present them all so the default
		// is not mistaken for the single right answer.
		writef(a.stdout, "%s %s\n", pal.Dim("Recommended:"), pal.BoldGreen("--spec "+strings.Join(diag.Recommendations, " or --spec ")))
		writeLine(a.stdout, pal.Yellow("Note: more than one preset fits equally well; confirm by eye."))
	default:
		writef(a.stdout, "%s %s\n", pal.Dim("Recommended:"), pal.BoldGreen("--spec "+diag.Recommended))
	}

	// Every preset tied at the best score is "recommended"; on a tie the default
	// must not be presented as the single answer.
	recommended := recommendedSet(diag)

	writef(a.stdout, "\n%s\n", pal.BoldCyan("Candidates:"))
	for _, c := range diag.Candidates {
		name := fmt.Sprintf("%-18s", c.Preset)
		switch {
		case recommended[c.Preset]:
			writef(a.stdout, "- %s %s  %s\n", pal.Green(name), pal.BoldGreen("recommended"), pal.Dim(c.Detail))
		case c.Unpacks:
			writef(a.stdout, "- %s %s         %s\n", pal.Green(name), pal.Cyan("fits"), pal.Dim(c.Detail))
		default:
			writef(a.stdout, "- %s %s           %s\n", name, pal.Red("no"), pal.Dim(c.Detail))
		}
	}

	confirmPresets := diag.Recommendations
	if len(confirmPresets) == 0 && diag.Recommended != "" {
		confirmPresets = []string{diag.Recommended}
	}
	if len(confirmPresets) > 0 {
		encFlag := ""
		if diag.InputEncoding == "raw" {
			encFlag = " --encoding raw"
		}
		if len(confirmPresets) == 1 {
			writef(a.stdout, "\n%s %s\n", pal.Dim("Confirm with:"), confirmCommand(target, confirmPresets[0], encFlag))
		} else {
			writef(a.stdout, "\n%s\n", pal.Dim("Confirm with:"))
			for _, preset := range confirmPresets {
				writef(a.stdout, "  %s\n", confirmCommand(target, preset, encFlag))
			}
		}
	}
}

// recommendedSet returns the set of presets tied at the best score. It falls
// back to the single Recommended when no explicit tie list is present.
func recommendedSet(diag service.SpecDiagnosis) map[string]bool {
	set := make(map[string]bool, len(diag.Recommendations)+1)
	for _, r := range diag.Recommendations {
		set[r] = true
	}
	if len(set) == 0 && diag.Recommended != "" {
		set[diag.Recommended] = true
	}
	return set
}

// confirmCommand builds a copy-pasteable `view` command for a doctor candidate.
// The target is shell-quoted, and a "-"-prefixed filename is placed after a "--"
// separator (with the flags before it) so it is not parsed as an option.
func confirmCommand(target, preset, encFlag string) string {
	hintTarget := strings.TrimSpace(target)
	if hintTarget == "" || hintTarget == "-" {
		return fmt.Sprintf("iso8583tool view --spec %s%s MESSAGE", preset, encFlag)
	}
	sep := ""
	if strings.HasPrefix(hintTarget, "-") {
		sep = "-- "
	}
	return fmt.Sprintf("iso8583tool view --spec %s%s %s%s", preset, encFlag, sep, shellQuoteArg(hintTarget))
}

// shellQuoteArg returns s safe to paste as a single shell argument: an unquoted
// value when it contains only portable filename characters, otherwise a
// single-quoted value with embedded single quotes escaped.
func shellQuoteArg(s string) string {
	safe := s != ""
	for _, r := range s {
		if !isPortableArgChar(r) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// isPortableArgChar reports whether r can appear in a shell argument without
// quoting (a conservative POSIX-portable filename set).
func isPortableArgChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '_' || r == '-' || r == '.' || r == '/' || r == '+' || r == '=':
		return true
	default:
		return false
	}
}

func (a *App) runSpecs(args []string) int {
	flagSet := newFlagSet("specs", a.stderr)
	format := flagSet.String("format", "text", "output format: text or json")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "List the built-in spec presets selectable with --spec.")
		writeLine(flagSet.Output(), "Usage: iso8583tool specs [--format text|json]")
		writeLine(flagSet.Output(), "Any moov-io/iso8583 JSON spec path also works as --spec.")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	if flagSet.NArg() > 0 {
		flagSet.Usage()
		return 1
	}

	presets := basei.Presets()
	switch *format {
	case "text":
		writeLine(a.stdout, "Built-in spec presets (use with --spec NAME):")
		for _, p := range presets {
			name := p.Name
			if p.Default {
				name += " (default)"
			}
			writef(a.stdout, "\n- %s\n", name)
			writef(a.stdout, "  %s\n", p.Title)
			writef(a.stdout, "  encoding: %s\n", p.Encoding)
			writef(a.stdout, "  %s\n", p.Summary)
		}
		writeLine(a.stdout, "")
		writeLine(a.stdout, "Any moov-io/iso8583 JSON spec path also works as --spec.")
		writeLine(a.stdout, "Unsure which one a capture uses? Run \"iso8583tool doctor MESSAGE\".")
	case "json":
		type specInfo struct {
			Name     string `json:"name"`
			Title    string `json:"title"`
			Encoding string `json:"encoding"`
			Summary  string `json:"summary"`
			Default  bool   `json:"default"`
		}
		out := make([]specInfo, 0, len(presets))
		for _, p := range presets {
			out = append(out, specInfo{
				Name:     p.Name,
				Title:    p.Title,
				Encoding: p.Encoding,
				Summary:  p.Summary,
				Default:  p.Default,
			})
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		writef(a.stdout, "%s\n", data)
	default:
		writef(a.stderr, "unsupported format %q\n", *format)
		return 1
	}
	return 0
}

func (a *App) runSample(args []string) int {
	flagSet := newFlagSet("sample", a.stderr)
	format := flagSet.String("format", "json", "sample format: json or hex")
	outputPath := flagSet.String("output", "", "path to write the exported sample")
	flagSet.Usage = func() {
		writeLine(flagSet.Output(), "List or export built-in BASE I starter samples.")
		writeLine(flagSet.Output(), "Usage: iso8583tool sample [NAME] [--format json|hex] [--output PATH]")
		printFlagDefaults(flagSet.Output(), flagSet)
	}
	if code, ok := a.parseFlags(flagSet, args); !ok {
		return code
	}
	if flagSet.NArg() > 1 {
		flagSet.Usage()
		return 1
	}
	name := flagSet.Arg(0)

	if strings.TrimSpace(name) == "" {
		writeLine(a.stdout, "Available samples:")
		for _, sample := range basei.StarterSamples() {
			writef(a.stdout, "- %s: %s\n", sample.Name, sample.Summary)
		}
		return 0
	}

	sample, ok := basei.LookupSample(strings.TrimSpace(name))
	if !ok {
		writef(a.stderr, "unknown sample %q\n", name)
		return 1
	}

	data, err := a.renderSample(sample, *format)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	if strings.TrimSpace(*outputPath) != "" {
		if err := os.WriteFile(filepath.Clean(*outputPath), data, 0o600); err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		writef(a.stdout, "Wrote sample %s to %s\n", sample.Name, *outputPath)
		return 0
	}

	_, _ = a.stdout.Write(data)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		_, _ = io.WriteString(a.stdout, "\n")
	}
	return 0
}

func (a *App) renderSample(sample basei.Sample, format string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		data, err := json.MarshalIndent(sample.Document, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	case "hex":
		result, err := service.WriteMessage(sample.Document, basei.StarterMessageSpec())
		if err != nil {
			return nil, err
		}
		data, err := messageio.EncodeOutput(result.Raw, "hex")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported sample format %q", format)
	}
}

func (a *App) loadContext(specName, configPath string) (resolvedContext, error) {
	cfg := config.Default()
	baseDir := a.workDir
	if path := strings.TrimSpace(configPath); path != "" {
		loaded, err := config.Load(path)
		if err != nil {
			return resolvedContext{}, err
		}
		cfg = loaded
		baseDir = filepath.Dir(path)
	}
	customSpec := false
	if spec := strings.TrimSpace(specName); spec != "" {
		cfg.Spec = spec
		baseDir = a.workDir
		// A custom JSON spec (--spec PATH) is not BASE I; only a built-in preset
		// name is described by the built-in extension catalog.
		_, isPreset := basei.LookupPreset(spec)
		customSpec = !isPreset
	}

	specResult, err := messagespec.Load(baseDir, cfg)
	if err != nil {
		return resolvedContext{}, err
	}

	// The extension catalog applies to built-in BASE I presets, or whatever a
	// --config explicitly provides. A bare custom --spec PATH gets no catalog, so
	// its fields are described by the spec itself, not by BASE I field names.
	catalog := cfg.Catalog()
	if customSpec && strings.TrimSpace(configPath) == "" {
		catalog = basei.ExtensionCatalog{}
	}

	return resolvedContext{
		specLabel: specResult.Label,
		spec:      specResult,
		catalog:   catalog,
	}, nil
}

// inputStdin returns the stdin reader to use, or nil when no file/raw is given
// but stdin is an interactive terminal (so we error instead of blocking).
func (a *App) inputStdin(target, raw string) io.Reader {
	if target == "-" {
		return a.stdin
	}
	if strings.TrimSpace(target) == "" && strings.TrimSpace(raw) == "" {
		if f, ok := a.stdin.(*os.File); ok && render.IsTerminal(f) {
			return nil
		}
	}
	return a.stdin
}

// palette resolves the color mode against stdout. JSON output is never
// colorized so it stays machine-parseable.
func (a *App) palette(mode, format string) render.Palette {
	if format == "json" {
		return render.NewPalette(false)
	}
	out, _ := a.stdout.(*os.File)
	return render.NewPalette(render.ResolveColor(mode, out))
}

// resolveColorMode validates the --color value against the documented set and
// combines it with --no-color. An unknown value is a hard error rather than a
// silent fallback to "no color".
func resolveColorMode(mode string, noColor bool) (string, error) {
	switch mode {
	case "auto", "always", "never":
	default:
		return "", fmt.Errorf("invalid --color %q (want auto, always, or never)", mode)
	}
	if noColor {
		return "never", nil
	}
	return mode, nil
}

func strategyColor(pal render.Palette, strategy string) string {
	switch strategy {
	case "tlv":
		return pal.Cyan(strategy)
	case "opaque":
		return pal.Yellow(strategy)
	case "positional":
		return pal.Blue(strategy)
	case "bitmap":
		return pal.Magenta(strategy)
	default:
		return strategy
	}
}

func (a *App) printRootHelp(w io.Writer) {
	writeLine(w, "BASE I oriented ISO8583 viewer, converter, and validator.")
	writeLine(w, "")
	writeLine(w, "Usage:")
	writeLine(w, "  iso8583tool <command> [arguments] [flags]")
	writeLine(w, "")
	writeLine(w, "Commands:")
	writeLine(w, "  view       Unpack and inspect a message")
	writeLine(w, "  diff       Compare two messages field by field")
	writeLine(w, "  redact     Mask sensitive fields for safe sharing")
	writeLine(w, "  convert    Convert between a packed message and a JSON document")
	writeLine(w, "  send       Send a message over TCP and decode the response")
	writeLine(w, "  validate   Check a message against the configured spec")
	writeLine(w, "  doctor     Detect which built-in spec preset fits a message")
	writeLine(w, "  specs      List the built-in spec presets")
	writeLine(w, "  sample     List or export built-in BASE I starter samples")
	writeLine(w, "  version    Print the version")
	writeLine(w, "  help       Show command help")
	writeLine(w, "")
	writeLine(w, "Messages can be read from a file, '-', or stdin. Use --spec for a preset")
	writeLine(w, "or JSON spec path, and --config PATH for extension catalogs/default overrides.")
	writeLine(w, "Not sure which spec a capture uses? Run \"iso8583tool doctor MESSAGE\".")
}

func (a *App) printValidationReport(report service.ValidationReport, pal render.Palette) {
	if report.HasErrors() {
		writef(a.stdout, "%s %s\n", pal.Dim("Validation:"), pal.Red("failed"))
	} else {
		writef(a.stdout, "%s %s\n", pal.Dim("Validation:"), pal.BoldGreen("ok"))
	}
	if report.Hint != "" {
		writef(a.stdout, "%s %s\n", pal.Dim("Hint:"), pal.Yellow(report.Hint))
	}
	writef(a.stdout, "%s %s\n", pal.Dim("Spec:"), pal.Bold(report.Spec))
	if report.MTI != "" {
		line := fmt.Sprintf("%s %s", pal.Dim("MTI:"), pal.BoldGreen(report.MTI))
		if report.MTIDescription != "" {
			line += "  " + pal.Cyan("→ "+report.MTIDescription)
		}
		writeLine(a.stdout, line)
	}
	if report.Summary != "" {
		writef(a.stdout, "%s %s\n", pal.Dim("Summary:"), pal.Cyan(report.Summary))
	}
	// The MTI is shown above, so it is skipped here. Only print the section when
	// at least one non-MTI field was decoded; otherwise the heading would stand
	// alone with nothing under it.
	decodedFields := make([]service.DecodedField, 0, len(report.Decoded))
	for _, d := range report.Decoded {
		if d.Path != "0" {
			decodedFields = append(decodedFields, d)
		}
	}
	if len(decodedFields) > 0 {
		writef(a.stdout, "\n%s\n", pal.BoldCyan("Decoded Fields:"))
		for _, d := range decodedFields {
			writef(a.stdout, "- %s = %s  %s\n",
				pal.Green(d.Path), pal.Yellow(render.SanitizeControl(d.Value)), pal.Cyan("→ "+d.Meaning))
		}
	}
	if len(report.Issues) > 0 {
		writef(a.stdout, "\n%s\n", pal.Bold("Issues:"))
		for _, issue := range report.Issues {
			sev := severityColor(pal, string(issue.Severity))
			if issue.Path == "" {
				writef(a.stdout, "- [%s] %s\n", sev, issue.Message)
				continue
			}
			writef(a.stdout, "- [%s] %s: %s\n", sev, issue.Path, issue.Message)
		}
	}
	if len(report.Extensions) > 0 {
		writef(a.stdout, "\n%s\n", pal.BoldCyan("Extension Field Strategy:"))
		for _, ext := range report.Extensions {
			writef(a.stdout, "- %s %s [%s]: %s\n",
				pal.Green(fmt.Sprintf("F%d", ext.Field)), ext.Name,
				strategyColor(pal, ext.Strategy), pal.Dim(ext.Note))
		}
	}
	if len(report.UnknownTags) > 0 {
		writef(a.stdout, "\n%s\n", pal.Yellow("Unknown TLV Tags:"))
		for _, tag := range report.UnknownTags {
			writef(a.stdout, "- %s: %s\n", pal.Yellow(tag.Path), tag.Raw)
		}
	}
}

func severityColor(pal render.Palette, severity string) string {
	switch severity {
	case "error":
		return pal.Red(severity)
	case "warning":
		return pal.Yellow(severity)
	default:
		return pal.Dim(severity)
	}
}

// multiFlag collects a repeatable string flag (e.g. --filter).
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// boolFlag matches the stdlib interface implemented by flags that take no value.
type boolFlag interface{ IsBoolFlag() bool }

// reorderArgs reorders args so positional arguments may appear before or after
// flags. The set of value-taking flags is derived from flagSet itself (every
// non-bool flag), so it can never drift from the flags a command registers.
func reorderArgs(flagSet *flag.FlagSet, args []string) []string {
	valueFlags := map[string]bool{}
	flagSet.VisitAll(func(f *flag.Flag) {
		if bf, ok := f.Value.(boolFlag); !ok || !bf.IsBoolFlag() {
			valueFlags[f.Name] = true
		}
	})
	return reorder(args, valueFlags)
}

// reorder moves flags ahead of positional arguments so the stdlib flag parser
// (which stops at the first non-flag) accepts both "view msg --json" and
// "view --json msg". valueFlags lists flags that consume the next token.
//
// Positionals are re-emitted after a literal "--" so the flag parser treats them
// as operands even when they start with "-" (e.g. a file named "-response.hex"
// passed as "view -- -response.hex"). Everything after the first "--" in the
// input is positional, mirroring standard end-of-options behavior.
func reorder(args []string, valueFlags map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags = append(flags, arg)
			name := strings.TrimLeft(arg, "-")
			if !strings.Contains(name, "=") && valueFlags[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	if len(positionals) == 0 {
		return flags
	}
	result := make([]string, 0, len(flags)+1+len(positionals))
	result = append(result, flags...)
	result = append(result, "--")
	result = append(result, positionals...)
	return result
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	flagSet := flag.NewFlagSet(name, flag.ContinueOnError)
	flagSet.SetOutput(stderr)
	return flagSet
}

func parseArgs(flagSet *flag.FlagSet, args []string) (int, bool) {
	if err := flagSet.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 1, false
	}
	return 0, true
}

// helpRequested reports whether args explicitly ask for help (a standalone -h or
// --help before any "--" terminator).
func helpRequested(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

// parseFlags reorders args and parses them. When help is explicitly requested it
// prints the command usage to stdout and returns success (a successful help
// request is not an error); flag-parsing errors still go to stderr with a
// non-zero code. Command Usage funcs write to flagSet.Output(), which this sets
// to stdout for the help case.
func (a *App) parseFlags(flagSet *flag.FlagSet, args []string) (int, bool) {
	if helpRequested(args) {
		flagSet.SetOutput(a.stdout)
		flagSet.Usage()
		return 0, false
	}
	return parseArgs(flagSet, reorderArgs(flagSet, args))
}

func printFlagDefaults(w io.Writer, flagSet *flag.FlagSet) {
	flagSet.VisitAll(func(f *flag.Flag) {
		writef(w, "  -%s=%s\n      %s\n", f.Name, f.DefValue, f.Usage)
	})
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeLine(w io.Writer, v any) {
	_, _ = fmt.Fprintln(w, v)
}
