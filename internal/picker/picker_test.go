package picker

import "testing"

func TestPick_MockPassesOptions(t *testing.T) {
	origPickFunc := pickFunc
	defer func() { pickFunc = origPickFunc }()

	var capturedItems []string
	var capturedOpts Options
	pickFunc = func(items []string, opts Options) (string, error) {
		capturedItems = items
		capturedOpts = opts
		return items[0], nil
	}

	result, err := Pick([]string{"/a", "/b"}, Options{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "/a" {
		t.Errorf("got %q, want %q", result, "/a")
	}
	if len(capturedItems) != 2 {
		t.Errorf("got %d items, want 2", len(capturedItems))
	}
	if capturedOpts.Query != "test" {
		t.Errorf("got query %q, want %q", capturedOpts.Query, "test")
	}
}

func TestPick_MockCancelled(t *testing.T) {
	origPickFunc := pickFunc
	defer func() { pickFunc = origPickFunc }()

	pickFunc = func(items []string, opts Options) (string, error) {
		return "", ErrCancelled
	}

	_, err := Pick([]string{"/a"}, Options{})
	if err != ErrCancelled {
		t.Errorf("expected ErrCancelled, got %v", err)
	}
}
