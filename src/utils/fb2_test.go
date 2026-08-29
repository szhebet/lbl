package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFB2(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{"Strugatsky book", "ponedelnikNachVSubbotu.fb2", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(os.Getenv("HOME"), "git/aitest/agents/lbl/example", tc.filename)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Skip("Test file not found:", filePath)
			}

			book, err := ParseFB2(filePath)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseFB2() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			if !tc.wantErr {
				if book.Title == "" {
					t.Error("Expected non-empty title")
				}
				if len(book.Authors) == 0 {
					t.Error("Expected at least one author")
				}
				t.Logf("Parsed: Title=%s, Authors=%v, Lang=%s, Year=%s, ISBN=%s",
					book.Title, book.Authors, book.Lang, book.Year, book.ISBN)
			}
		})
	}
}

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ru", "rus"},
		{"ру", "rus"},
		{"rus", "rus"},
		{"en", "eng"},
		{"eng", "eng"},
		{"de", "deu"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeLanguage(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeLanguage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeAuthorName(t *testing.T) {
	tests := []struct {
		input         string
		expectedFirst string
		expectedLast  string
	}{
		{"Аркадий и Борис Стругацкие", "Аркадий и Борис", "Стругацкие"},
		{"Иван Тургенев", "Иван", "Тургенев"},
		{"Пушкин", "", "Пушкин"},
		{"А. С. Пушкин", "А. С.", "Пушкин"},
		{"Чалдини, Роберт", "Роберт", "Чалдини"},
		{"Талеб, Нассим Николас", "Нассим Николас", "Талеб"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			first, last := NormalizeAuthorName(tt.input)
			if first != tt.expectedFirst {
				t.Errorf("NormalizeAuthorName(%q) first = %q, want %q", tt.input, first, tt.expectedFirst)
			}
			if last != tt.expectedLast {
				t.Errorf("NormalizeAuthorName(%q) last = %q, want %q", tt.input, last, tt.expectedLast)
			}
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  Понедельник начинается в субботу  ", "Понедельник начинается в субботу"},
		{"Title", "Title"},
		{"  ", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeTitle(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseFB2FromBytes(t *testing.T) {
	testFile := filepath.Join(os.Getenv("HOME"), "git/aitest/agents/lbl/example", "ponedelnikNachVSubbotu.fb2")

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Skip("Test file not found:", testFile)
	}

	book, err := ParseFB2FromBytes(data)
	if err != nil {
		t.Fatalf("ParseFB2FromBytes() error = %v", err)
	}

	if book.Title != "Понедельник начинается в субботу" {
		t.Errorf("Expected title 'Понедельник начинается в субботу', got '%s'", book.Title)
	}

	if len(book.Authors) != 1 {
		t.Errorf("Expected 1 author, got %d", len(book.Authors))
	}

	if book.Lang != "ru" {
		t.Errorf("Expected lang 'ru', got '%s'", book.Lang)
	}

	t.Logf("Parsed book: %+v", book)
}

func TestParseFB2InvalidFile(t *testing.T) {
	invalidData := []byte(`<?xml version="1.0"?><book><title>Test</title></book>`)

	_, err := ParseFB2FromBytes(invalidData)
	if err == nil {
		t.Error("Expected error for invalid FB2 file")
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		dateStr  string
		expected string
	}{
		{"1964", "1964"},
		{"1964-01-01", "1964"},
		{"", ""},
		{"2023-12-31", "2023"},
	}

	for _, tt := range tests {
		t.Run(tt.dateStr, func(t *testing.T) {
			result := extractYear(tt.dateStr)
			if result != tt.expected {
				t.Errorf("extractYear(%q) = %q, want %q", tt.dateStr, result, tt.expected)
			}
		})
	}
}
