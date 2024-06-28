package extract

import (
	"io/fs"
	"math"
	"os"
	"path"
	"strings"

	"knative.dev/hack"
)

const (
	// HackScriptsDirEnvVar is the name of the environment variable that points
	// to directory where knative-hack scripts will be extracted.
	HackScriptsDirEnvVar = "KNATIVE_HACK_SCRIPTS_DIR"
	// PermOwnerWrite is the permission bits for owner write.
	PermOwnerWrite = 0o200
	// PermAllExecutable is the permission bits for executable.
	PermAllExecutable = 0o111
)

// Printer is an interface for printing messages.
type Printer interface {
	Print(i ...interface{})
	Println(i ...interface{})
	Printf(format string, i ...interface{})
	PrintErr(i ...interface{})
	PrintErrln(i ...interface{})
	PrintErrf(format string, i ...interface{})
}

// Operation is the main extract object that can extract scripts.
type Operation struct {
	// ScriptName is the name of the script to extract.
	ScriptName string
	// Verbose will print more information.
	Verbose bool
}

// Extract will extract a script from the library to a temporary directory and
// provide the file path to it.
func (o Operation) Extract(prtr Printer) error {
	l := logger{o.Verbose, prtr}
	hackRootDir := os.Getenv(HackScriptsDirEnvVar)
	if f, err := hack.Scripts.Open(o.ScriptName); err != nil {
		return wrapErr(err, ErrUnexpected)
	} else if err = f.Close(); err != nil {
		return wrapErr(err, ErrUnexpected)
	}
	if hackRootDir == "" {
		hackRootDir = path.Join(os.TempDir(), "knative", "hack", "scripts")
	}
	l.debugf("Extracting hack scripts to directory: %s", hackRootDir)
	if err := copyDir(l, hack.Scripts, hackRootDir, "."); err != nil {
		return err
	}
	scriptPath := path.Join(hackRootDir, o.ScriptName)
	l.Println(scriptPath)
	return nil
}

func copyDir(l logger, inputFS fs.ReadDirFS, destRootDir, dir string) error {
	return wrapErr(fs.WalkDir(inputFS, dir, func(filePath string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return wrapErr(err, ErrBug)
		}
		return copyFile(l, inputFS, destRootDir, filePath, dirEntry)
	}), ErrUnexpected)
}

func copyFile(
	l logger,
	inputFS fs.ReadDirFS,
	destRootDir, filePath string,
	dirEntry fs.DirEntry,
) error {
	inputFI, err := dirEntry.Info()
	if err != nil {
		return wrapErr(err, ErrBug)
	}

	destPath := path.Join(destRootDir, filePath)
	perm := inputFI.Mode().Perm()

	perm |= PermOwnerWrite
	if dirEntry.IsDir() {
		perm |= PermAllExecutable
		if err = os.MkdirAll(destPath, perm); err != nil {
			return wrapErr(err, ErrUnexpected)
		}
		return nil
	}

	var (
		inputCS checksum
		destCS  checksum
		destFI  fs.FileInfo
		bytes   []byte
	)
	inputCS = asChecksum(inputFI)
	if destFI, err = os.Stat(destPath); err != nil {
		if !os.IsNotExist(err) {
			return wrapErr(err, ErrUnexpected)
		}
	} else {
		destCS = asChecksum(destFI)
	}
	if inputCS == destCS {
		l.debugf("%-30s up-to-date", filePath)
		return nil
	}
	if bytes, err = fs.ReadFile(inputFS, filePath); err != nil {
		return wrapErr(err, ErrBug)
	}
	if err = os.WriteFile(destPath, bytes, perm); err != nil {
		return wrapErr(err, ErrUnexpected)
	}

	sizeKB := int(inputFI.Size() / 1024)
	size5k := int(math.Ceil(float64(sizeKB) / 5))
	l.debugf("%-30s %3d KiB %s", filePath, sizeKB, strings.Repeat("+", size5k))
	return nil
}

func asChecksum(f fs.FileInfo) checksum {
	return checksum{
		exists: true,
		size:   f.Size(),
	}
}

type checksum struct {
	exists bool
	size   int64
}
