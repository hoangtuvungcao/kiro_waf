package handlers

// buildContentPages returns all documentation content pages organized by language and page ID.
func buildContentPages() map[string]map[string]DocsContentPage {
	return map[string]map[string]DocsContentPage{
		"en": buildEnglishPages(),
		"vi": buildVietnamesePages(),
	}
}
