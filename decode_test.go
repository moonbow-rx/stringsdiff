package main

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

// utf16Bytes encodes s as UTF-16 in the given byte order, optionally
// preceded by a byte order mark.
func utf16Bytes(s string, order binary.ByteOrder, bom bool) []byte {
	units := utf16.Encode([]rune(s))
	if bom {
		units = append([]uint16{0xFEFF}, units...)
	}
	out := make([]byte, 2*len(units))
	for i, u := range units {
		order.PutUint16(out[2*i:], u)
	}
	return out
}

func TestDecode(t *testing.T) {
	const sample = "\"greeting\" = \"h\u00E9llo \u4E16\u754C \U0001F600\";\r"

	tests := []struct {
		name     string
		data     []byte
		wantText string
		wantEnc  string
	}{
		{
			name:     "utf-16 little endian with BOM",
			data:     utf16Bytes(sample, binary.LittleEndian, true),
			wantText: sample,
			wantEnc:  encUTF16LEBOM,
		},
		{
			name:     "utf-16 big endian with BOM",
			data:     utf16Bytes(sample, binary.BigEndian, true),
			wantText: sample,
			wantEnc:  encUTF16BEBOM,
		},
		{
			name:     "utf-16 little endian without BOM",
			data:     utf16Bytes(sample, binary.LittleEndian, false),
			wantText: sample,
			wantEnc:  encUTF16LE,
		},
		{
			name:     "utf-16 big endian without BOM",
			data:     utf16Bytes(sample, binary.BigEndian, false),
			wantText: sample,
			wantEnc:  encUTF16BE,
		},
		{
			name:     "utf-16 without BOM, mostly CJK",
			data:     utf16Bytes("\"\u4E16\u754C\u4F60\u597D\u4E16\u754C\u4F60\u597D\" = \"\u4E16\u754C\u4F60\u597D\u4E16\u754C\u4F60\u597D\u4E16\u754C\";", binary.BigEndian, false),
			wantText: "\"\u4E16\u754C\u4F60\u597D\u4E16\u754C\u4F60\u597D\" = \"\u4E16\u754C\u4F60\u597D\u4E16\u754C\u4F60\u597D\u4E16\u754C\";",
			wantEnc:  encUTF16BE,
		},
		{
			name:     "utf-16 with BOM and odd trailing byte",
			data:     append(utf16Bytes("a;", binary.LittleEndian, true), 0x41),
			wantText: "a;\uFFFD",
			wantEnc:  encUTF16LEBOM,
		},
		{
			name:     "utf-8 with BOM",
			data:     append([]byte{0xEF, 0xBB, 0xBF}, sample...),
			wantText: sample,
			wantEnc:  encUTF8BOM,
		},
		{
			name:     "utf-8 without BOM",
			data:     []byte(sample),
			wantText: sample,
			wantEnc:  encUTF8,
		},
		{
			name:     "plain ASCII is UTF-8",
			data:     []byte("key = value;\n"),
			wantText: "key = value;\n",
			wantEnc:  encUTF8,
		},
		{
			name:     "empty input",
			data:     nil,
			wantText: "",
			wantEnc:  encUTF8,
		},
		{
			name:     "MacRoman fallback",
			data:     []byte{'"', 'k', '"', ' ', '=', ' ', '"', 0x8E, 't', 0x8E, ' ', 0xD2, 'x', 0xD3, ' ', 0xDB, 0xA9, '"', ';', '\r'},
			wantText: "\"k\" = \"\u00E9t\u00E9 \u201Cx\u201D \u20AC\u00A9\";\r",
			wantEnc:  encMacRoman,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotEnc := decode(tt.data)
			if gotEnc != tt.wantEnc {
				t.Errorf("encoding = %q, want %q", gotEnc, tt.wantEnc)
			}
			if gotText != tt.wantText {
				t.Errorf("text = %q, want %q", gotText, tt.wantText)
			}
		})
	}
}
