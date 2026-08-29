package utils

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type ZipContentType int

const (
	ZipContentFB2 ZipContentType = iota
	ZipContentPDF
	ZipContentDOC
	ZipContentDOCX
	ZipContentEPUB
	ZipContentUnknown
)

type ZipExtractResult struct {
	Content     []byte
	ContentType ZipContentType
}

func DetectZipContent(data []byte) (*ZipExtractResult, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}

	for _, f := range reader.File {
		ext := strings.ToLower(filepath.Ext(f.Name))

		if ext == ".zip" || ext == ".rar" || ext == ".7z" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			innerData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			result, err := DetectZipContent(innerData)
			if err == nil {
				return result, nil
			}
		}
	}

	for _, f := range reader.File {
		ext := strings.ToLower(filepath.Ext(f.Name))
		switch ext {
		case ".fb2", ".xml":
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
			if looksLikeFB2(content) {
				return &ZipExtractResult{Content: content, ContentType: ZipContentFB2}, nil
			}
			// Entry has .fb2 extension but is not FB2 — try as nested archive (FB2.ZIP disguised as .fb2)
			if nested, nestErr := DetectZipContent(content); nestErr == nil {
				return nested, nil
			}
		}
	}

	for _, f := range reader.File {
		ext := strings.ToLower(filepath.Ext(f.Name))
		switch ext {
		case ".pdf":
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			return &ZipExtractResult{Content: content, ContentType: ZipContentPDF}, nil
		case ".docx":
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			return &ZipExtractResult{Content: content, ContentType: ZipContentDOCX}, nil
		case ".doc":
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			return &ZipExtractResult{Content: content, ContentType: ZipContentDOC}, nil
		case ".epub":
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			return &ZipExtractResult{Content: content, ContentType: ZipContentEPUB}, nil
		}
	}

	return nil, fmt.Errorf("no supported book file found in archive")
}

func looksLikeFB2(data []byte) bool {
	checkLen := len(data)
	if checkLen > 1024 {
		checkLen = 1024
	}
	sample := string(data[:checkLen])
	return strings.Contains(sample, "<FictionBook") || strings.Contains(sample, "<description")
}

func ZipContentTypeToFormatName(ct ZipContentType) string {
	switch ct {
	case ZipContentFB2:
		return "FB2"
	case ZipContentPDF:
		return "PDF"
	case ZipContentDOC:
		return "DOC"
	case ZipContentDOCX:
		return "DOCX"
	case ZipContentEPUB:
		return "EPUB"
	default:
		return "FB2.ZIP"
	}
}

func InnerFileExtFromZipContent(ct ZipContentType) string {
	switch ct {
	case ZipContentFB2:
		return ".fb2"
	case ZipContentPDF:
		return ".pdf"
	case ZipContentDOC:
		return ".doc"
	case ZipContentDOCX:
		return ".docx"
	case ZipContentEPUB:
		return ".epub"
	default:
		return ".fb2"
	}
}

func GetFormatNameFromZip(filename string) string {
	base := strings.TrimSuffix(filename, ".zip")
	base = strings.TrimSuffix(base, ".fb2")
	ext := strings.ToLower(filepath.Ext(base))
	switch ext {
	case ".pdf":
		return "PDF"
	case ".doc":
		return "DOC"
	case ".docx":
		return "DOCX"
	case ".epub":
		return "EPUB"
	default:
		return "FB2.ZIP"
	}
}

func GetFormatNameByExt(ext string) string {
	switch ext {
	case ".pdf":
		return "PDF"
	case ".doc":
		return "DOC"
	case ".docx":
		return "DOCX"
	case ".epub":
		return "EPUB"
	case ".fb2":
		return "FB2"
	default:
		return strings.ToUpper(strings.TrimPrefix(ext, "."))
	}
}
