package picker

import "errors"

var ErrCancelled = errors.New("selection cancelled")

type Options struct {
	Query string
}

var pickFunc func(items []string, opts Options) (string, error) = fzfPick

func Pick(items []string, opts Options) (string, error) {
	return pickFunc(items, opts)
}
