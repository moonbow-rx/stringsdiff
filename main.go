// Command stringsdiff compares the .strings files of an Apple application
// across its languages.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// Exit codes.
const (
	exitOK      = 0
	exitMissing = 1
	exitError   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stringsdiff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "", "base language directory (default: English.lproj, then en.lproj)")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: stringsdiff [-base name.lproj] [-json] [directory]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Compares the .strings files of the base language with those of every")
		fmt.Fprintln(stderr, "other *.lproj directory found in directory (default: current directory).")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return exitError
	}
	root := "."
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	report, err := compare(root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "stringsdiff: %v\n", err)
		return exitError
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		err = enc.Encode(report)
	} else {
		err = printText(stdout, report)
	}
	if err != nil {
		fmt.Fprintf(stderr, "stringsdiff: %v\n", err)
		return exitError
	}

	if missingKeys, missingFiles, _ := report.Counts(); missingKeys+missingFiles > 0 {
		return exitMissing
	}
	return exitOK
}

// printText writes the human-readable report. Files identical to the base
// are not listed.
func printText(w io.Writer, report Report) error {
	var b []byte
	b = fmt.Appendf(b, "base: %s\n", report.Base)
	for _, res := range report.Results {
		switch {
		case res.FileMissing:
			b = fmt.Appendf(b, "%s/%s: file missing\n", res.Language, res.File)
		case len(res.Missing)+len(res.Extra) > 0:
			b = fmt.Appendf(b, "%s/%s: %d missing, %d extra\n", res.Language, res.File, len(res.Missing), len(res.Extra))
			for _, k := range res.Missing {
				b = fmt.Appendf(b, "  missing: %q\n", k)
			}
			for _, k := range res.Extra {
				b = fmt.Appendf(b, "  extra:   %q\n", k)
			}
		}
	}
	missingKeys, missingFiles, extraKeys := report.Counts()
	b = fmt.Appendf(b, "missing keys: %d, missing files: %d, extra keys: %d\n", missingKeys, missingFiles, extraKeys)
	_, err := w.Write(b)
	return err
}
