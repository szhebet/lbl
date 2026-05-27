package utils

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func createTestEPUB(t *testing.T, containerXML, opfXML []byte) string {
	t.Helper()

	tmpDir := t.TempDir()
	epubPath := filepath.Join(tmpDir, "test.epub")

	zipFile, err := os.Create(epubPath)
	if err != nil {
		t.Fatalf("failed to create test epub: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)

	containerWriter, _ := zipWriter.Create("META-INF/container.xml")
	containerWriter.Write(containerXML)

	opfWriter, _ := zipWriter.Create("OEBPS/content.opf")
	opfWriter.Write(opfXML)

	zipWriter.Close()

	return epubPath
}

func TestParseEPUB(t *testing.T) {
	containerXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)

	opfXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf" unique-identifier="BookID">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book Title</dc:title>
    <dc:creator id="creator1">John Doe</dc:creator>
    <dc:creator id="creator2">Jane Smith</dc:creator>
    <dc:description>This is a test description</dc:description>
    <dc:publisher>Test Publisher</dc:publisher>
    <dc:date>2023-05-15</dc:date>
    <dc:language>en</dc:language>
    <dc:identifier id="BookID" scheme="ISBN">978-3-16-148410-0</dc:identifier>
    <dc:subject>Fiction</dc:subject>
    <dc:subject>Science Fiction</dc:subject>
  </metadata>
  <manifest>
    <item id="content" href="content.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="content"/>
  </spine>
</package>`)

	epubPath := createTestEPUB(t, containerXML, opfXML)

	book, err := ParseEPUB(epubPath)
	if err != nil {
		t.Fatalf("failed to parse EPUB: %v", err)
	}

	if book.Title != "Test Book Title" {
		t.Errorf("expected title 'Test Book Title', got '%s'", book.Title)
	}

	if len(book.Authors) != 2 {
		t.Errorf("expected 2 authors, got %d", len(book.Authors))
	}

	expectedAuthors := []string{"John Doe", "Jane Smith"}
	for i, author := range book.Authors {
		if author != expectedAuthors[i] {
			t.Errorf("expected author '%s', got '%s'", expectedAuthors[i], author)
		}
	}

	if book.Lang != "en" {
		t.Errorf("expected language 'en', got '%s'", book.Lang)
	}

	if book.Year != "2023" {
		t.Errorf("expected year '2023', got '%s'", book.Year)
	}

	if book.ISBN != "978-3-16-148410-0" {
		t.Errorf("expected ISBN '978-3-16-148410-0', got '%s'", book.ISBN)
	}

	if book.Publisher != "Test Publisher" {
		t.Errorf("expected publisher 'Test Publisher', got '%s'", book.Publisher)
	}

	if book.Annotation != "This is a test description" {
		t.Errorf("expected annotation 'This is a test description', got '%s'", book.Annotation)
	}

	if len(book.Genres) != 2 {
		t.Errorf("expected 2 genres, got %d", len(book.Genres))
	}
}

func TestParseEPUBFromBytes(t *testing.T) {
	containerXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)

	opfXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf" unique-identifier="BookID">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Byte Test Book</dc:title>
    <dc:creator>Author Name</dc:creator>
    <dc:language>ru</dc:language>
  </metadata>
  <manifest>
    <item id="content" href="content.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="content"/>
  </spine>
</package>`)

	tmpDir := t.TempDir()
	epubPath := filepath.Join(tmpDir, "test.epub")

	zipFile, _ := os.Create(epubPath)
	zipWriter := zip.NewWriter(zipFile)
	containerWriter, _ := zipWriter.Create("META-INF/container.xml")
	containerWriter.Write(containerXML)
	opfWriter, _ := zipWriter.Create("OEBPS/content.opf")
	opfWriter.Write(opfXML)
	zipWriter.Close()
	zipFile.Close()

	data, _ := os.ReadFile(epubPath)

	book, err := ParseEPUBFromBytes(data)
	if err != nil {
		t.Fatalf("failed to parse EPUB from bytes: %v", err)
	}

	if book.Title != "Byte Test Book" {
		t.Errorf("expected title 'Byte Test Book', got '%s'", book.Title)
	}

	if len(book.Authors) != 1 || book.Authors[0] != "Author Name" {
		t.Errorf("expected author 'Author Name', got %v", book.Authors)
	}

	if book.Lang != "ru" {
		t.Errorf("expected language 'ru', got '%s'", book.Lang)
	}
}

func TestParseEPUBMinimal(t *testing.T) {
	containerXML := []byte(`<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="package.opf"/>
  </rootfiles>
</container>`)

	opfXML := []byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Minimal Book</dc:title>
  </metadata>
</package>`)

	tmpDir := t.TempDir()
	epubPath := filepath.Join(tmpDir, "minimal.epub")

	zipFile, _ := os.Create(epubPath)
	zipWriter := zip.NewWriter(zipFile)
	cw, _ := zipWriter.Create("META-INF/container.xml")
	cw.Write(containerXML)
	pw, _ := zipWriter.Create("package.opf")
	pw.Write(opfXML)
	zipWriter.Close()
	zipFile.Close()

	book, err := ParseEPUB(epubPath)
	if err != nil {
		t.Fatalf("failed to parse minimal EPUB: %v", err)
	}

	if book.Title != "Minimal Book" {
		t.Errorf("expected title 'Minimal Book', got '%s'", book.Title)
	}

	if len(book.Authors) != 0 {
		t.Errorf("expected 0 authors, got %d", len(book.Authors))
	}
}

func TestParseEPUBInvalidFile(t *testing.T) {
	_, err := ParseEPUB("/nonexistent/file.epub")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseEPUBInvalidZip(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.epub")
	os.WriteFile(invalidPath, []byte("not a zip file"), 0644)

	_, err := ParseEPUB(invalidPath)
	if err == nil {
		t.Error("expected error for invalid zip file")
	}
}

func TestGetEPUBFormatName(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"book.epub", "EPUB"},
		{"book.EPUB", "EPUB"},
		{"book.epub.zip", "EPUB.ZIP"},
		{"book.zip", "ZIP"},
		{"book.fb2", "FB2"},
	}

	for _, test := range tests {
		result := GetEPUBFormatName(test.filename)
		if result != test.expected {
			t.Errorf("GetEPUBFormatName(%s) = %s, expected %s", test.filename, result, test.expected)
		}
	}
}

func TestReadZipFileFromBytes(t *testing.T) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	w, _ := zipWriter.Create("test.txt")
	w.Write([]byte("hello world"))
	zipWriter.Close()

	data := buf.Bytes()

	content, err := ReadZipFileFromBytes(data, "test.txt")
	if err != nil {
		t.Fatalf("failed to read file from bytes: %v", err)
	}

	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(content))
	}

	_, err = ReadZipFileFromBytes(data, "nonexistent.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
