package utils

import "strings"

var cyrillicToLatin = map[rune]string{
	'А': "A", 'а': "a",
	'Б': "B", 'б': "b",
	'В': "V", 'в': "v",
	'Г': "G", 'г': "g",
	'Д': "D", 'д': "d",
	'Е': "E", 'е': "e",
	'Ё': "Yo", 'ё': "yo",
	'Ж': "Zh", 'ж': "zh",
	'З': "Z", 'з': "z",
	'И': "I", 'и': "i",
	'Й': "Y", 'й': "y",
	'К': "K", 'к': "k",
	'Л': "L", 'л': "l",
	'М': "M", 'м': "m",
	'Н': "N", 'н': "n",
	'О': "O", 'о': "o",
	'П': "P", 'п': "p",
	'Р': "R", 'р': "r",
	'С': "S", 'с': "s",
	'Т': "T", 'т': "t",
	'У': "U", 'у': "u",
	'Ф': "F", 'ф': "f",
	'Х': "Kh", 'х': "kh",
	'Ц': "Ts", 'ц': "ts",
	'Ч': "Ch", 'ч': "ch",
	'Ш': "Sh", 'ш': "sh",
	'Щ': "Shch", 'щ': "shch",
	'Ъ': "", 'ъ': "",
	'Ы': "Y", 'ы': "y",
	'Ь': "", 'ь': "",
	'Э': "E", 'э': "e",
	'Ю': "Yu", 'ю': "yu",
	'Я': "Ya", 'я': "ya",
}

var latinToLatin = map[rune]string{
	'À': "A", 'Á': "A", 'Â': "A", 'Ã': "A", 'Ä': "A", 'Å': "A", 'Ā': "A", 'Ă': "A", 'Ą': "A",
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'È': "E", 'É': "E", 'Ê': "E", 'Ë': "E", 'Ē': "E", 'Ĕ': "E", 'Ė': "E", 'Ę': "E", 'Ě': "E",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ĕ': "e", 'ė': "e", 'ę': "e", 'ě': "e",
	'Ì': "I", 'Í': "I", 'Î': "I", 'Ï': "I", 'Ĩ': "I", 'Ī': "I", 'Ĭ': "I", 'Į': "I", 'İ': "I",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ĩ': "i", 'ī': "i", 'ĭ': "i", 'į': "i", 'ı': "i",
	'Ò': "O", 'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O", 'Ø': "O", 'Ō': "O", 'Ŏ': "O", 'Ő': "O",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ŏ': "o", 'ő': "o",
	'Ù': "U", 'Ú': "U", 'Û': "U", 'Ü': "U", 'Ũ': "U", 'Ū': "U", 'Ŭ': "U", 'Ů': "U", 'Ű': "U", 'Ų': "U",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ũ': "u", 'ū': "u", 'ŭ': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'Ý': "Y", 'ý': "y", 'ÿ': "y", 'Ÿ': "Y",
	'Ñ': "N", 'ñ': "n",
	'Ç': "C", 'ç': "c", 'Ć': "C", 'ć': "c", 'Ĉ': "C", 'ĉ': "c", 'Ċ': "C", 'ċ': "c", 'Č': "C", 'č': "c",
	'Ğ': "G", 'ğ': "g", 'Ģ': "G", 'ģ': "g",
	'Ł': "L", 'ł': "l", 'Ļ': "L", 'ļ': "l",
	'Ş': "S", 'ş': "s", 'Ś': "S", 'ś': "s", 'Ŝ': "S", 'ŝ': "s", 'Š': "S", 'š': "s",
	'Ţ': "T", 'ţ': "t", 'Ť': "T", 'ť': "t",
	'Ž': "Z", 'ž': "z", 'Ź': "Z", 'ź': "z", 'Ż': "Z", 'ż': "z",
	'Ð': "D", 'ð': "d", 'Þ': "Th", 'þ': "th",
	'Æ': "Ae", 'æ': "ae", 'Œ': "Oe", 'œ': "oe",
	'ß': "ss",
	'Ĳ': "IJ", 'ĳ': "ij",
	'ƒ': "f",
}

// Transliterate converts a string to ASCII, mapping Cyrillic and accented Latin characters to their nearest ASCII equivalents.
// Non-alphanumeric characters (except hyphen and underscore) are replaced with underscores.
func Transliterate(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			result.WriteRune(r)
		default:
			if latin, ok := latinToLatin[r]; ok {
				result.WriteString(latin)
			} else if cyr, ok := cyrillicToLatin[r]; ok {
				result.WriteString(cyr)
			} else {
				result.WriteRune('_')
			}
		}
	}

	return result.String()
}

// TransliterateFilename converts a filename (without extension) to a safe ASCII string suitable for use in file paths.
func TransliterateFilename(name string) string {
	result := Transliterate(name)

	result = strings.Trim(result, "_")
	if result == "" {
		result = "book"
	}

	return result
}
