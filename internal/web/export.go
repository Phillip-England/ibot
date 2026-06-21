package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var exportMu sync.Mutex

func validatePythonModule(directory string) error {
	info, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("export module %q: %w", directory, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("export module %q is not a directory", directory)
	}
	initPath := filepath.Join(directory, "__init__.py")
	info, err = os.Stat(initPath)
	if err != nil {
		return fmt.Errorf("export module requires %s: %w", initPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("export module %s is not a regular file", initPath)
	}
	return nil
}

func exportPythonFunction(directory, name, source string) error {
	exportMu.Lock()
	defer exportMu.Unlock()

	if err := validatePythonModule(directory); err != nil {
		return err
	}
	if !strings.HasSuffix(source, "\n") {
		source += "\n"
	}
	if err := writeAtomic(filepath.Join(directory, name+".py"), []byte(source), 0o644); err != nil {
		return err
	}

	initPath := filepath.Join(directory, "__init__.py")
	contents, err := os.ReadFile(initPath)
	if err != nil {
		return err
	}
	importLine := fmt.Sprintf("from .%s import *", name)
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.TrimSpace(line) == importLine {
			return nil
		}
	}
	updated := string(contents)
	if updated != "" && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	updated += importLine + "\n"
	info, err := os.Stat(initPath)
	if err != nil {
		return err
	}
	return writeAtomic(initPath, []byte(updated), info.Mode().Perm())
}

func writeAtomic(path string, contents []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ibot-export-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
