package utils

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type EPUBBook struct {
	Title     string
	Authors   []string
	Lang      string
	Year      string
	ISBN      string
	Publisher string
	Genres    []string
	Annotation string
	Sequence  string
}

type epubContainer struct {
	XMLName  xml.Name `xml:"container"`
	RootFile struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type epubPackage struct {
	XMLName   xml.Name    `xml:"package"`
	Metadata  epubMetadata `xml:"metadata"`
	Manifest  epubManifest `xml:"manifest"`
	Spine     epubSpine    `xml:"spine"`
}

type epubMetadata struct {
	Titles       []string         `xml:"title"`
	Creators     []epubCreator    `xml:"creator"`
	Contributors []epubContributor `xml:"contributor"`
	Descriptions []string         `xml:"description"`
	Publishers   []string         `xml:"publisher"`
	Dates        []epubDate       `xml:"date"`
	Identifiers  []epubIdentifier `xml:"identifier"`
	Subjects     []string         `xml:"subject"`
	Languages    []string         `xml:"language"`
}

type epubCreator struct {
	Value string `xml:",chardata"`
	Role  string `xml:"role,attr"`
}

type epubContributor struct {
	Value string `xml:",chardata"`
	Role  string `xml:"role,attr"`
}

type epubDate struct {
	Value string `xml:",chardata"`
	Event string `xml:"event,attr"`
}

type epubIdentifier struct {
	Value    string `xml:",chardata"`
	Scheme   string `xml:"scheme,attr"`
	ID       string `xml:"id,attr"`
}

type epubManifest struct {
	Items []epubManifestItem `xml:"item"`
}

type epubManifestItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type epubSpine struct {
	Items []epubSpineItem `xml:"itemref"`
}

type epubSpineItem struct {
	IDRef string `xml:"idref,attr"`
}

func ParseEPUB(filePath string) (*EPUBBook, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open epub: %w", err)
	}
	defer reader.Close()

	return parseEPUBFromReadCloser(reader)
}

func ParseEPUBFromBytes(data []byte) (*EPUBBook, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open epub from bytes: %w", err)
	}

	return parseEPUBFromReader(reader)
}

func parseEPUBFromReadCloser(reader *zip.ReadCloser) (*EPUBBook, error) {
	containerPath := "META-INF/container.xml"
	containerData, err := readZipFileFromCloser(reader, containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read container.xml: %w", err)
	}

	var container epubContainer
	if err := xml.Unmarshal(containerData, &container); err != nil {
		return nil, fmt.Errorf("failed to parse container.xml: %w", err)
	}

	if container.RootFile.FullPath == "" {
		return nil, fmt.Errorf("no rootfile path found in container.xml")
	}

	opfPath := container.RootFile.FullPath
	opfData, err := readZipFileFromCloser(reader, opfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OPF file %s: %w", opfPath, err)
	}

	return parseOPF(opfData)
}

func parseEPUBFromReader(reader *zip.Reader) (*EPUBBook, error) {
	containerPath := "META-INF/container.xml"
	containerData, err := readZipFileFromReader(reader, containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read container.xml: %w", err)
	}

	var container epubContainer
	if err := xml.Unmarshal(containerData, &container); err != nil {
		return nil, fmt.Errorf("failed to parse container.xml: %w", err)
	}

	if container.RootFile.FullPath == "" {
		return nil, fmt.Errorf("no rootfile path found in container.xml")
	}

	opfPath := container.RootFile.FullPath
	opfData, err := readZipFileFromReader(reader, opfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OPF file %s: %w", opfPath, err)
	}

	return parseOPF(opfData)
}

func parseOPF(data []byte) (*EPUBBook, error) {
	var pkg epubPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse OPF: %w", err)
	}

	book := &EPUBBook{}

	if len(pkg.Metadata.Titles) > 0 {
		book.Title = strings.TrimSpace(pkg.Metadata.Titles[0])
	}

	for _, creator := range pkg.Metadata.Creators {
		if creator.Role == "" || creator.Role == "aut" {
			name := strings.TrimSpace(creator.Value)
			if name != "" {
				book.Authors = append(book.Authors, name)
			}
		}
	}

	if len(book.Authors) == 0 {
		for _, contributor := range pkg.Metadata.Contributors {
			if contributor.Role == "" || contributor.Role == "aut" {
				name := strings.TrimSpace(contributor.Value)
				if name != "" {
					book.Authors = append(book.Authors, name)
				}
			}
		}
	}

	if len(pkg.Metadata.Languages) > 0 {
		book.Lang = pkg.Metadata.Languages[0]
	}

	for _, date := range pkg.Metadata.Dates {
		if date.Event == "publication" || date.Event == "" {
			book.Year = extractYear(date.Value)
			break
		}
	}

	for _, identifier := range pkg.Metadata.Identifiers {
		if strings.EqualFold(identifier.Scheme, "ISBN") || strings.EqualFold(identifier.ID, "isbn") {
			book.ISBN = strings.TrimSpace(identifier.Value)
			break
		}
	}

	if book.ISBN == "" {
		for _, identifier := range pkg.Metadata.Identifiers {
			val := strings.TrimSpace(identifier.Value)
			digits := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, val)
			if len(digits) == 10 || len(digits) == 13 {
				book.ISBN = val
				break
			}
		}
	}

	if len(pkg.Metadata.Publishers) > 0 {
		book.Publisher = strings.TrimSpace(pkg.Metadata.Publishers[0])
	}

	if len(pkg.Metadata.Descriptions) > 0 {
		book.Annotation = strings.TrimSpace(pkg.Metadata.Descriptions[0])
	}

	book.Genres = pkg.Metadata.Subjects

	return book, nil
}

func readZipFileFromCloser(reader *zip.ReadCloser, name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "./")
	for _, f := range reader.File {
		if strings.TrimPrefix(f.Name, "./") == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file %s not found in archive", name)
}

func readZipFileFromReader(reader *zip.Reader, name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "./")
	for _, f := range reader.File {
		if strings.TrimPrefix(f.Name, "./") == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file %s not found in archive", name)
}

func ReadZipFileFromBytes(data []byte, filename string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}

	for _, f := range reader.File {
		if f.Name == filename {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file %s not found in archive", filename)
}

func GetEPUBFormatName(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".epub":
		return "EPUB"
	case ".zip":
		if strings.HasSuffix(strings.ToLower(filename), ".epub.zip") {
			return "EPUB.ZIP"
		}
		return "ZIP"
	default:
		return strings.ToUpper(strings.TrimPrefix(ext, "."))
	}
}
