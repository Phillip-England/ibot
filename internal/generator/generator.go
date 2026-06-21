package generator

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/phillip-england/ibot/internal/model"
)

var pythonKeywords = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true,
	"assert": true, "async": true, "await": true, "break": true, "class": true,
	"continue": true, "def": true, "del": true, "elif": true, "else": true,
	"except": true, "finally": true, "for": true, "from": true, "global": true,
	"if": true, "import": true, "in": true, "is": true, "lambda": true,
	"nonlocal": true, "not": true, "or": true, "pass": true, "raise": true,
	"return": true, "try": true, "while": true, "with": true, "yield": true,
}

func ValidateFunctionName(name string) error {
	if name == "" || pythonKeywords[name] {
		return fmt.Errorf("%q is not a valid Python function name", name)
	}
	for index, r := range name {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return fmt.Errorf("%q is not a valid Python function name", name)
			}
		} else if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return fmt.Errorf("%q is not a valid Python function name", name)
		}
	}
	return nil
}

func Point(options model.PointOptions) (string, error) {
	if err := ValidateFunctionName(options.Name); err != nil {
		return "", err
	}
	imports := ""
	if options.IncludeImports {
		imports = "import pyautogui\n"
		if options.Variation.All || options.Variation.Pixels > 0 {
			imports += "import random\n"
		}
		imports += "\n\n"
	}
	doc := fmt.Sprintf("    \"\"\"Click the screen at (%d, %d).\"\"\"\n", options.Position.X, options.Position.Y)
	action := fmt.Sprintf("    pyautogui.click(x=%d, y=%d)\n", options.Position.X, options.Position.Y)
	if options.Variation.All {
		return "", fmt.Errorf("point variation does not support all")
	}
	if options.Variation.Pixels > 0 {
		v := options.Variation.Pixels
		doc = fmt.Sprintf("    \"\"\"Click within %d pixels of (%d, %d) on each axis.\"\"\"\n", v, options.Position.X, options.Position.Y)
		action = fmt.Sprintf("    click_x = random.randint(%d, %d)\n    click_y = random.randint(%d, %d)\n    pyautogui.click(x=click_x, y=click_y)\n", options.Position.X-v, options.Position.X+v, options.Position.Y-v, options.Position.Y+v)
	}
	return imports + "def " + options.Name + "() -> None:\n" + doc + wrapHeld(action, options.Hold), nil
}

func Box(options model.BoxOptions) (string, error) {
	if err := ValidateFunctionName(options.Name); err != nil {
		return "", err
	}
	left, right, top, bottom := bounds(options.Corners[:])
	centerX, centerY := (left+right)/2, (top+bottom)/2
	imports := ""
	if options.IncludeImports {
		imports = "import pyautogui\n"
		if options.Variation.All || options.Variation.Pixels > 0 {
			imports += "import random\n"
		}
		imports += "\n\n"
	}
	body := ""
	if options.Variation.All {
		body = fmt.Sprintf("    \"\"\"Click a random point in the box (%d, %d) to (%d, %d).\"\"\"\n    click_x = random.randint(%d, %d)\n    click_y = random.randint(%d, %d)\n    pyautogui.click(x=click_x, y=click_y)\n", left, top, right, bottom, left, right, top, bottom)
	} else if options.Variation.Pixels > 0 {
		v := options.Variation.Pixels
		body = fmt.Sprintf("    \"\"\"Click within %d pixels of the box center (%d, %d) on each axis.\"\"\"\n    click_x = random.randint(%d, %d)\n    click_y = random.randint(%d, %d)\n    pyautogui.click(x=click_x, y=click_y)\n", v, centerX, centerY, max(left, centerX-v), min(right, centerX+v), max(top, centerY-v), min(bottom, centerY+v))
	} else {
		body = fmt.Sprintf("    \"\"\"Click the center of the captured box at (%d, %d).\"\"\"\n    pyautogui.click(x=%d, y=%d)\n", centerX, centerY, centerX, centerY)
	}
	return imports + "def " + options.Name + "() -> None:\n" + body, nil
}

func Image(options model.ImageOptions) (string, error) {
	if options.Order == "" {
		options.Order = "linear"
	}
	if options.Confidence == 0 {
		options.Confidence = 0.9
	}
	if options.Timeout == 0 {
		options.Timeout = 30
	}
	if err := validateImage(options); err != nil {
		return "", err
	}
	needsRandom := options.Variation.All || options.Variation.Pixels > 0 || options.Order == "random" || isRange(options.Stall) || isRange(options.Gap)
	needsTime := options.WaitFor || options.Stall != nil || options.Gap != nil
	imports := ""
	if options.IncludeImports {
		imports = "import base64\nimport io\n"
		if needsRandom {
			imports += "import random\n"
		}
		if needsTime {
			imports += "import time\n"
		}
		imports += "\nimport pyautogui\nfrom PIL import Image\n\n\n"
	}

	encoded := base64.StdEncoding.EncodeToString(options.PNG)
	var body strings.Builder
	fmt.Fprintf(&body, "def %s() -> None:\n", options.Name)
	body.WriteString("    \"\"\"Locate the embedded image on screen and click it.\"\"\"\n    image_data = (\n")
	for len(encoded) > 0 {
		length := min(76, len(encoded))
		fmt.Fprintf(&body, "        %s\n", strconv.Quote(encoded[:length]))
		encoded = encoded[length:]
	}
	body.WriteString("    )\n    image = Image.open(io.BytesIO(base64.b64decode(image_data)))\n    screen_size = tuple(int(value) for value in pyautogui.size())\n\n    def find_images():\n        screen = pyautogui.screenshot().convert(\"RGB\")\n        if screen.size != screen_size:\n            screen = screen.resize(screen_size, Image.Resampling.LANCZOS)\n        try:\n")
	if options.ClickAll {
		fmt.Fprintf(&body, "            return list(pyautogui.locateAll(image, screen, confidence=%s))\n", number(options.Confidence))
	} else {
		fmt.Fprintf(&body, "            match = pyautogui.locate(image, screen, confidence=%s)\n            return [] if match is None else [match]\n", number(options.Confidence))
	}
	body.WriteString("        except pyautogui.ImageNotFoundException:\n            return []\n\n")
	notFound := fmt.Sprintf("embedded image was not found on screen at confidence %s", number(options.Confidence))
	if options.WaitFor {
		fmt.Fprintf(&body, "    deadline = time.monotonic() + %s\n    matches = find_images()\n    while not matches:\n        if time.monotonic() >= deadline:\n            raise RuntimeError(%s)\n        time.sleep(0.25)\n        matches = find_images()\n", number(options.Timeout), strconv.Quote(notFound+" before timeout"))
	} else {
		fmt.Fprintf(&body, "    matches = find_images()\n    if not matches:\n        raise RuntimeError(%s)\n", strconv.Quote(notFound))
	}
	if options.ClickAll {
		switch options.Order {
		case "random":
			body.WriteString("    random.shuffle(matches)\n")
		case "backwards":
			body.WriteString("    matches.sort(key=lambda match: (int(match.top), int(match.left)))\n    matches.reverse()\n")
		default:
			body.WriteString("    matches.sort(key=lambda match: (int(match.top), int(match.left)))\n")
		}
	}
	writeSleep(&body, "    ", options.Stall)

	var clicks strings.Builder
	clicks.WriteString("    for index, match in enumerate(matches):\n        left = int(match.left)\n        top = int(match.top)\n        right = left + int(match.width) - 1\n        bottom = top + int(match.height) - 1\n        center_x = (left + right) // 2\n        center_y = (top + bottom) // 2\n")
	if options.Variation.All {
		clicks.WriteString("        click_x = random.randint(left, right)\n        click_y = random.randint(top, bottom)\n        pyautogui.click(x=click_x, y=click_y)\n")
	} else if options.Variation.Pixels > 0 {
		v := options.Variation.Pixels
		fmt.Fprintf(&clicks, "        click_x = random.randint(max(left, center_x - %d), min(right, center_x + %d))\n        click_y = random.randint(max(top, center_y - %d), min(bottom, center_y + %d))\n        pyautogui.click(x=click_x, y=click_y)\n", v, v, v, v)
	} else {
		clicks.WriteString("        pyautogui.click(x=center_x, y=center_y)\n")
	}
	if options.Gap != nil {
		clicks.WriteString("        if index < len(matches) - 1:\n")
		writeSleep(&clicks, "            ", options.Gap)
	}
	body.WriteString(wrapHeld(clicks.String(), options.Hold))
	return imports + body.String(), nil
}

func validateImage(options model.ImageOptions) error {
	if err := ValidateFunctionName(options.Name); err != nil {
		return err
	}
	if len(options.PNG) == 0 {
		return fmt.Errorf("image data cannot be empty")
	}
	if !finite(options.Confidence) || options.Confidence <= 0 || options.Confidence > 1 {
		return fmt.Errorf("confidence must be greater than zero and at most one")
	}
	if !finite(options.Timeout) || options.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if options.Order != "linear" && options.Order != "backwards" && options.Order != "random" {
		return fmt.Errorf("order must be linear, backwards, or random")
	}
	if !options.ClickAll && (options.Order != "linear" || options.Gap != nil) {
		return fmt.Errorf("order and gap require all")
	}
	for label, value := range map[string]*model.SecondsRange{"stall": options.Stall, "gap": options.Gap} {
		if value != nil && (!finite(value.Min) || !finite(value.Max) || value.Min < 0 || value.Max < value.Min) {
			return fmt.Errorf("%s must be a finite non-negative range", label)
		}
	}
	return nil
}

func wrapHeld(action string, keys []string) string {
	keys = unique(keys)
	if len(keys) == 0 {
		return action
	}
	quoted := make([]string, len(keys))
	for i, key := range keys {
		quoted[i] = strconv.Quote(key)
	}
	tuple := "(" + strings.Join(quoted, ", ")
	if len(keys) == 1 {
		tuple += ","
	}
	tuple += ")"
	return fmt.Sprintf("    invalid_keys = [key for key in %s if key not in pyautogui.KEYBOARD_KEYS]\n    if invalid_keys:\n        raise ValueError(f\"unsupported hold key(s): {', '.join(invalid_keys)}\")\n    held_keys = []\n    try:\n        for key in %s:\n            pyautogui.keyDown(key)\n            held_keys.append(key)\n%s    finally:\n        release_error = None\n        for key in reversed(held_keys):\n            try:\n                pyautogui.keyUp(key)\n            except Exception as error:\n                if release_error is None:\n                    release_error = error\n        if release_error is not None:\n            raise release_error\n", tuple, tuple, indent(action, "    "))
}

func writeSleep(builder *strings.Builder, indentation string, value *model.SecondsRange) {
	if value == nil {
		return
	}
	if value.Min == value.Max {
		fmt.Fprintf(builder, "%stime.sleep(%s)\n", indentation, number(value.Min))
	} else {
		fmt.Fprintf(builder, "%stime.sleep(random.uniform(%s, %s))\n", indentation, number(value.Min), number(value.Max))
	}
}

func bounds(points []model.Position) (left, right, top, bottom int) {
	left, right, top, bottom = points[0].X, points[0].X, points[0].Y, points[0].Y
	for _, point := range points[1:] {
		left, right = min(left, point.X), max(right, point.X)
		top, bottom = min(top, point.Y), max(bottom, point.Y)
	}
	return
}

func indent(value, prefix string) string {
	lines := strings.SplitAfter(value, "\n")
	var result strings.Builder
	for _, line := range lines {
		if line != "" {
			result.WriteString(prefix)
			result.WriteString(line)
		}
	}
	return result.String()
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func isRange(value *model.SecondsRange) bool { return value != nil && value.Min != value.Max }
func finite(value float64) bool              { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func number(value float64) string            { return strconv.FormatFloat(value, 'f', -1, 64) }
