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
	"sort"
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
		a.printRootHelp()
		return 0
	}

	switch args[0] {
	case "help":
		// "help" alone prints root help; "help <command>" describes a command.
		if len(args) > 1 {
			return a.runHelp(args[1:])
		}
		a.printRootHelp()
		return 0
	case "-h", "--help":
		if len(args) > 1 {
			writef(a.stderr, "%q takes no arguments; use \"iso8583tool help <command>\"\n\n", args[0])
			a.printRootHelp()
			return 1
		}
		a.printRootHelp()
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
	case "sample":
		return a.runSample(args[1:])
	case "version", "-v", "--version":
		if len(args) > 1 {
			writef(a.stderr, "%q takes no arguments\n\n", args[0])
			a.printRootHelp()
			return 1
		}
		writef(a.stdout, "iso8583tool %s\n", resolveVersion())
		return 0
	default:
		writef(a.stderr, "unknown command: %s\n\n", args[0])
		a.printRootHelp()
		return 1
	}
}

func (a *App) runHelp(args []string) int {
	name := args[0]
	rest := args[1:]

	switch name {
	case "view", "diff", "redact", "convert", "validate", "sample":
		forwarded := make([]string, 0, len(args)+1)
		forwarded = append(forwarded, args...)
		forwarded = append(forwarded, "--help")
		return a.Run(forwarded)
	case "help", "version":
		if len(rest) > 0 {
			writef(a.stderr, "%q takes no arguments\n\n", name)
			a.printRootHelp()
			return 1
		}
		a.printRootHelp()
		return 0
	default:
		writef(a.stderr, "unknown command: %s\n\n", name)
		a.printRootHelp()
		return 1
	}
}

func (a *App) runView(args []string) int {
	flagSet := newFlagSet("view", a.stderr)
	configPath := flagSet.String("config", "", "path to a JSON config (spec + extension catalog)")
	raw := flagSet.String("raw", "", "inline input message instead of a file argument")
	encoding := flagSet.String("encoding", "hex", "input encoding: hex or raw")
	format := flagSet.String("format", "describe", "output format: describe or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	var filters multiFlag
	flagSet.Var(&filters, "filter", "only show this field path (repeatable, e.g. --filter 39 --filter 55.9F02)")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Inspect an ISO8583 message with the configured spec.")
		writeLine(a.stderr, "Usage: iso8583tool view [MESSAGE|-] [--config PATH] [--filter PATH ...] [--encoding hex|raw] [--format describe|json] [--color auto|always|never]")
		writeLine(a.stderr, "Reads from stdin when MESSAGE is '-' or omitted.")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, reorder(args, viewValueFlags)); !ok {
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

	ctx, err := a.loadContext(*configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, err := messageio.ReadMessage(target, *raw, *encoding, a.inputStdin(target, *raw))
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	pal := a.palette(mode, *format)
	result, err := service.ViewMessage(input, ctx.spec.MessageSpec, ctx.catalog, *format, filters, pal)
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
	configPath := flagSet.String("config", "", "path to a JSON config (spec + extension catalog)")
	encoding := flagSet.String("encoding", "hex", "input encoding: hex or raw")
	format := flagSet.String("format", "text", "output format: text or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	var filters multiFlag
	flagSet.Var(&filters, "filter", "only compare this field path (repeatable)")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Compare two ISO8583 messages field by field.")
		writeLine(a.stderr, "Usage: iso8583tool diff BEFORE AFTER [--filter PATH ...] [--format text|json] [--config PATH] [--color auto|always|never]")
		writeLine(a.stderr, "Either BEFORE or AFTER may be '-' to read that side from stdin.")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, reorder(args, diffValueFlags)); !ok {
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

	mode, err := resolveColorMode(*color, *noColor)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	ctx, err := a.loadContext(*configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	before, err := messageio.ReadMessage(beforeArg, "", *encoding, a.sideStdin(beforeArg))
	if err != nil {
		writeLine(a.stderr, fmt.Errorf("before: %w", err))
		return 1
	}
	after, err := messageio.ReadMessage(afterArg, "", *encoding, a.sideStdin(afterArg))
	if err != nil {
		writeLine(a.stderr, fmt.Errorf("after: %w", err))
		return 1
	}

	result, err := service.DiffMessages(ctx.spec.MessageSpec, before, after, filters)
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
}

func (a *App) runRedact(args []string) int {
	flagSet := newFlagSet("redact", a.stderr)
	configPath := flagSet.String("config", "", "path to a JSON config (spec + extension catalog)")
	raw := flagSet.String("raw", "", "inline input message instead of a file argument")
	encoding := flagSet.String("encoding", "hex", "input encoding: hex or raw")
	format := flagSet.String("format", "json", "output format: json or text")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Mask cardholder data and secrets so a message can be shared safely.")
		writeLine(a.stderr, "Usage: iso8583tool redact [MESSAGE|-] [--format json|text] [--config PATH] [--encoding hex|raw]")
		writeLine(a.stderr, "Output is a sanitized document for sharing, not a re-packable message.")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, reorder(args, redactValueFlags)); !ok {
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

	ctx, err := a.loadContext(*configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, err := messageio.ReadMessage(target, *raw, *encoding, a.inputStdin(target, *raw))
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
	sort.Strings(keys)
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
	configPath := flagSet.String("config", "", "path to a JSON config (spec + extension catalog)")
	outputPath := flagSet.String("output", "", "path to write the result")
	encoding := flagSet.String("encoding", "hex", "message-side encoding: hex or raw")
	to := flagSet.String("to", "", "force output direction: json or hex (default: auto-detect)")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Convert between a packed BASE I message and a JSON document (auto-detected).")
		writeLine(a.stderr, "Usage: iso8583tool convert [INPUT|-] [--to json|hex] [--output PATH] [--config PATH] [--encoding hex|raw]")
		writeLine(a.stderr, "JSON input is packed to a message; a message is unpacked to a JSON document.")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, reorder(args, convertValueFlags)); !ok {
		return code
	}
	target := flagSet.Arg(0)
	if flagSet.NArg() > 1 {
		flagSet.Usage()
		return 1
	}

	ctx, err := a.loadContext(*configPath)
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
		encoded, err := messageio.EncodeOutput(result.Raw, *encoding)
		if err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		if *encoding == "hex" {
			encoded = append(encoded, '\n')
		}
		out = encoded
		summary = fmt.Sprintf("packed %d fields to %s", result.FieldCount, *encoding)
	case "json": // packed message -> JSON document
		msgRaw, err := messageio.DecodeInput(source, *encoding)
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
	configPath := flagSet.String("config", "", "path to a JSON config (spec + extension catalog)")
	raw := flagSet.String("raw", "", "inline input message instead of a file argument")
	encoding := flagSet.String("encoding", "hex", "input encoding: hex or raw")
	format := flagSet.String("format", "text", "output format: text or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	noColor := flagSet.Bool("no-color", false, "disable color (same as --color never)")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Validate that a message can be unpacked and highlight extension-field strategy.")
		writeLine(a.stderr, "Usage: iso8583tool validate [MESSAGE|-] [--config PATH] [--encoding hex|raw] [--format text|json] [--color auto|always|never]")
		writeLine(a.stderr, "Reads from stdin when MESSAGE is '-' or omitted.")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, reorder(args, validateValueFlags)); !ok {
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

	ctx, err := a.loadContext(*configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, err := messageio.ReadMessage(target, *raw, *encoding, a.inputStdin(target, *raw))
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	report := service.ValidateMessage(input, ctx.spec.MessageSpec, ctx.specLabel, ctx.catalog)
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

func (a *App) runSample(args []string) int {
	flagSet := newFlagSet("sample", a.stderr)
	format := flagSet.String("format", "json", "sample format: json or hex")
	outputPath := flagSet.String("output", "", "path to write the exported sample")
	flagSet.Usage = func() {
		writeLine(a.stderr, "List or export built-in BASE I starter samples.")
		writeLine(a.stderr, "Usage: iso8583tool sample [NAME] [--format json|hex] [--output PATH]")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, reorder(args, sampleValueFlags)); !ok {
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

func (a *App) loadContext(configPath string) (resolvedContext, error) {
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

func (a *App) printRootHelp() {
	writeLine(a.stderr, "BASE I oriented ISO8583 viewer, converter, and validator.")
	writeLine(a.stderr, "")
	writeLine(a.stderr, "Usage:")
	writeLine(a.stderr, "  iso8583tool <command> [arguments] [flags]")
	writeLine(a.stderr, "")
	writeLine(a.stderr, "Commands:")
	writeLine(a.stderr, "  view       Unpack and inspect a message")
	writeLine(a.stderr, "  diff       Compare two messages field by field")
	writeLine(a.stderr, "  redact     Mask sensitive fields for safe sharing")
	writeLine(a.stderr, "  convert    Convert between a packed message and a JSON document")
	writeLine(a.stderr, "  validate   Check a message against the configured spec")
	writeLine(a.stderr, "  sample     List or export built-in BASE I starter samples")
	writeLine(a.stderr, "  version    Print the version")
	writeLine(a.stderr, "  help       Show command help")
	writeLine(a.stderr, "")
	writeLine(a.stderr, "Messages can be read from a file, '-', or stdin. Pass a spec or extension")
	writeLine(a.stderr, "catalog with --config PATH; without it the built-in basei-starter is used.")
}

func (a *App) printValidationReport(report service.ValidationReport, pal render.Palette) {
	if report.HasErrors() {
		writef(a.stdout, "%s %s\n", pal.Dim("Validation:"), pal.Red("failed"))
	} else {
		writef(a.stdout, "%s %s\n", pal.Dim("Validation:"), pal.BoldGreen("ok"))
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
			sev := severityColor(pal, issue.Severity)
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

// Value-taking flags per command, used by reorder so positional arguments may
// appear before or after flags.
var (
	viewValueFlags     = map[string]bool{"config": true, "raw": true, "encoding": true, "format": true, "color": true, "filter": true}
	validateValueFlags = map[string]bool{"config": true, "raw": true, "encoding": true, "format": true, "color": true}
	convertValueFlags  = map[string]bool{"config": true, "output": true, "encoding": true, "to": true}
	sampleValueFlags   = map[string]bool{"format": true, "output": true}
	diffValueFlags     = map[string]bool{"config": true, "encoding": true, "format": true, "color": true, "filter": true}
	redactValueFlags   = map[string]bool{"config": true, "raw": true, "encoding": true, "format": true, "color": true}
)

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
