//go:build windows

package appicon

import _ "embed"

//go:embed queue_up.ico
var iconBytes []byte

func Bytes() []byte {
	return iconBytes
}
