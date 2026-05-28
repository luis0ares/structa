package meta

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const FileName = "structa.yaml"

type ItemMeta struct {
	Name        string   `yaml:"name,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Link        string   `yaml:"link,omitempty"`
	Favorite    bool     `yaml:"favorite,omitempty"`
}

func Read(folderPath string) (ItemMeta, bool, error) {
	data, err := os.ReadFile(filepath.Join(folderPath, FileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ItemMeta{}, false, nil
		}
		return ItemMeta{}, false, err
	}
	var m ItemMeta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return ItemMeta{}, true, err
	}
	return m, true, nil
}

func Write(folderPath string, m ItemMeta) error {
	data, err := yaml.Marshal(&m)
	if err != nil {
		return err
	}
	tmp := filepath.Join(folderPath, FileName+".tmp")
	dst := filepath.Join(folderPath, FileName)
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
