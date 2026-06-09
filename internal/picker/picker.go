package picker

import "errors"

var ErrCancelled = errors.New("selection cancelled")

type Options struct {
	Query string
}

type PickFunc func(items []string, opts Options) (string, error)

var pickFunc PickFunc = fzfPick

func Pick(items []string, opts Options) (string, error) {
	return pickFunc(items, opts)
}
