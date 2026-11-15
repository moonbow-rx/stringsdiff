# stringsdiff

stringsdiff compares the `.strings` files of an Apple application across its
languages. Given a directory that contains `*.lproj` directories, it takes
every `.strings` file of the base language and lists, for each other
language, the keys that are missing and the keys that are extra.

Files are decoded before parsing: UTF-16 with a byte order mark (little or
big endian), UTF-16 without one (detected heuristically), UTF-8 with or
without a byte order mark, and MacRoman as a last resort. The parser does not
work line by line, so LF, CRLF and lone CR line endings are all accepted. It
understands quoted and unquoted keys and values, `/* */` and `//` comments,
entries without a value (`"key";`) and the escapes `\"`, `\\`, `\n`, `\t`,
`\r`, `\Uxxxx` (including surrogate pairs) and octal `\ooo`.

## Install

Go 1.21 or later is required. There are no dependencies outside the standard
library. From a checkout of this repository:

    go build -o stringsdiff .

or, to install the binary into `$GOBIN`:

    go install .

## Usage

    stringsdiff [-base name.lproj] [-json] [directory]

- `directory` is the directory containing the `*.lproj` directories. It
  defaults to the current directory.
- `-base` selects the base language. The `.lproj` suffix may be omitted. By
  default `English.lproj` is used if it exists, then `en.lproj`.
- `-json` prints the full report as JSON instead of text.

Exit status:

- `0`: no key and no file is missing (extra keys alone do not fail).
- `1`: at least one key or one file is missing in some language.
- `2`: usage error, unreadable file or syntax error.

## Example

    $ stringsdiff demo
    base: en.lproj
    de.lproj/InfoPlist.strings: file missing
    fr.lproj/Localizable.strings: 1 missing, 1 extra
      missing: "button.cancel"
      extra:   "old.key"
    missing keys: 1, missing files: 1, extra keys: 1

The text report only lists files that differ from the base. With `-json`,
every compared file is listed:

    $ stringsdiff -json demo
    {
      "base": "en.lproj",
      "results": [
        {
          "language": "de.lproj",
          "file": "InfoPlist.strings",
          "file_missing": true,
          "missing": [],
          "extra": []
        },
        {
          "language": "fr.lproj",
          "file": "Localizable.strings",
          "file_missing": false,
          "missing": [
            "button.cancel"
          ],
          "extra": [
            "old.key"
          ]
        }
      ]
    }

(The JSON above is shortened: entries without differences were removed.)

## Limitations

- Only keys are compared. Values, format specifiers and comments are not.
- Only `.strings` files directly inside each `.lproj` directory are read.
  `.stringsdict`, `.xcstrings`, binary and XML property lists are not
  supported.
- `.strings` files that exist in another language but not in the base
  language are not reported.
- Duplicate keys within a file are not reported; a key counts once.
- Encoding detection is a heuristic. UTF-16 without a byte order mark is
  recognised by the position of its zero bytes, and any file that is neither
  UTF-16 nor valid UTF-8 is read as MacRoman, even if it was written in
  another legacy encoding. This only matters for keys containing non-ASCII
  characters.
- Octal escapes are mapped directly to Unicode code points, not through the
  NeXTSTEP character set.
- Directory names are matched exactly, including case.
