package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Tab struct {
	Name       string     `json:"tab_name"`
	Categories []Category `json:"categories"`
}

type Category struct {
	Name    string   `json:"name"`
	Folders []string `json:"folders"`
}

type Config struct {
	Tabs []Tab `json:"tabs"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{Tabs: []Tab{}}, nil
		}
		return Config{}, err
	}
	if len(data) == 0 {
		return Config{Tabs: []Tab{}}, nil
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if c.Tabs == nil {
		c.Tabs = []Tab{}
	}
	for i := range c.Tabs {
		if c.Tabs[i].Categories == nil {
			c.Tabs[i].Categories = []Category{}
		}
		for j := range c.Tabs[i].Categories {
			if c.Tabs[i].Categories[j].Folders == nil {
				c.Tabs[i].Categories[j].Folders = []string{}
			}
		}
	}
	return c, nil
}

func Save(path string, c Config) error {
	if c.Tabs == nil {
		c.Tabs = []Tab{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
