package paths

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
)

const (
	appDirName = "structa"
	// DotStructa is the hidden subfolder created inside each profile's data directory.
	DotStructa = ".structa"
)

type Paths struct {
	Root       string
	ConfigFile string
	DBFile     string
	ThumbsDir  string
}

// AppMetaDir returns the directory that holds profiles.json.
// Always %AppData%/structa — independent of which profile is active.
func AppMetaDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ResolveProfile returns paths for a profile's data directory.
// If dataDir already ends with ".structa" it is used directly as the root;
// otherwise a ".structa" subdirectory is created inside it.
func ResolveProfile(dataDir string) (Paths, error) {
	var root string
	if filepath.Base(filepath.Clean(dataDir)) == DotStructa {
		root = filepath.Clean(dataDir)
	} else {
		root = filepath.Join(dataDir, DotStructa)
	}
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
