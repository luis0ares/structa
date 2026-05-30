package profiles

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const fileName = "profiles.json"

type Profile struct {
	Name    string `json:"name"`
	DataDir string `json:"data_dir"`
}

type Registry struct {
	Active   string    `json:"active"`
	Profiles []Profile `json:"profiles"`
}

func Load(metaDir string) (Registry, error) {
	data, err := os.ReadFile(filepath.Join(metaDir, fileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Registry{Profiles: []Profile{}}, nil
		}
		return Registry{}, err
	}
	if len(data) == 0 {
		return Registry{Profiles: []Profile{}}, nil
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return Registry{}, err
	}
	if r.Profiles == nil {
		r.Profiles = []Profile{}
	}
	return r, nil
}

func Save(metaDir string, r Registry) error {
	if r.Profiles == nil {
		r.Profiles = []Profile{}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(metaDir, fileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
