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
	case "view", "diff", "redact", "convert", "validate", "doctor", "specs", "sample":
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
		writef(a.stdout, "%s %s\n", pal.Bold(label), string(c.Kind))
		switch c.Kind {
		case service.DiffChanged:
			writef(a.stdout, "%s\n", pal.Red("- "+c.Before))
			writef(a.stdout, "%s\n", pal.Green("+ "+c.After))
		case service.DiffRemoved:
			writef(a.stdout, "%s\n", pal.Red("- "+c.Before))
		case service.DiffAdded:
			writef(a.stdout, "%s\n", pal.Green("+ "+c.After))
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
		writeLine(flagSet.Output(), "Usage: iso8583tool redact [MESSAGE|-] [--raw HEX] [--format json|text] [--spec NAME|PATH] [--config PATH] [--encoding auto|hex|raw] [--color auto|always|never]")
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
		writeLine(flagSet.Output(), "Usage: iso8583tool validate [MESSAGE|-] [--raw HEX] [--strict] [--spec NAME|PATH] [--config PATH] [--encoding auto|hex|raw] [--format text|json] [--color auto|always|never]")
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
		writeLine(flagSet.Output(), "Usage: iso8583tool doctor [MESSAGE|-] [--raw HEX] [--encoding auto|hex|raw] [--format text|json] [--color auto|always|never]")
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

	writef(a.stdout, "\n%s\n", pal.BoldCyan("Candidates:"))
	for _, c := range diag.Candidates {
		name := fmt.Sprintf("%-18s", c.Preset)
		switch {
		case c.Preset == diag.Recommended:
			writef(a.stdout, "- %s %s  %s\n", pal.Green(name), pal.BoldGreen("recommended"), pal.Dim(c.Detail))
		case c.Unpacks:
			writef(a.stdout, "- %s %s         %s\n", pal.Green(name), pal.Cyan("fits"), pal.Dim(c.Detail))
		default:
			writef(a.stdout, "- %s %s           %s\n", name, pal.Red("no"), pal.Dim(c.Detail))
		}
	}

	if diag.Recommended != "" {
		hintTarget := target
		if strings.TrimSpace(hintTarget) == "" || hintTarget == "-" {
			hintTarget = "MESSAGE"
		}
		encFlag := ""
		if diag.InputEncoding == "raw" {
			encFlag = " --encoding raw"
		}
		writef(a.stdout, "\n%s iso8583tool view %s --spec %s%s\n",
			pal.Dim("Confirm with:"), hintTarget, diag.Recommended, encFlag)
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
	if spec := strings.TrimSpace(specName); spec != "" {
		cfg.Spec = spec
		baseDir = a.workDir
	}

	specResult, err := messagespec.Load(baseDir, cfg)
	if err != nil {
		return resolvedContext{}, err
	}

	return resolvedContext{
		specLabel: specResult.Label,
		spec:      specResult,
		catalog:   cfg.Catalog(),
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
	if len(report.Decoded) > 0 {
		writef(a.stdout, "\n%s\n", pal.BoldCyan("Decoded Fields:"))
		for _, d := range report.Decoded {
			if d.Path == "0" {
				continue // MTI already shown above
			}
			writef(a.stdout, "- %s = %s  %s\n",
				pal.Green(d.Path), pal.Yellow(d.Value), pal.Cyan("→ "+d.Meaning))
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
