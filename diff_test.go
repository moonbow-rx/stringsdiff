package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTree creates the given files (slash-separated relative path to
// content) in a new temporary directory and returns that directory.
func writeTree(t *testing.T, files map[string][]byte) string {
	t.Helper()
	root := t.TempDir()
	for name, data := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCompare(t *testing.T) {
	none := []string{}

	tests := []struct {
		name    string
		files   map[string][]byte
		base    string
		want    Report
		wantErr string
	}{
		{
			name: "missing and extra keys across encodings and line endings",
			files: map[string][]byte{
				"en.lproj/Localizable.strings": []byte("\"a\" = \"A\";\n\"b\" = \"B\";\n\"c\" = \"C\";\n"),
				"fr.lproj/Localizable.strings": utf16Bytes("/* French */\r\"c\" = \"C\";\r\"a\" = \"A\";\r\"z\" = \"Z\";\r", binary.BigEndian, true),
				"de.lproj/Localizable.strings": utf16Bytes("\"a\" = \"A\";\r\n\"b\" = \"B\";\r\n\"c\" = \"C\";\r\n", binary.LittleEndian, false),
			},
			want: Report{Base: "en.lproj", Results: []Result{
				{Language: "de.lproj", File: "Localizable.strings", Missing: none, Extra: none},
				{Language: "fr.lproj", File: "Localizable.strings", Missing: []string{"b"}, Extra: []string{"z"}},
			}},
		},
		{
			name: "English.lproj is preferred over en.lproj",
			files: map[string][]byte{
				"English.lproj/Main.strings": []byte("a = 1;"),
				"en.lproj/Main.strings":      []byte("a = 1; b = 2;"),
			},
			want: Report{Base: "English.lproj", Results: []Result{
				{Language: "en.lproj", File: "Main.strings", Missing: none, Extra: []string{"b"}},
			}},
		},
		{
			name: "explicit base without the .lproj suffix",
			files: map[string][]byte{
				"en.lproj/Main.strings": []byte("a = 1;"),
				"fr.lproj/Main.strings": []byte("a = 1; b = 2;"),
			},
			base: "fr",
			want: Report{Base: "fr.lproj", Results: []Result{
				{Language: "en.lproj", File: "Main.strings", Missing: []string{"b"}, Extra: none},
			}},
		},
		{
			name: "missing file and several files",
			files: map[string][]byte{
				"en.lproj/A.strings":   []byte("a = 1;"),
				"en.lproj/B.strings":   []byte("b = 1;"),
				"en.lproj/notes.txt":   []byte("not a strings file"),
				"fr.lproj/A.strings":   []byte("a = 1;"),
				"fr.lproj/Other.plist": []byte("ignored"),
				"README.md":            []byte("ignored"),
			},
			want: Report{Base: "en.lproj", Results: []Result{
				{Language: "fr.lproj", File: "A.strings", Missing: none, Extra: none},
				{Language: "fr.lproj", File: "B.strings", FileMissing: true, Missing: none, Extra: none},
			}},
		},
		{
			name: "duplicate keys are reported once",
			files: map[string][]byte{
				"en.lproj/A.strings": []byte("a = 1; a = 2; b = 3;"),
				"fr.lproj/A.strings": []byte("x = 1; x = 2;"),
			},
			want: Report{Base: "en.lproj", Results: []Result{
				{Language: "fr.lproj", File: "A.strings", Missing: []string{"a", "b"}, Extra: []string{"x"}},
			}},
		},
		{
			name: "no other language",
			files: map[string][]byte{
				"en.lproj/A.strings": []byte("a = 1;"),
			},
			want: Report{Base: "en.lproj", Results: []Result{}},
		},
		{
			name: "base language not found",
			files: map[string][]byte{
				"fr.lproj/A.strings": []byte("a = 1;"),
			},
			wantErr: "base language not found",
		},
		{
			name: "base language without .strings files",
			files: map[string][]byte{
				"en.lproj/notes.txt": []byte("x"),
			},
			wantErr: "no .strings files",
		},
		{
			name: "syntax error names the file and position",
			files: map[string][]byte{
				"en.lproj/A.strings": []byte("a = 1;"),
				"fr.lproj/A.strings": []byte("a = 1;\rb = ;"),
			},
			wantErr: filepath.Join("fr.lproj", "A.strings") + ":2:5: expected value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeTree(t, tt.files)
			got, err := compare(root, tt.base)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("report = %+v, want %+v", got, tt.want)
			}
		})
	}
}
