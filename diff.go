package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	lprojExt   = ".lproj"
	stringsExt = ".strings"
)

// defaultBases are the base languages tried, in order, when none is given.
var defaultBases = []string{"English.lproj", "en.lproj"}

// Result is the comparison of one .strings file of one language against the
// same file of the base language.
type Result struct {
	Language    string   `json:"language"`
	File        string   `json:"file"`
	FileMissing bool     `json:"file_missing"`
	Missing     []string `json:"missing"`
	Extra       []string `json:"extra"`
}

// Report is the comparison of every language against the base language.
type Report struct {
	Base    string   `json:"base"`
	Results []Result `json:"results"`
}

// Counts returns the total number of missing keys, missing files and extra
// keys in the report.
func (r Report) Counts() (missingKeys, missingFiles, extraKeys int) {
	for _, res := range r.Results {
		missingKeys += len(res.Missing)
		extraKeys += len(res.Extra)
		if res.FileMissing {
			missingFiles++
		}
	}
	return
}

// compare checks every .strings file of the base language found in root
// against all other *.lproj directories of root. An empty base selects the
// first existing directory of defaultBases.
func compare(root, base string) (Report, error) {
	base, err := findBase(root, base)
	if err != nil {
		return Report{}, err
	}
	files, err := listNames(filepath.Join(root, base), stringsExt, false)
	if err != nil {
		return Report{}, err
	}
	if len(files) == 0 {
		return Report{}, fmt.Errorf("no %s files in %s", stringsExt, filepath.Join(root, base))
	}
	languages, err := listNames(root, lprojExt, true)
	if err != nil {
		return Report{}, err
	}

	report := Report{Base: base, Results: []Result{}}
	for _, file := range files {
		baseKeys, err := loadKeys(filepath.Join(root, base, file))
		if err != nil {
			return Report{}, err
		}
		for _, lang := range languages {
			if lang == base {
				continue
			}
			res := Result{Language: lang, File: file, Missing: []string{}, Extra: []string{}}
			keys, err := loadKeys(filepath.Join(root, lang, file))
			switch {
			case errors.Is(err, fs.ErrNotExist):
				res.FileMissing = true
			case err != nil:
				return Report{}, err
			default:
				res.Missing = difference(baseKeys, keys)
				res.Extra = difference(keys, baseKeys)
			}
			report.Results = append(report.Results, res)
		}
	}
	return report, nil
}

// findBase resolves the base language directory name inside root.
func findBase(root, base string) (string, error) {
	candidates := defaultBases
	if base != "" {
		if !strings.HasSuffix(base, lprojExt) {
			base += lprojExt
		}
		candidates = []string{base}
	}
	for _, name := range candidates {
		info, err := os.Stat(filepath.Join(root, name))
		if err == nil && info.IsDir() {
			return name, nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("base language not found in %s (tried %s)", root, strings.Join(candidates, ", "))
}

// listNames returns the sorted names of the entries of dir that have the
// given extension and are directories (dirs) or not directories (!dirs).
func listNames(dir, ext string, dirs bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		isDir := e.IsDir()
		if e.Type()&fs.ModeSymlink != 0 {
			if info, err := os.Stat(filepath.Join(dir, e.Name())); err == nil {
				isDir = info.IsDir()
			}
		}
		if isDir == dirs && strings.HasSuffix(e.Name(), ext) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// loadKeys reads, decodes and parses a .strings file and returns its keys
// in file order, without duplicates.
func loadKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text, _ := decode(data)
	entries, err := parse(text)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", path, err)
	}
	seen := make(map[string]bool, len(entries))
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		if !seen[e.Key] {
			seen[e.Key] = true
			keys = append(keys, e.Key)
		}
	}
	return keys, nil
}

// difference returns the elements of a that are not in b, in the order of a.
// The result is never nil.
func difference(a, b []string) []string {
	in := make(map[string]bool, len(b))
	for _, k := range b {
		in[k] = true
	}
	out := []string{}
	for _, k := range a {
		if !in[k] {
			out = append(out, k)
		}
	}
	return out
}
