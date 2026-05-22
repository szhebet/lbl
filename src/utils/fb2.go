package utils

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FB2Book struct {
	Title     string
	Authors   []string
	Lang      string
	Year      string
	ISBN      string
	Publisher string
	Genres    []string
	Annotation string
	Sequence  string
	SequenceNumber string
}

type fb2Description struct {
	TitleInfo struct {
		Genre             []string `xml:"genre"`
		Author            []struct {
			FirstName string `xml:"first-name"`
			LastName  string `xml:"last-name"`
		} `xml:"author"`
		BookTitle  string `xml:"book-title"`
		Lang       string `xml:"lang"`
		Date       string `xml:"date"`
		ISBN       string `xml:"isbn"`
		Publisher  string `xml:"publisher"`
		Sequence   string `xml:"sequence"`
	} `xml:"title-info"`
	DocumentInfo struct {
		Author struct {
			Nickname string `xml:"nickname"`
		} `xml:"author"`
	} `xml:"document-info"`
	PublishInfo struct {
		ISBN string `xml:"isbn"`
	} `xml:"publish-info"`
}

func ParseFB2(filePath string) (*FB2Book, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if startElement, ok := token.(xml.StartElement); ok {
			if startElement.Name.Local == "description" {
				var desc fb2Description
				if err := decoder.DecodeElement(&desc, &startElement); err != nil {
					return nil, fmt.Errorf("failed to parse description: %w", err)
				}

				book := &FB2Book{
					Title:    desc.TitleInfo.BookTitle,
					Lang:     desc.TitleInfo.Lang,
					Year:     extractYear(desc.TitleInfo.Date),
					ISBN:     getISBN(desc),
					Publisher: getPublisher(desc),
					Genres:   desc.TitleInfo.Genre,
				}

				for _, author := range desc.TitleInfo.Author {
					name := strings.TrimSpace(fmt.Sprintf("%s %s", author.FirstName, author.LastName))
					if name != "" {
						book.Authors = append(book.Authors, name)
					}
				}

				if desc.TitleInfo.Sequence != "" {
					parts := strings.Split(desc.TitleInfo.Sequence, "@")
					if len(parts) > 0 {
						book.Sequence = strings.TrimSpace(parts[0])
					}
					if len(parts) > 1 {
						book.SequenceNumber = strings.TrimSpace(parts[1])
					}
				}

				return book, nil
			}
		}
	}

	return nil, fmt.Errorf("description not found in FB2 file")
}

func ParseFB2FromBytes(data []byte) (*FB2Book, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if startElement, ok := token.(xml.StartElement); ok {
			if startElement.Name.Local == "description" {
				var desc fb2Description
				if err := decoder.DecodeElement(&desc, &startElement); err != nil {
					return nil, fmt.Errorf("failed to parse description: %w", err)
				}

				book := &FB2Book{
					Title:    desc.TitleInfo.BookTitle,
					Lang:     desc.TitleInfo.Lang,
					Year:     extractYear(desc.TitleInfo.Date),
					ISBN:     getISBN(desc),
					Publisher: getPublisher(desc),
					Genres:   desc.TitleInfo.Genre,
				}

				for _, author := range desc.TitleInfo.Author {
					name := strings.TrimSpace(fmt.Sprintf("%s %s", author.FirstName, author.LastName))
					if name != "" {
						book.Authors = append(book.Authors, name)
					}
				}

				if desc.TitleInfo.Sequence != "" {
					parts := strings.Split(desc.TitleInfo.Sequence, "@")
					if len(parts) > 0 {
						book.Sequence = strings.TrimSpace(parts[0])
					}
					if len(parts) > 1 {
						book.SequenceNumber = strings.TrimSpace(parts[1])
					}
				}

				return book, nil
			}
		}
	}

	return nil, fmt.Errorf("description not found in FB2 file")
}

func ParseFB2FromZip(filePath string) (*FB2Book, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		ext := strings.ToLower(filepath.Ext(file.Name))
		if ext == ".fb2" || ext == ".xml" {
			rc, err := file.Open()
			if err != nil {
				continue
			}

			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			book, err := ParseFB2FromBytes(data)
			if err == nil {
				return book, nil
			}
		}
	}

	return nil, fmt.Errorf("no FB2 file found in archive")
}

func extractYear(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	if len(dateStr) >= 4 {
		return dateStr[:4]
	}
	return dateStr
}

func getISBN(desc fb2Description) string {
	if desc.PublishInfo.ISBN != "" {
		return desc.PublishInfo.ISBN
	}
	return desc.TitleInfo.ISBN
}

func getPublisher(desc fb2Description) string {
	return desc.TitleInfo.Publisher
}

func NormalizeLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "ru", "ру", "русский":
		return "rus"
	case "en", "eng", "английский", "english":
		return "eng"
	case "de", "deu", "немецкий", "german":
		return "deu"
	case "fr", "fra", "французский", "french":
		return "fra"
	case "es", "spa", "испанский", "spanish":
		return "spa"
	default:
		if len(lang) == 3 {
			return lang
		}
		if len(lang) == 2 {
			return lang + "u"
		}
		return "" // Return empty for unknown - let caller handle default
	}
}

func NormalizeTitle(title string) string {
	return strings.TrimSpace(title)
}

func NormalizeAuthorName(author string) (firstName, lastName string) {
	author = strings.TrimSpace(author)
	
	parts := strings.Fields(author)
	if len(parts) == 1 {
		return "", parts[0]
	}
	
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	
	lastName = parts[len(parts)-1]
	firstName = strings.Join(parts[:len(parts)-1], " ")
	
	return firstName, lastName
}