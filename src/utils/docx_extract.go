package utils

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type docxDocument struct {
	Body docxBody `xml:"body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text string `xml:"t"`
}

func ExtractDOCXText(data []byte, numPages int) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to open DOCX as zip: %w", err)
	}

	var docData []byte
	for _, f := range reader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to read word/document.xml: %w", err)
			}
			docData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", fmt.Errorf("failed to read word/document.xml: %w", err)
			}
			break
		}
	}

	if docData == nil {
		return "", fmt.Errorf("word/document.xml not found in DOCX")
	}

	var doc docxDocument
	if err := xml.Unmarshal(docData, &doc); err != nil {
		return "", fmt.Errorf("failed to parse word/document.xml: %w", err)
	}

	var paragraphs []string
	for _, p := range doc.Body.Paragraphs {
		var textParts []string
		for _, r := range p.Runs {
			textParts = append(textParts, strings.TrimSpace(r.Text))
		}
		line := strings.Join(textParts, "")
		line = strings.TrimSpace(line)
		if line != "" {
			paragraphs = append(paragraphs, line)
		}
	}

	charsPerPage := 3000
	charsNeeded := numPages * charsPerPage
	var result []string
	charCount := 0

	for _, p := range paragraphs {
		if charCount >= charsNeeded {
			break
		}
		result = append(result, p)
		charCount += len(p)
	}

	return strings.Join(result, "\n"), nil
}
