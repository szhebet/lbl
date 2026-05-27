package utils

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

func ExtractPDFText(data []byte, numPages int) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}

	totalPages := reader.NumPage()
	pagesToRead := numPages
	if pagesToRead > totalPages {
		pagesToRead = totalPages
	}

	fonts := make(map[string]*pdf.Font)
	var textParts []string
	for i := 1; i <= pagesToRead; i++ {
		page := reader.Page(i)
		for _, name := range page.Fonts() {
			if _, ok := fonts[name]; !ok {
				f := page.Font(name)
				fonts[name] = &f
			}
		}
		text, err := page.GetPlainText(fonts)
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			textParts = append(textParts, text)
		}
	}

	return strings.Join(textParts, "\n\n"), nil
}
