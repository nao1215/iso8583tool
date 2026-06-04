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
	"github.com/nao1215/iso8583tool/internal/project"
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
	workDir string
}

type resolvedContext struct {
	root      string
	specLabel string
	spec      *messagespec.Spec
	catalog   basei.ExtensionCatalog
}

func NewApp(stdout, stderr io.Writer, workDir string) *App {
	return &App{
		stdout:  stdout,
		stderr:  stderr,
		workDir: workDir,
	}
}

func (a *App) Run(args []string) int {
	if len(args) == 0 {
		a.printRootHelp()
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		if args[0] == "help" && len(args) > 1 {
			return a.runHelp(args[1:])
		}
		a.printRootHelp()
		return 0
	case "init":
		return a.runInit(args[1:])
	case "view":
		return a.runView(args[1:])
	case "write":
		return a.runWrite(args[1:])
	case "validate":
		return a.runValidate(args[1:])
	case "sample":
		return a.runSample(args[1:])
	case "version", "-v", "--version":
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
	case "init", "view", "write", "validate", "sample":
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

func (a *App) runInit(args []string) int {
	flagSet := newFlagSet("init", a.stderr)
	name := flagSet.String("name", "", "project name (defaults to the directory name)")
	dir := flagSet.String("dir", "", "directory to initialize (defaults to the current directory)")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Create an iso8583tool workspace with a starter BASE I overlay.")
		writeLine(a.stderr, "Usage: iso8583tool init [--name NAME] [--dir PATH]")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, args); !ok {
		return code
	}
	if flagSet.NArg() != 0 {
		flagSet.Usage()
		return 1
	}

	target := a.workDir
	if strings.TrimSpace(*dir) != "" {
		if filepath.IsAbs(*dir) {
			target = *dir
		} else {
			target = filepath.Join(a.workDir, *dir)
		}
	}

	result, err := project.Init(target, *name)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	writeLine(a.stdout, "Initialized iso8583tool project.")
	writef(a.stdout, "Config: %s\n", result.ConfigPath)
	writef(a.stdout, "Specs: %s\n", result.SpecsDir)
	writef(a.stdout, "Examples: %s\n", result.ExamplesDir)
	writef(a.stdout, "Messages: %s\n", result.MessagesDir)
	return 0
}

func (a *App) runView(args []string) int {
	flagSet := newFlagSet("view", a.stderr)
	specPath := flagSet.String("spec", "", "path to a moov-io/iso8583 JSON spec file")
	filePath := flagSet.String("file", "", "path to an input message file")
	raw := flagSet.String("raw", "", "inline input message")
	encoding := flagSet.String("encoding", "hex", "input encoding: hex or raw")
	format := flagSet.String("format", "describe", "output format: describe or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Inspect an ISO8583 message with the configured spec.")
		writeLine(a.stderr, "Usage: iso8583tool view (--file PATH | --raw DATA) [--spec PATH] [--encoding hex|raw] [--format describe|json] [--color auto|always|never]")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, args); !ok {
		return code
	}
	if flagSet.NArg() != 0 {
		flagSet.Usage()
		return 1
	}

	ctx, err := a.loadContext(*specPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, err := messageio.ReadRawInput(*filePath, *raw, *encoding)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	pal := a.palette(*color, *format)
	result, err := service.ViewMessage(input, ctx.spec.MessageSpec, ctx.catalog, *format, pal)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	if *format == "describe" {
		writef(a.stdout, "%s %s\n\n", pal.Dim("Spec:"), pal.Bold(ctx.specLabel))
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

// palette resolves the --color flag against stdout. JSON output is never
// colorized so it stays machine-parseable.
func (a *App) palette(mode, format string) render.Palette {
	if format == "json" {
		return render.NewPalette(false)
	}
	out, _ := a.stdout.(*os.File)
	return render.NewPalette(render.ResolveColor(mode, out))
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

func (a *App) runWrite(args []string) int {
	flagSet := newFlagSet("write", a.stderr)
	specPath := flagSet.String("spec", "", "path to a moov-io/iso8583 JSON spec file")
	inputPath := flagSet.String("input", "", "path to a message document JSON file")
	outputPath := flagSet.String("output", "", "path to write the packed message")
	encoding := flagSet.String("encoding", "hex", "output encoding: hex or raw")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Build an ISO8583 message from a JSON document.")
		writeLine(a.stderr, "Usage: iso8583tool write --input PATH [--output PATH] [--spec PATH] [--encoding hex|raw]")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, args); !ok {
		return code
	}
	if flagSet.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		flagSet.Usage()
		return 1
	}

	ctx, err := a.loadContext(*specPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	doc, err := messageio.LoadDocument(*inputPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	result, err := service.WriteMessage(doc, ctx.spec.MessageSpec)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	out, err := messageio.EncodeOutput(result.Raw, *encoding)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	if strings.TrimSpace(*outputPath) != "" {
		if err := os.WriteFile(filepath.Clean(*outputPath), out, 0o600); err != nil {
			writeLine(a.stderr, err)
			return 1
		}
		writef(a.stdout, "Wrote %d fields with %s using %s.\n", result.FieldCount, ctx.specLabel, *encoding)
		writef(a.stdout, "Output: %s\n", *outputPath)
		return 0
	}

	_, _ = a.stdout.Write(out)
	if *encoding == "hex" {
		_, _ = io.WriteString(a.stdout, "\n")
	}
	return 0
}

func (a *App) runValidate(args []string) int {
	flagSet := newFlagSet("validate", a.stderr)
	specPath := flagSet.String("spec", "", "path to a moov-io/iso8583 JSON spec file")
	filePath := flagSet.String("file", "", "path to an input message file")
	raw := flagSet.String("raw", "", "inline input message")
	encoding := flagSet.String("encoding", "hex", "input encoding: hex or raw")
	format := flagSet.String("format", "text", "output format: text or json")
	color := flagSet.String("color", "auto", "colorize output: auto, always, or never")
	flagSet.Usage = func() {
		writeLine(a.stderr, "Validate that a message can be unpacked and highlight extension-field strategy.")
		writeLine(a.stderr, "Usage: iso8583tool validate (--file PATH | --raw DATA) [--spec PATH] [--encoding hex|raw] [--format text|json] [--color auto|always|never]")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, args); !ok {
		return code
	}
	if flagSet.NArg() != 0 {
		flagSet.Usage()
		return 1
	}

	ctx, err := a.loadContext(*specPath)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	input, err := messageio.ReadRawInput(*filePath, *raw, *encoding)
	if err != nil {
		writeLine(a.stderr, err)
		return 1
	}

	report := service.ValidateMessage(input, ctx.spec.MessageSpec, ctx.specLabel, ctx.catalog)
	switch *format {
	case "text":
		a.printValidationReport(report, a.palette(*color, *format))
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
	name := flagSet.String("name", "", "sample name to export")
	format := flagSet.String("format", "json", "sample format: json or hex")
	outputPath := flagSet.String("output", "", "path to write the exported sample")
	flagSet.Usage = func() {
		writeLine(a.stderr, "List or export built-in BASE I starter samples.")
		writeLine(a.stderr, "Usage: iso8583tool sample [--name SAMPLE] [--format json|hex] [--output PATH]")
		printFlagDefaults(a.stderr, flagSet)
	}
	if code, ok := parseArgs(flagSet, args); !ok {
		return code
	}
	if flagSet.NArg() != 0 {
		flagSet.Usage()
		return 1
	}

	if strings.TrimSpace(*name) == "" {
		writeLine(a.stdout, "Available samples:")
		for _, sample := range basei.StarterSamples() {
			writef(a.stdout, "- %s: %s\n", sample.Name, sample.Summary)
		}
		return 0
	}

	sample, ok := basei.LookupSample(strings.TrimSpace(*name))
	if !ok {
		writef(a.stderr, "unknown sample %q\n", *name)
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

func (a *App) loadContext(overrideSpec string) (resolvedContext, error) {
	root, cfg, found, err := project.LoadOptional(a.workDir)
	if err != nil {
		return resolvedContext{}, err
	}
	if !found {
		root = a.workDir
		cfg = config.Default(filepath.Base(a.workDir))
	}
	if strings.TrimSpace(overrideSpec) != "" {
		cfg.Spec.MessageSpec = overrideSpec
	}

	specResult, err := messagespec.Load(root, cfg)
	if err != nil {
		return resolvedContext{}, err
	}

	catalog := basei.DefaultExtensionCatalog()
	if path := strings.TrimSpace(cfg.Spec.ExtensionCatalog); path != "" {
		resolved := path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(root, path)
		}
		loaded, loadErr := basei.LoadCatalog(resolved)
		if loadErr == nil {
			catalog = loaded
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			return resolvedContext{}, loadErr
		}
	}

	return resolvedContext{
		root:      root,
		specLabel: specResult.Label,
		spec:      specResult,
		catalog:   catalog,
	}, nil
}

func (a *App) printRootHelp() {
	writeLine(a.stderr, "BASE I oriented ISO8583 viewer, writer, and validator.")
	writeLine(a.stderr, "")
	writeLine(a.stderr, "Usage:")
	writeLine(a.stderr, "  iso8583tool <command> [flags]")
	writeLine(a.stderr, "")
	writeLine(a.stderr, "Commands:")
	writeLine(a.stderr, "  init       Create a workspace with starter config, examples, and extension catalog")
	writeLine(a.stderr, "  view       Unpack and inspect a message")
	writeLine(a.stderr, "  write      Build a message from JSON input")
	writeLine(a.stderr, "  validate   Check a message against the configured spec")
	writeLine(a.stderr, "  sample     List or export built-in BASE I starter samples")
	writeLine(a.stderr, "  version    Print the version")
	writeLine(a.stderr, "  help       Show command help")
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
