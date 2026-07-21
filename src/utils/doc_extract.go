package utils

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/richardlehane/mscfb"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func ExtractDOCText(data []byte, numPages int) (string, error) {
	reader := bytes.NewReader(data)
	doc, err := mscfb.New(reader)
	if err != nil {
		return "", fmt.Errorf("cannot open OLE2 document: %w", err)
	}

	var wordData []byte
	for _, entry := range doc.File {
		if entry.Name == "WordDocument" && !entry.FileInfo().IsDir() {
			wordData, err = io.ReadAll(entry)
			if err != nil {
				return "", fmt.Errorf("cannot read WordDocument stream: %w", err)
			}
			break
		}
	}
	if wordData == nil {
		return "", fmt.Errorf("WordDocument stream not found")
	}

	// fUnicode flag (bit 0 of the Fib flags word at offset 0x0A) tells us
	// whether the text is stored as UTF-16LE (1) or single-byte ANSI (0).
	isUnicode := false
	if len(wordData) > 0x0B {
		isUnicode = wordData[0x0A]&0x01 == 0x01
	}

	unicodeText := extractDOCWords(decodeDOCRunes(wordData, true), numPages)
	ansiText := extractDOCWords(decodeDOCRunes(wordData, false), numPages)

	// Prefer the encoding that yields the most readable letters. This also
	// covers the (rare) case where the fUnicode flag is misread.
	if !isUnicode || countDOCLetters(unicodeText) < countDOCLetters(ansiText) {
		if countDOCLetters(ansiText) > countDOCLetters(unicodeText) {
			unicodeText = ansiText
		}
	}

	if unicodeText == "" {
		return "", fmt.Errorf("no readable text found in DOC file")
	}
	return unicodeText, nil
}

func decodeDOCRunes(wordData []byte, unicode bool) []rune {
	if unicode {
		n := len(wordData) / 2
		runes := make([]rune, 0, n)
		for i := 0; i+1 < len(wordData); i += 2 {
			u := uint16(wordData[i]) | uint16(wordData[i+1])<<8
			runes = append(runes, rune(u))
		}
		return runes
	}

	dec := charmap.Windows1251.NewDecoder()
	out, _, err := transform.Bytes(dec, wordData)
	if err != nil {
		out = wordData
	}
	runes := make([]rune, len(out))
	for i, b := range out {
		runes[i] = rune(b)
	}
	return runes
}

func isDOCPrintable(u rune) bool {
	if u >= 0x20 && u <= 0x7E {
		return true
	}
	if u >= 0x400 && u <= 0x52F {
		return true
	}
	switch u {
	case 0x2013, 0x2014, 0x2018, 0x2019, 0x201C, 0x201D, 0x201E, 0x2026,
		0x2212, 0x221A, 0x2248, 0x2260, 0x2264, 0x2265, 0xA0, 0xAB, 0xBB:
		return true
	}
	return false
}

func extractDOCWords(runes []rune, numPages int) string {
	wordsPerPage := 250
	maxWords := numPages * wordsPerPage
	var textParts []string
	var cur []rune
	wordCount := 0

	flush := func() {
		if len(cur) == 0 {
			return
		}
		word := strings.TrimSpace(string(cur))
		if word != "" {
			textParts = append(textParts, word)
			wordCount++
		}
		cur = nil
	}

	for _, u := range runes {
		if wordCount >= maxWords && len(cur) == 0 {
			break
		}
		if isDOCPrintable(u) {
			cur = append(cur, u)
		} else if u == 0x0D || u == 0x0A || u == 0x09 || u == 0x20 {
			cur = append(cur, ' ')
		} else if u == 0x0C || u == 0x1E {
			flush()
		} else if len(cur) > 0 {
			flush()
		}
	}
	flush()

	return strings.Join(textParts, " ")
}

func countDOCLetters(text string) int {
	n := 0
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= 0x400 && r <= 0x52F) {
			n++
		}
	}
	return n
}
