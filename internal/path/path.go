package path

import (
	"os"
	"path/filepath"
	"runtime"

	internalos "github.com/ayoisaiah/f2/internal/os"
)

// Collection represents directory paths and their respective contents.
type Collection map[string][]os.DirEntry

// Separator represents the filepath separator.
var Separator = "/"

func init() {
	if runtime.GOOS == internalos.Windows {
		Separator = `\`
	}
}

// StripExtension returns the input file name without its extension.
func StripExtension(fileName string) string {
	return fileName[:len(fileName)-len(filepath.Ext(fileName))]
}
