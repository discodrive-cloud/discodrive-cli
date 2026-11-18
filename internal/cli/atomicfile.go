package cli

import (
	"io"
	"os"

	"github.com/discodrive-cloud/discodrive-cli/internal/atomicfile"
)

// writeFileAtomic: see atomicfile.Write.
func writeFileAtomic(dst string, write func(io.Writer) error) error {
	return atomicfile.Write(dst, write)
}

// writeFileAtomicMode: see atomicfile.WriteMode.
func writeFileAtomicMode(dst string, mode os.FileMode, write func(io.Writer) error) error {
	return atomicfile.WriteMode(dst, mode, write)
}
