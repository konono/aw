package embed

import "embed"

//go:embed check-inbox.sh
var CheckInboxScript []byte

//go:embed all:scripts
var Scripts embed.FS
