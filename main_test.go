package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	complete := map[string][]byte{
		"en.lproj/Localizable.strings": []byte("\"a\" = \"A\";\n\"b\" = \"B\";\n"),
		"fr.lproj/Localizable.strings": utf16Bytes("\"a\" = \"A\";\r\"b\" = \"B\";\r", binary.LittleEndian, true),
	}
	extraOnly := map[string][]byte{
		"en.lproj/Localizable.strings": []byte("\"a\" = \"A\";\n"),
		"fr.lproj/Localizable.strings": []byte("\"a\" = \"A\";\n\"z\" = \"Z\";\n"),
	}
	incomplete := map[string][]byte{
		"en.lproj/Localizable.strings": []byte("\"a\" = \"A\";\n\"b\" = \"B\";\n"),
		"en.lproj/InfoPlist.strings":   []byte("CFBundleName = \"App\";\n"),
		"fr.lproj/Localizable.strings": utf16Bytes("\"a\" = \"A\";\r\"z\" = \"Z\";\r", binary.BigEndian, false),
	}

	tests := []struct {
		name       string
		files      map[string][]byte
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string // substring
	}{
		{
			name:     "identical languages",
			files:    complete,
			wantCode: exitOK,
			wantStdout: "base: en.lproj\n" +
				"missing keys: 0, missing files: 0, extra keys: 0\n",
		},
		{
			name:     "extra keys alone do not fail",
			files:    extraOnly,
			wantCode: exitOK,
			wantStdout: "base: en.lproj\n" +
				"fr.lproj/Localizable.strings: 0 missing, 1 extra\n" +
				"  extra:   \"z\"\n" +
				"missing keys: 0, missing files: 0, extra keys: 1\n",
		},
		{
			name:     "missing key and missing file",
			files:    incomplete,
			wantCode: exitMissing,
			wantStdout: "base: en.lproj\n" +
				"fr.lproj/InfoPlist.strings: file missing\n" +
				"fr.lproj/Localizable.strings: 1 missing, 1 extra\n" +
				"  missing: \"b\"\n" +
				"  extra:   \"z\"\n" +
				"missing keys: 1, missing files: 1, extra keys: 1\n",
		},
		{
			name:     "explicit base",
			files:    extraOnly,
			args:     []string{"-base", "fr.lproj"},
			wantCode: exitMissing,
			wantStdout: "base: fr.lproj\n" +
				"en.lproj/Localizable.strings: 1 missing, 0 extra\n" +
				"  missing: \"z\"\n" +
				"missing keys: 1, missing files: 0, extra keys: 0\n",
		},
		{
			name:       "unknown base",
			files:      complete,
			args:       []string{"-base", "ja"},
			wantCode:   exitError,
			wantStderr: "base language not found",
		},
		{
			name:       "unknown flag",
			files:      complete,
			args:       []string{"-nope"},
			wantCode:   exitError,
			wantStderr: "usage: stringsdiff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeTree(t, tt.files)
			var stdout, stderr bytes.Buffer
			code := run(append(tt.args, root), &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}
			if stdout.String() != tt.wantStdout {
				t.Errorf("stdout =\n%s\nwant\n%s", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunJSON(t *testing.T) {
	root := writeTree(t, map[string][]byte{
		"en.lproj/Localizable.strings": []byte("\"a\" = \"A\";\n\"b <1>\" = \"B\";\n"),
		"en.lproj/InfoPlist.strings":   []byte("CFBundleName = \"App\";\n"),
		"fr.lproj/Localizable.strings": []byte("\"a\" = \"A\";\r"),
	})
	var stdout, stderr bytes.Buffer
	code := run([]string{"-json", root}, &stdout, &stderr)
	if code != exitMissing {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitMissing, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"extra": []`) {
		t.Errorf("empty lists must be encoded as [], got:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"b <1>"`) {
		t.Errorf("keys must not be HTML-escaped, got:\n%s", stdout.String())
	}

	var got Report
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	want := Report{Base: "en.lproj", Results: []Result{
		{Language: "fr.lproj", File: "InfoPlist.strings", FileMissing: true, Missing: []string{}, Extra: []string{}},
		{Language: "fr.lproj", File: "Localizable.strings", Missing: []string{"b <1>"}, Extra: []string{}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("report = %+v, want %+v", got, want)
	}
}
