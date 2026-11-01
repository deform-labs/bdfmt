package worldgen

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InstallDataPacks copies local datapack directories or zip files into a world
// and adds their file-pack IDs to the level metadata options.
func InstallDataPacks(worldDir string, paths []string, opts *LevelOptions) error {
	if len(paths) == 0 {
		return nil
	}
	type sourcePack struct {
		path  string
		name  string
		isDir bool
	}
	packs := make([]sourcePack, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, rawPath := range paths {
		path := strings.Trim(strings.TrimSpace(rawPath), "\"'")
		if path == "" {
			return fmt.Errorf("empty datapack path")
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("datapack %q: %w", path, err)
		}
		name := filepath.Base(filepath.Clean(path))
		if name == "." || name == string(filepath.Separator) || name == "" {
			return fmt.Errorf("invalid datapack path %q", path)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("multiple datapacks have the same name %q", name)
		}
		seen[name] = struct{}{}
		if info.IsDir() {
			if _, err := os.Stat(filepath.Join(path, "pack.mcmeta")); err != nil {
				return fmt.Errorf("datapack directory %q has no pack.mcmeta", path)
			}
			packs = append(packs, sourcePack{path: path, name: name, isDir: true})
			continue
		}
		if !strings.EqualFold(filepath.Ext(name), ".zip") {
			return fmt.Errorf("datapack %q must be a directory or .zip file", path)
		}
		archive, err := zip.OpenReader(path)
		if err != nil {
			return fmt.Errorf("open datapack zip %q: %w", path, err)
		}
		foundMeta := false
		for _, file := range archive.File {
			if file.Name == "pack.mcmeta" {
				foundMeta = true
				break
			}
		}
		_ = archive.Close()
		if !foundMeta {
			return fmt.Errorf("datapack zip %q has no root pack.mcmeta", path)
		}
		packs = append(packs, sourcePack{path: path, name: name})
	}

	destination := filepath.Join(worldDir, "datapacks")
	if err := os.MkdirAll(destination, 0755); err != nil {
		return fmt.Errorf("create datapacks directory: %w", err)
	}
	for _, pack := range packs {
		target := filepath.Join(destination, pack.name)
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("world already has a datapack named %q", pack.name)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check destination for datapack %q: %w", pack.name, err)
		}
		if pack.isDir {
			if err := copyDataPackDirectory(pack.path, target); err != nil {
				return fmt.Errorf("copy datapack %q: %w", pack.name, err)
			}
		} else if err := copyDataPackFile(pack.path, target); err != nil {
			return fmt.Errorf("copy datapack %q: %w", pack.name, err)
		}
		id := "file/" + pack.name
		if !containsString(opts.DataPacksEnabled, id) {
			opts.DataPacksEnabled = append(opts.DataPacksEnabled, id)
		}
		filtered := opts.DataPacksDisabled[:0]
		for _, disabled := range opts.DataPacksDisabled {
			if disabled != id {
				filtered = append(filtered, disabled)
			}
		}
		opts.DataPacksDisabled = filtered
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func copyDataPackFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copyDataPackDirectory(source, target string) error {
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(target, 0755)
		}
		destination := filepath.Join(target, relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("datapack contains unsupported symlink %q", relative)
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		return copyDataPackFile(path, destination)
	})
}
