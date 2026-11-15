package main

import (
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []Entry
	}{
		{
			name: "empty file",
			text: "",
			want: nil,
		},
		{
			name: "only blanks and comments",
			text: " \n/* nothing */\r\n// here\r",
			want: nil,
		},
		{
			name: "quoted pairs with LF",
			text: "\"a\" = \"1\";\n\"b\" = \"2\";\n",
			want: []Entry{{"a", "1", 1}, {"b", "2", 2}},
		},
		{
			name: "CRLF line endings",
			text: "\"a\" = \"1\";\r\n\"b\" = \"2\";\r\n",
			want: []Entry{{"a", "1", 1}, {"b", "2", 2}},
		},
		{
			name: "lone CR line endings",
			text: "\"a\" = \"1\";\r\"b\" = \"2\";\r\r\"c\" = \"3\";",
			want: []Entry{{"a", "1", 1}, {"b", "2", 2}, {"c", "3", 4}},
		},
		{
			name: "line comment ended by lone CR",
			text: "// first\r\"a\" = \"1\"; // trailing\r\"b\" = \"2\";\r",
			want: []Entry{{"a", "1", 2}, {"b", "2", 3}},
		},
		{
			name: "block comments, also between tokens",
			text: "/* header\r   on two lines */\r\"a\" /* k */ = /* v */ \"1\" /* end */;\r/**/\"b\"=\"2\";",
			want: []Entry{{"a", "1", 3}, {"b", "2", 4}},
		},
		{
			name: "comment markers inside strings are text",
			text: "\"a\" = \"http://example.com/*x*/\";\n\"//b\" = \"2\";",
			want: []Entry{{"a", "http://example.com/*x*/", 1}, {"//b", "2", 2}},
		},
		{
			name: "escaped quotes and backslashes",
			text: `"say \"hi\"" = "a \"quoted\" word and a back\\slash";`,
			want: []Entry{{`say "hi"`, `a "quoted" word and a back\slash`, 1}},
		},
		{
			name: "string ending with an escaped backslash",
			text: `"k" = "C:\\"; "l" = "\\\"";`,
			want: []Entry{{"k", `C:\`, 1}, {"l", `\"`, 1}},
		},
		{
			name: "control escapes",
			text: `"k" = "line1\nline2\tend\r";`,
			want: []Entry{{"k", "line1\nline2\tend\r", 1}},
		},
		{
			name: "unicode escapes",
			text: `"k" = "\U00e9t\u00E9 \U4e16";`,
			want: []Entry{{"k", "\u00e9t\u00e9 \u4e16", 1}},
		},
		{
			name: "unicode escape surrogate pair",
			text: `"k" = "\UD83D\UDE00!";`,
			want: []Entry{{"k", "\U0001F600!", 1}},
		},
		{
			name: "lone surrogate escape becomes U+FFFD",
			text: `"k" = "\UD83D\n";`,
			want: []Entry{{"k", "\uFFFD\n", 1}},
		},
		{
			name: "octal and unknown escapes",
			text: `"k" = "\101\7z\q";`,
			want: []Entry{{"k", "A\az" + "q", 1}},
		},
		{
			name: "unquoted keys",
			text: "CFBundleName = \"App\";\nNS.key-1_x = \"v\";\n",
			want: []Entry{{"CFBundleName", "App", 1}, {"NS.key-1_x", "v", 2}},
		},
		{
			name: "unquoted key and value without spaces",
			text: "key=value;other=42;",
			want: []Entry{{"key", "value", 1}, {"other", "42", 1}},
		},
		{
			name: "key without value",
			text: "\"Cancel\";\rOK;\r",
			want: []Entry{{"Cancel", "Cancel", 1}, {"OK", "OK", 2}},
		},
		{
			name: "multi-line value keeps raw line breaks",
			text: "\"k\" = \"one\rtwo\";\r\"l\" = \"3\";",
			want: []Entry{{"k", "one\rtwo", 1}, {"l", "3", 3}},
		},
		{
			name: "empty key and value",
			text: `"" = "";`,
			want: []Entry{{"", "", 1}},
		},
		{
			name: "non-ASCII text and a stray BOM",
			text: "\uFEFF\"cl\u00e9\" = \"\u4e16\u754c\";",
			want: []Entry{{"cl\u00e9", "\u4e16\u754c", 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(tt.text)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("entries = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantLine int
		wantCol  int
	}{
		{"missing semicolon at end of file", `"a" = "1"`, 1, 10},
		{"missing semicolon between entries", "\"a\" = \"1\"\r\"b\" = \"2\";", 2, 1},
		{"missing value", `"a" = ;`, 1, 7},
		{"missing equals sign", `"a" "1";`, 1, 5},
		{"unterminated string", "\"a\" = \"1\";\r\"b\" = \"2;\r", 2, 7},
		{"unterminated string ending in backslash", `"a\`, 1, 1},
		{"unterminated comment", "\"a\" = \"1\";\r\n/* open", 2, 1},
		{"short unicode escape", `"a" = "\U12";`, 1, 12},
		{"unexpected character", "\"a\" = \"1\";\n{", 2, 1},
		{"lone slash is not a comment", "\n\n  / * x */", 3, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.text)
			var perr *ParseError
			if !errors.As(err, &perr) {
				t.Fatalf("error = %v, want a *ParseError", err)
			}
			if perr.Line != tt.wantLine || perr.Col != tt.wantCol {
				t.Errorf("position = %d:%d, want %d:%d (%s)", perr.Line, perr.Col, tt.wantLine, tt.wantCol, perr.Msg)
			}
		})
	}
}

// TestDecodeAndParse runs the same document through every supported
// encoding and line ending and expects identical entries each time.
func TestDecodeAndParse(t *testing.T) {
	const doc = "/* Greeting */\n" +
		"\"hello\" = \"Bonjour \\\"monde\\\"\";\n" +
		"// plain key\n" +
		"CFBundleName = \"Caf\u00e9\";\n" +
		"\n" +
		"\"bye\" = \"\\U00c0 bient\u00f4t\";\n"
	want := []Entry{
		{"hello", `Bonjour "monde"`, 2},
		{"CFBundleName", "Caf\u00e9", 4},
		{"bye", "\u00c0 bient\u00f4t", 6},
	}

	macRomanBytes := func(s string) []byte {
		var out []byte
		for _, r := range s {
			switch r {
			case '\u00e9':
				out = append(out, 0x8E)
			case '\u00f4':
				out = append(out, 0x99)
			default:
				out = append(out, byte(r))
			}
		}
		return out
	}

	encodings := []struct {
		name    string
		encode  func(string) []byte
		wantEnc string
	}{
		{"utf-16le bom", func(s string) []byte { return utf16Bytes(s, binary.LittleEndian, true) }, encUTF16LEBOM},
		{"utf-16be bom", func(s string) []byte { return utf16Bytes(s, binary.BigEndian, true) }, encUTF16BEBOM},
		{"utf-16le no bom", func(s string) []byte { return utf16Bytes(s, binary.LittleEndian, false) }, encUTF16LE},
		{"utf-16be no bom", func(s string) []byte { return utf16Bytes(s, binary.BigEndian, false) }, encUTF16BE},
		{"utf-8 bom", func(s string) []byte { return append([]byte{0xEF, 0xBB, 0xBF}, s...) }, encUTF8BOM},
		{"utf-8", func(s string) []byte { return []byte(s) }, encUTF8},
		{"macroman", macRomanBytes, encMacRoman},
	}
	lineEndings := []struct {
		name string
		eol  string
	}{
		{"LF", "\n"},
		{"CRLF", "\r\n"},
		{"CR", "\r"},
	}

	for _, enc := range encodings {
		for _, le := range lineEndings {
			t.Run(enc.name+"/"+le.name, func(t *testing.T) {
				var text string
				for _, r := range doc {
					if r == '\n' {
						text += le.eol
					} else {
						text += string(r)
					}
				}
				decoded, gotEnc := decode(enc.encode(text))
				if gotEnc != enc.wantEnc {
					t.Fatalf("encoding = %q, want %q", gotEnc, enc.wantEnc)
				}
				got, err := parse(decoded)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("entries = %+v, want %+v", got, want)
				}
			})
		}
	}
}
