package paths

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
)

const appDirName = "structa"

type Paths struct {
	Root       string
	ConfigFile string
	DBFile     string
	ThumbsDir  string
}

func Resolve() (Paths, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	root := filepath.Join(base, appDirName)
	p := Paths{
		Root:       root,
		ConfigFile: filepath.Join(root, "config.json"),
		DBFile:     filepath.Join(root, "catalog.db"),
		ThumbsDir:  filepath.Join(root, "thumbs"),
	}
	for _, d := range []string{p.Root, p.ThumbsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Paths{}, err
		}
	}
	return p, nil
}

func FolderKey(folderPath string) string {
	sum := sha1.Sum([]byte(filepath.Clean(folderPath)))
	return hex.EncodeToString(sum[:])
}

func (p Paths) ThumbDir(folderPath string) string {
	return filepath.Join(p.ThumbsDir, FolderKey(folderPath))
}
