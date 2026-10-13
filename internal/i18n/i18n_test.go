package i18n

import (
	"reflect"
	"regexp"
	"testing"
)

// TestLanguageSelection verifies supported languages and English fallback behavior.
func TestLanguageSelection(t *testing.T) {
	SetLanguage(English)
	if got := T("Add profile"); got != "Add profile" {
		t.Fatalf("English translation is %q, want Add profile", got)
	}

	SetLanguage(Ukrainian)
	if got := T("Add profile"); got != "Додати профіль" {
		t.Fatalf("Ukrainian translation is %q, want Додати профіль", got)
	}
	if got := T("untranslated phrase"); got != "untranslated phrase" {
		t.Fatalf("missing translation fallback is %q, want the source phrase", got)
	}
	if got := Normalize("unsupported"); got != English {
		t.Fatalf("unsupported language normalized to %q, want English", got)
	}
	SetLanguage(English)
}

// TestTranslationPlaceholdersMatch verifies that translations preserve formatting verbs.
func TestTranslationPlaceholdersMatch(t *testing.T) {
	verbs := regexp.MustCompile(`%[-+#0-9.*]*[a-zA-Z]`)
	for source, translation := range ukrainian {
		sourceVerbs := verbs.FindAllString(source, -1)
		translationVerbs := verbs.FindAllString(translation, -1)
		if !reflect.DeepEqual(sourceVerbs, translationVerbs) {
			t.Errorf("placeholders for %q are %v, want %v", source, translationVerbs, sourceVerbs)
		}
	}
	for source, translation := range additionalUkrainian {
		sourceVerbs := verbs.FindAllString(source, -1)
		translationVerbs := verbs.FindAllString(translation, -1)
		if !reflect.DeepEqual(sourceVerbs, translationVerbs) {
			t.Errorf("placeholders for %q are %v, want %v", source, translationVerbs, sourceVerbs)
		}
	}
}
