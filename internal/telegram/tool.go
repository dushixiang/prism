package telegram

import "regexp"

// escapeMarkdownV2 用于转义 MarkdownV2 格式中的特殊字符
func escapeMarkdownV2(input string) string {
	// 需要转义的特殊字符
	specialChars := []struct {
		char   string
		escape string
	}{
		{"\\", "\\\\"},
		{"*", "\\*"},
		{"_", "\\_"},
		{"`", "\\`"},
		{"{", "\\{"},
		{"}", "\\}"},
		{"[", "\\["},
		{"]", "\\]"},
		{"(", "\\("},
		{")", "\\)"},
		{"~", "\\~"},
		{">", "\\>"},
		{"#", "\\#"},
		{"+", "\\+"},
		{"-", "\\-"},
		{"=", "\\="},
		{"|", "\\|"},
		{".", "\\."},
	}

	// 遍历并转义每个字符
	for _, sc := range specialChars {
		// 使用正则替换特殊字符
		re := regexp.MustCompile(regexp.QuoteMeta(sc.char))
		input = re.ReplaceAllString(input, sc.escape)
	}

	return input
}
