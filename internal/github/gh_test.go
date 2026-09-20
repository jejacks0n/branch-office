package github
 
import (
	"testing"
)

func TestCleanCommitMessage(t *testing.T) {
	input := `I have enough context to write the commit message.

feat(howto): add caution field with callout rendering

- Add optional caution field to howto card, rendered as an aside paragraph above the steps list
- Update card schema, generated types, and cedar-house fixture with caution/blocks output
- Add caution copy (label, placeholder, hint, careful) to en/es/fr/de locales
- Add test verifying caution renders as a set-apart aside block before the steps`

	expected := `feat(howto): add caution field with callout rendering

- Add optional caution field to howto card, rendered as an aside paragraph above the steps list
- Update card schema, generated types, and cedar-house fixture with caution/blocks output
- Add caution copy (label, placeholder, hint, careful) to en/es/fr/de locales
- Add test verifying caution renders as a set-apart aside block before the steps`

	got := CleanCommitMessage(input)
	if got != expected {
		t.Fatalf("expected:\n%s\n\ngot:\n%s", expected, got)
	}

	// Test with markdown code block
	markdownInput := "```\n" + input + "\n```"
	gotMd := CleanCommitMessage(markdownInput)
	if gotMd != expected {
		t.Fatalf("markdown wrapped test failed: expected:\n%s\n\ngot:\n%s", expected, gotMd)
	}

	// Test with "Here is the commit message:"
	hereIsInput := "Here is the commit message:\n\n" + expected
	gotHereIs := CleanCommitMessage(hereIsInput)
	if gotHereIs != expected {
		t.Fatalf("here is test failed: expected:\n%s\n\ngot:\n%s", expected, gotHereIs)
	}

	// Test stripping Co-authored-by lines
	coauthorInput := expected + "\n\nCo-authored-by: Copilot <223556530+copilot[bot]@users.noreply.github.com>"
	gotCoauthor := CleanCommitMessage(coauthorInput)
	if gotCoauthor != expected {
		t.Fatalf("co-authored-by test failed: expected:\n%s\n\ngot:\n%s", expected, gotCoauthor)
	}

	// Test stripping multiple Co-authored-by variations and bullets
	multiCoauthorInput := expected + "\n\nCo-authored-by: Copilot <copilot@github.com>\nCo-Authored-By: Octocat <octo@github.com>\n- Co-authored-by: Test <test@example.com>"
	gotMultiCoauthor := CleanCommitMessage(multiCoauthorInput)
	if gotMultiCoauthor != expected {
		t.Fatalf("multi co-author test failed: expected:\n%s\n\ngot:\n%s", expected, gotMultiCoauthor)
	}
}
