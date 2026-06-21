package project

import (
	"fmt"
	"os"
	"path/filepath"
)

const UtilitiesSource = `"""Reusable helpers installed by ibot."""

import json
import os
import platform
import random
import time

import pyautogui


def print_and_do(message, action):
    print(message)
    return action()


def press_hotkey(windows_linux_keys, mac_keys):
    keys = mac_keys if platform.system() == "Darwin" else windows_linux_keys
    aliases = {"control": "ctrl"}
    pyautogui.hotkey(*(aliases.get(key, key) for key in keys))


def random_wait(min_seconds, max_seconds):
    if min_seconds < 0 or max_seconds < min_seconds:
        raise ValueError("expected 0 <= min_seconds <= max_seconds")
    duration = random.uniform(min_seconds, max_seconds)
    time.sleep(duration)
    return duration


# Kept for projects that used the original helper name.
wait_random = random_wait


def wait_until(predicate, timeout=30, interval=0.25):
    if timeout < 0 or interval <= 0:
        raise ValueError("timeout must be non-negative and interval must be positive")
    deadline = time.monotonic() + timeout
    while True:
        result = predicate()
        if result:
            return result
        if time.monotonic() >= deadline:
            return None
        time.sleep(min(interval, max(0, deadline - time.monotonic())))


def retry(action, attempts=3, delay=0):
    if attempts < 1:
        raise ValueError("attempts must be at least one")
    if delay < 0:
        raise ValueError("delay must be non-negative")
    for attempt in range(attempts):
        try:
            return action()
        except Exception:
            if attempt == attempts - 1:
                raise
            if delay:
                time.sleep(delay)


def random_number(minimum, maximum):
    return random.randint(minimum, maximum)


def roll(chance, out_of=100):
    if any(not isinstance(value, (int, float)) or isinstance(value, bool) for value in (chance, out_of)):
        raise TypeError("chance and out_of must be numbers")
    if out_of <= 0:
        raise ValueError("out_of must be greater than zero")
    if chance < 0 or chance > out_of:
        raise ValueError("expected 0 <= chance <= out_of")
    return random.random() * out_of < chance


def perform_random_actions(min_actions, max_actions, actions):
    actions = list(actions)
    if min_actions < 0 or max_actions < min_actions:
        raise ValueError("expected 0 <= min_actions <= max_actions")
    if max_actions > len(actions):
        raise ValueError("max_actions cannot exceed the number of actions")
    if not all(callable(action) for action in actions):
        raise TypeError("every action must be callable")

    amount = random_number(min_actions, max_actions)
    for action in random.sample(actions, amount):
        action()


def run_with_random_wait(min_seconds, max_seconds, functions):
    for index, function in enumerate(functions):
        function()
        if index < len(functions) - 1:
            random_wait(min_seconds, max_seconds)


def _read_target(*path):
    with open(os.path.join(*path), encoding="utf-8") as target_file:
        return json.load(target_file)


def _click_target(x, y, delay=None):
    if delay is None:
        delay_range = None
    elif isinstance(delay, (int, float)) and not isinstance(delay, bool):
        delay_range = (delay, delay)
    elif isinstance(delay, (tuple, list)) and len(delay) == 2:
        delay_range = delay
    else:
        raise ValueError("delay must be a non-negative number or a two-number range")
    if delay_range is not None:
        minimum, maximum = delay_range
        if any(not isinstance(value, (int, float)) or isinstance(value, bool) for value in delay_range):
            raise ValueError("delay must be a non-negative number or a two-number range")
        if minimum < 0 or maximum < minimum:
            raise ValueError("expected 0 <= delay minimum <= delay maximum")

    pyautogui.moveTo(x=x, y=y)
    if delay_range is not None:
        random_wait(minimum, maximum)
    pyautogui.click()


def click_saved_point(*path, variation=0, delay=None):
    point = _read_target(*path)
    x, y = int(point["x"]), int(point["y"])
    if not isinstance(variation, int) or isinstance(variation, bool) or variation < 0:
        raise ValueError("point variation must be a non-negative integer")
    if variation:
        x = random.randint(x - variation, x + variation)
        y = random.randint(y - variation, y + variation)
    _click_target(x, y, delay)


def click_saved_box(*path, variation=0, delay=None):
    corners = _read_target(*path)["corners"]
    left = min(int(point["x"]) for point in corners)
    right = max(int(point["x"]) for point in corners)
    top = min(int(point["y"]) for point in corners)
    bottom = max(int(point["y"]) for point in corners)
    center_x, center_y = (left + right) // 2, (top + bottom) // 2
    if isinstance(variation, str) and variation.lower() == "all":
        x, y = random.randint(left, right), random.randint(top, bottom)
    else:
        if not isinstance(variation, int) or isinstance(variation, bool) or variation < 0:
            raise ValueError('box variation must be a non-negative integer or "all"')
        if variation:
            x = random.randint(max(left, center_x - variation), min(right, center_x + variation))
            y = random.randint(max(top, center_y - variation), min(bottom, center_y + variation))
        else:
            x, y = center_x, center_y
    _click_target(x, y, delay)
`

type Layout struct {
	Root, Points, Boxes, Images string
}

func Open(root string) (Layout, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return Layout{}, fmt.Errorf("project %q must be an initialized directory", root)
	}
	for _, name := range []string{"__init__.py", "utilities.py"} {
		if info, statErr := os.Stat(filepath.Join(root, name)); statErr != nil || !info.Mode().IsRegular() {
			return Layout{}, fmt.Errorf("project %q is missing %s; run ibot init %s", root, name, root)
		}
	}
	layout := Layout{Root: root, Points: filepath.Join(root, "points"), Boxes: filepath.Join(root, "boxes"), Images: filepath.Join(root, "images")}
	for _, dir := range []string{layout.Points, layout.Boxes, layout.Images} {
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			return Layout{}, fmt.Errorf("project %q is missing directory %s", root, filepath.Base(dir))
		}
	}
	return layout, nil
}

func Init(root string) (Layout, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Layout{}, fmt.Errorf("create project: %w", err)
	}
	for _, name := range []string{"points", "boxes", "images"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			return Layout{}, fmt.Errorf("create %s directory: %w", name, err)
		}
	}
	files := map[string]string{
		"__init__.py":  "\"\"\"Automation package generated by ibot.\"\"\"\n\nfrom .utilities import *\n",
		"utilities.py": UtilitiesSource,
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if _, statErr := os.Stat(path); statErr == nil {
			continue
		} else if !os.IsNotExist(statErr) {
			return Layout{}, statErr
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			return Layout{}, fmt.Errorf("create %s: %w", name, err)
		}
	}
	return Open(root)
}

func PatchUtilities(root string) (string, error) {
	layout, err := Open(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(layout.Root, "utilities.py")
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("inspect utilities.py: %w", err)
	}

	temporary, err := os.CreateTemp(layout.Root, ".utilities-*.py")
	if err != nil {
		return "", fmt.Errorf("create temporary utilities.py: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporary.WriteString(UtilitiesSource); err != nil {
		temporary.Close()
		return "", fmt.Errorf("write utilities.py: %w", err)
	}
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		temporary.Close()
		return "", fmt.Errorf("set utilities.py permissions: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close utilities.py: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", fmt.Errorf("replace utilities.py: %w", err)
	}
	return path, nil
}
