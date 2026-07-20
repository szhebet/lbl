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
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
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
		Annotation struct {
			Inner string `xml:",innerxml"`
		} `xml:"annotation"`
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
			return nil, fmt.Errorf("parse fb2 stream: %w", err)
		}

		if startElement, ok := token.(xml.StartElement); ok {
			if startElement.Name.Local == "description" {
				var desc fb2Description
				if err := decoder.DecodeElement(&desc, &startElement); err != nil {
					return nil, fmt.Errorf("failed to parse description: %w", err)
				}

				book := &FB2Book{
					Title:      desc.TitleInfo.BookTitle,
					Lang:       desc.TitleInfo.Lang,
					Year:       extractYear(desc.TitleInfo.Date),
					ISBN:       getISBN(desc),
					Publisher:  getPublisher(desc),
					Genres:     desc.TitleInfo.Genre,
					Annotation: stripXMLTags(desc.TitleInfo.Annotation.Inner),
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

	descStart := bytes.Index(data, []byte("<description>"))
	if descStart < 0 {
		descStart = bytes.Index(data, []byte("<description"))
		if descStart < 0 {
			return nil, fmt.Errorf("description tag not found in FB2 file")
		}
	}
	descData := data[descStart:]
	descEnd := bytes.Index(descData, []byte("</description>"))
	if descEnd < 0 {
		return nil, fmt.Errorf("closing description tag not found in FB2 file")
	}
	descData = descData[:descEnd+len("</description>")]

	descData = decodeFB2ToUTF8(data, descData)

	var desc fb2Description
	decoder := xml.NewDecoder(bytes.NewReader(descData))
	decoder.Strict = false
	if err := decoder.Decode(&desc); err != nil {
		return nil, fmt.Errorf("failed to parse description: %w", err)
	}

	if len(desc.TitleInfo.Genre) == 0 && desc.TitleInfo.BookTitle == "" {
		return nil, fmt.Errorf("empty title-info in FB2 description")
	}

	book := &FB2Book{
		Title:      desc.TitleInfo.BookTitle,
		Lang:       desc.TitleInfo.Lang,
		Year:       extractYear(desc.TitleInfo.Date),
		ISBN:       getISBN(desc),
		Publisher:  desc.TitleInfo.Publisher,
		Genres:     desc.TitleInfo.Genre,
		Annotation: stripXMLTags(desc.TitleInfo.Annotation.Inner),
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

func ExtractFB2FromZipBytes(data []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}

	for _, file := range reader.File {
		ext := strings.ToLower(filepath.Ext(file.Name))
		if ext == ".fb2" || ext == ".xml" {
			rc, err := file.Open()
			if err != nil {
				continue
			}

			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			return content, nil
		}
	}

	return nil, fmt.Errorf("no FB2 file found in archive")
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

// stripXMLTags removes XML tags from inner content and collapses whitespace,
// producing a plain-text annotation from FB2/EPUB markup.
func stripXMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
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
	// Handle regional variants (e.g. "ru-RU" → "ru")
	if idx := strings.Index(lang, "-"); idx > 0 {
		lang = lang[:idx]
	}
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
	case "it", "ita", "итальянский", "italian":
		return "ita"
	case "jp", "jpn", "японский", "japanese":
		return "jpn"
	case "zh", "chi", "китайский", "chinese":
		return "chi"
	case "ar", "ara", "арабский", "arabic":
		return "ara"
	case "pt", "por", "португальский", "portuguese":
		return "por"
	case "uk", "ukr", "украинский", "ukrainian":
		return "ukr"
	case "be", "bel", "белорусский", "belarusian":
		return "bel"
	default:
		if len(lang) == 3 {
			return lang
		}
		return ""
	}
}

func NormalizeTitle(title string) string {
	return strings.TrimSpace(title)
}

func NormalizeAuthorName(author string) (firstName, lastName string) {
	author = strings.TrimSpace(author)

	if strings.Contains(author, ",") {
		parts := strings.SplitN(author, ",", 2)
		lastName = strings.TrimSpace(parts[0])
		firstName = strings.TrimSpace(parts[1])
		return firstName, lastName
	}

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

func decodeFB2ToUTF8(fullData, descData []byte) []byte {
	if utf8.Valid(descData) {
		return descData
	}
	enc := detectFB2Encoding(fullData)
	if enc == nil {
		return descData
	}
	reader := transform.NewReader(bytes.NewReader(descData), enc.NewDecoder())
	out, err := io.ReadAll(reader)
	if err != nil {
		return descData
	}
	return out
}

func detectFB2Encoding(data []byte) encoding.Encoding {
	start := bytes.Index(data, []byte("<?xml"))
	if start < 0 {
		return nil
	}
	end := bytes.Index(data[start:], []byte("?>"))
	if end < 0 {
		return nil
	}
	decl := string(data[start : start+end+2])

	encName := extractEncoding(decl)
	if encName == "" || strings.EqualFold(encName, "utf-8") || strings.EqualFold(encName, "utf8") {
		return nil
	}

	switch {
	case strings.Contains(encName, "1251") || strings.Contains(encName, "cp1251") || strings.Contains(encName, "windows-1251"):
		return charmap.Windows1251
	case strings.Contains(encName, "1252") || strings.Contains(encName, "windows-1252"):
		return charmap.Windows1252
	case strings.Contains(encName, "koi8") || strings.Contains(encName, "koi8-r"):
		return charmap.KOI8R
	case strings.Contains(encName, "iso-8859-1") || strings.Contains(encName, "latin1"):
		return charmap.ISO8859_1
	case strings.Contains(encName, "iso-8859-2") || strings.Contains(encName, "latin2"):
		return charmap.ISO8859_2
	case strings.Contains(encName, "iso-8859-5"):
		return charmap.ISO8859_5
	case strings.Contains(encName, "iso-8859-15"):
		return charmap.ISO8859_15
	case strings.Contains(encName, "gbk") || strings.Contains(encName, "gb2312") || strings.Contains(encName, "gb18030"):
		return simplifiedchinese.GBK
	case strings.Contains(encName, "big5"):
		return traditionalchinese.Big5
	case strings.Contains(encName, "euc-kr") || strings.Contains(encName, "ks_c"):
		return korean.EUCKR
	case strings.Contains(encName, "euc-jp") || strings.Contains(encName, "shift_jis") || strings.Contains(encName, "iso-2022-jp"):
		return japanese.EUCJP
	case strings.Contains(encName, "utf-16"):
		return nil
	}
	return nil
}

func extractEncoding(decl string) string {
	start := strings.Index(strings.ToLower(decl), "encoding")
	if start < 0 {
		return ""
	}
	eq := strings.IndexByte(decl[start:], '=')
	if eq < 0 {
		return ""
	}
	val := decl[start+eq+1:]
	val = strings.TrimSpace(val)
	if len(val) > 0 && (val[0] == '"' || val[0] == '\'') {
		quote := val[0]
		val = val[1:]
		end := strings.IndexByte(val, byte(quote))
		if end < 0 {
			return ""
		}
		val = val[:end]
	}
	return strings.TrimSpace(val)
}