package main

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Encoding names reported by decode.
const (
	encUTF8       = "UTF-8"
	encUTF8BOM    = "UTF-8 (BOM)"
	encUTF16LE    = "UTF-16LE"
	encUTF16BE    = "UTF-16BE"
	encUTF16LEBOM = "UTF-16LE (BOM)"
	encUTF16BEBOM = "UTF-16BE (BOM)"
	encMacRoman   = "MacRoman"
)

// decode converts the raw bytes of a .strings file to a Go string and
// reports which encoding was assumed.
//
// Detection order: byte order mark, UTF-16 without BOM (heuristic), valid
// UTF-8, and finally MacRoman, which accepts any byte sequence.
func decode(data []byte) (text, encoding string) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return strings.ToValidUTF8(string(data[3:]), "\uFFFD"), encUTF8BOM
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return decodeUTF16(data[2:], binary.LittleEndian), encUTF16LEBOM
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return decodeUTF16(data[2:], binary.BigEndian), encUTF16BEBOM
	}
	switch order := guessUTF16(data); order {
	case binary.ByteOrder(binary.LittleEndian):
		return decodeUTF16(data, order), encUTF16LE
	case binary.ByteOrder(binary.BigEndian):
		return decodeUTF16(data, order), encUTF16BE
	}
	if utf8.Valid(data) {
		return string(data), encUTF8
	}
	return decodeMacRoman(data), encMacRoman
}

// guessUTF16 returns the likely byte order of BOM-less UTF-16 data, or nil
// if the data does not look like UTF-16.
//
// The syntax of a .strings file (quotes, '=', ';', line breaks) is ASCII, so
// UTF-16 data always contains many code units whose high byte is zero. Those
// zero bytes sit at even offsets in big-endian data and at odd offsets in
// little-endian data. Text in UTF-8 or MacRoman contains no zero bytes.
func guessUTF16(data []byte) binary.ByteOrder {
	units := len(data) / 2
	if units == 0 {
		return nil
	}
	var even, odd int
	for i, b := range data[:units*2] {
		if b != 0 {
			continue
		}
		if i%2 == 0 {
			even++
		} else {
			odd++
		}
	}
	hi, lo := even, odd
	var order binary.ByteOrder = binary.BigEndian
	if odd > even {
		hi, lo = odd, even
		order = binary.LittleEndian
	}
	// Require the zero bytes to be frequent and clearly on one side.
	if hi*10 < units || hi <= 3*lo {
		return nil
	}
	return order
}

// decodeUTF16 decodes UTF-16 data in the given byte order. Unpaired
// surrogates and a trailing odd byte become U+FFFD.
func decodeUTF16(data []byte, order binary.ByteOrder) string {
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		units = append(units, order.Uint16(data[i:]))
	}
	runes := utf16.Decode(units)
	if len(data)%2 == 1 {
		runes = append(runes, utf8.RuneError)
	}
	return string(runes)
}

// decodeMacRoman decodes data in the classic Mac OS Roman encoding.
func decodeMacRoman(data []byte) string {
	var sb strings.Builder
	sb.Grow(len(data))
	for _, b := range data {
		if b < 0x80 {
			sb.WriteByte(b)
		} else {
			sb.WriteRune(macRoman[b-0x80])
		}
	}
	return sb.String()
}

// macRoman maps the bytes 0x80-0xFF of Mac OS Roman to Unicode.
var macRoman = [128]rune{
	// 0x80
	0x00C4, 0x00C5, 0x00C7, 0x00C9, 0x00D1, 0x00D6, 0x00DC, 0x00E1,
	0x00E0, 0x00E2, 0x00E4, 0x00E3, 0x00E5, 0x00E7, 0x00E9, 0x00E8,
	// 0x90
	0x00EA, 0x00EB, 0x00ED, 0x00EC, 0x00EE, 0x00EF, 0x00F1, 0x00F3,
	0x00F2, 0x00F4, 0x00F6, 0x00F5, 0x00FA, 0x00F9, 0x00FB, 0x00FC,
	// 0xA0
	0x2020, 0x00B0, 0x00A2, 0x00A3, 0x00A7, 0x2022, 0x00B6, 0x00DF,
	0x00AE, 0x00A9, 0x2122, 0x00B4, 0x00A8, 0x2260, 0x00C6, 0x00D8,
	// 0xB0
	0x221E, 0x00B1, 0x2264, 0x2265, 0x00A5, 0x00B5, 0x2202, 0x2211,
	0x220F, 0x03C0, 0x222B, 0x00AA, 0x00BA, 0x03A9, 0x00E6, 0x00F8,
	// 0xC0
	0x00BF, 0x00A1, 0x00AC, 0x221A, 0x0192, 0x2248, 0x2206, 0x00AB,
	0x00BB, 0x2026, 0x00A0, 0x00C0, 0x00C3, 0x00D5, 0x0152, 0x0153,
	// 0xD0
	0x2013, 0x2014, 0x201C, 0x201D, 0x2018, 0x2019, 0x00F7, 0x25CA,
	0x00FF, 0x0178, 0x2044, 0x20AC, 0x2039, 0x203A, 0xFB01, 0xFB02,
	// 0xE0
	0x2021, 0x00B7, 0x201A, 0x201E, 0x2030, 0x00C2, 0x00CA, 0x00C1,
	0x00CB, 0x00C8, 0x00CD, 0x00CE, 0x00CF, 0x00CC, 0x00D3, 0x00D4,
	// 0xF0
	0xF8FF, 0x00D2, 0x00DA, 0x00DB, 0x00D9, 0x0131, 0x02C6, 0x02DC,
	0x00AF, 0x02D8, 0x02D9, 0x02DA, 0x00B8, 0x02DD, 0x02DB, 0x02C7,
}
