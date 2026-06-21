package target

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/phillip-england/ibot/internal/model"
)

type PointFile struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type BoxFile struct {
	Corners [4]model.Position `json:"corners"`
}

type Entry struct {
	Name    string             `json:"name"`
	Kind    string             `json:"kind"`
	Point   *model.Position    `json:"point,omitempty"`
	Corners *[4]model.Position `json:"corners,omitempty"`
}

func WritePoint(path string, point model.Position) error {
	return write(path, PointFile{X: point.X, Y: point.Y})
}

func WriteBox(path string, corners [4]model.Position) error {
	return write(path, BoxFile{Corners: corners})
}

func WriteImage(path string, data []byte) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("image path is required")
	}
	if filepath.Ext(path) != ".png" {
		return fmt.Errorf("image name must end in .png")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create image directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".ibot-*.png")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Chmod(0o644)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func write(path string, value any) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("JSON path is required")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".ibot-*.json")
	if err != nil {
		return fmt.Errorf("create temporary target: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Chmod(0o644)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write target: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace target: %w", err)
	}
	return nil
}

func List(pointsDir, boxesDir string) ([]Entry, error) {
	entries := []Entry{}
	if pointsDir != "" {
		files, err := jsonFiles(pointsDir)
		if err != nil {
			return nil, fmt.Errorf("points: %w", err)
		}
		for _, file := range files {
			var value PointFile
			if err := read(filepath.Join(pointsDir, file), &value); err != nil {
				return nil, fmt.Errorf("point %s: %w", file, err)
			}
			point := model.Position{X: value.X, Y: value.Y}
			entries = append(entries, Entry{Name: file, Kind: "point", Point: &point})
		}
	}
	if boxesDir != "" {
		files, err := jsonFiles(boxesDir)
		if err != nil {
			return nil, fmt.Errorf("boxes: %w", err)
		}
		for _, file := range files {
			var value BoxFile
			if err := read(filepath.Join(boxesDir, file), &value); err != nil {
				return nil, fmt.Errorf("box %s: %w", file, err)
			}
			corners := value.Corners
			entries = append(entries, Entry{Name: file, Kind: "box", Corners: &corners})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind == entries[j].Kind {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Kind < entries[j].Kind
	})
	return entries, nil
}

func Path(dir, name string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("this target directory was not configured")
	}
	if name != filepath.Base(name) || filepath.Ext(name) != ".json" || name == ".json" {
		return "", fmt.Errorf("target name must be a .json filename")
	}
	return filepath.Join(dir, name), nil
}

func jsonFiles(dir string) ([]string, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, item := range items {
		if !item.IsDir() && filepath.Ext(item.Name()) == ".json" {
			files = append(files, item.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func read(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
