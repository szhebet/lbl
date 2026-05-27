package utils

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/richardlehane/mscfb"
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
			wordData = make([]byte, entry.Size)
			_, err := entry.Read(wordData)
			if err != nil {
				return "", fmt.Errorf("cannot read WordDocument stream: %w", err)
			}
			break
		}
	}
	if wordData == nil {
		return "", fmt.Errorf("WordDocument stream not found")
	}

	var textParts []string
	var cur []rune
	wordsPerPage := 250
	maxWords := numPages * wordsPerPage
	wordCount := 0

	isPrintable := func(u uint16) bool {
		if u >= 0x20 && u <= 0x7E {
			return true
		}
		if u >= 0x400 && u <= 0x52F {
			return true
		}
		switch u {
		case 0x2013, 0x2014, 0x2018, 0x2019, 0x201C, 0x201D, 0x201E, 0x2026:
			return true
		case 0x2212, 0x221A, 0x2248, 0x2260, 0x2264, 0x2265:
			return true
		case 0xA0, 0xAB, 0xBB:
			return true
		}
		return false
	}

	toRune := func(u uint16) rune {
		switch u {
		case 0x2013, 0x2014:
			return '—'
		case 0x2018, 0x2019, 0x201A, 0x201B:
			return '\''
		case 0x201C, 0x201D, 0x201E, 0x201F, 0xAB, 0xBB:
			return '"'
		case 0x2026:
			return '…'
		case 0xA0:
			return ' '
		}
		return rune(u)
	}

	for i := 0; i+1 < len(wordData); i += 2 {
		u := uint16(wordData[i]) | uint16(wordData[i+1])<<8

		if wordCount >= maxWords && len(cur) == 0 {
			break
		}

		if isPrintable(u) {
			cur = append(cur, toRune(u))
		} else if u == 0x0D || u == 0x0A || u == 0x09 || u == 0x20 {
			cur = append(cur, ' ')
		} else if u == 0x0C || u == 0x1E {
			// Page break / paragraph separator -> word boundary
			if len(cur) > 0 {
				word := strings.TrimSpace(string(cur))
				if word != "" {
					textParts = append(textParts, word)
					wordCount++
				}
				cur = nil
			}
		} else if len(cur) > 0 {
			word := strings.TrimSpace(string(cur))
			if word != "" {
				textParts = append(textParts, word)
				wordCount++
			}
			cur = nil
		}
	}
	if len(cur) > 0 {
		word := strings.TrimSpace(string(cur))
		if word != "" {
			textParts = append(textParts, word)
		}
	}

	if len(textParts) == 0 {
		return "", fmt.Errorf("no readable text found in DOC file")
	}

	return strings.Join(textParts, " "), nil
}
