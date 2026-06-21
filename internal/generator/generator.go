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
	if options.Variation.All {
		return "", fmt.Errorf("point variation does not support all")
	}
	if !validSecondsRange(options.Delay) {
		return "", fmt.Errorf("delay must be a finite non-negative range")
	}
	imports := ""
	if options.IncludeImports {
		imports = "import random\nimport time\n\nimport pyautogui\n\n\n"
	}
	return fmt.Sprintf(`%sdef %s(variation=%d, delay=%s, hold=%s) -> None:
    """Click the captured point; runtime arguments control click behavior."""
    if not isinstance(variation, int) or isinstance(variation, bool) or variation < 0:
        raise ValueError("variation must be a non-negative integer")
    click_x = random.randint(%d - variation, %d + variation)
    click_y = random.randint(%d - variation, %d + variation)
%s`, imports, options.Name, options.Variation.Pixels, pythonSecondsRange(options.Delay), pythonTuple(options.Hold),
		options.Position.X, options.Position.X, options.Position.Y, options.Position.Y,
		runtimeClick("click_x", "click_y", "delay", "hold", "    ")), nil
}

func Box(options model.BoxOptions) (string, error) {
	if err := ValidateFunctionName(options.Name); err != nil {
		return "", err
	}
	if !validSecondsRange(options.Delay) {
		return "", fmt.Errorf("delay must be a finite non-negative range")
	}
	left, right, top, bottom := bounds(options.Corners[:])
	baseLeft, baseRight, baseTop, baseBottom := left, right, top, bottom
	if options.Grid != nil {
		if _, err := model.NewGridTarget(options.Grid.Rows, options.Grid.Columns, options.Grid.Cell); err != nil {
			return "", err
		}
		width, height := right-left+1, bottom-top+1
		if options.Grid.Columns > width || options.Grid.Rows > height {
			return "", fmt.Errorf("grid cannot have more columns or rows than the box has pixels")
		}
		row := options.Grid.Cell / options.Grid.Columns
		column := options.Grid.Cell % options.Grid.Columns
		left, right = left+(width*column)/options.Grid.Columns, left+(width*(column+1))/options.Grid.Columns-1
		top, bottom = top+(height*row)/options.Grid.Rows, top+(height*(row+1))/options.Grid.Rows-1
	}
	imports := ""
	if options.IncludeImports {
		imports = "import random\nimport time\n\nimport pyautogui\n\n\n"
	}
	gridDefault := "None"
	if options.Grid != nil {
		gridDefault = fmt.Sprintf("(%d, %d, %d)", options.Grid.Rows, options.Grid.Columns, options.Grid.Cell)
	}
	variationDefault := strconv.Itoa(options.Variation.Pixels)
	if options.Variation.All {
		variationDefault = `"all"`
	}
	return fmt.Sprintf(`%sdef %s(variation=%s, grid=%s, delay=%s, hold=()) -> None:
    """Click the captured box; runtime arguments control targeting and behavior."""
    left, right, top, bottom = %d, %d, %d, %d
    if grid is not None:
        if not isinstance(grid, (tuple, list)) or len(grid) != 3:
            raise ValueError("grid must be (rows, columns, cell) or None")
        rows, columns, cell = grid
        if any(not isinstance(value, int) or isinstance(value, bool) for value in grid):
            raise ValueError("grid values must be integers")
        if rows <= 0 or columns <= 0 or cell < 0 or cell >= rows * columns:
            raise ValueError("grid rows, columns, and cell are out of range")
        width, height = right - left + 1, bottom - top + 1
        if columns > width or rows > height:
            raise ValueError("grid cannot have more columns or rows than the box has pixels")
        row, column = divmod(cell, columns)
        left, right = left + (width * column) // columns, left + (width * (column + 1)) // columns - 1
        top, bottom = top + (height * row) // rows, top + (height * (row + 1)) // rows - 1
    center_x, center_y = (left + right) // 2, (top + bottom) // 2
    if isinstance(variation, str) and variation.lower() in ("all", "*"):
        click_x, click_y = random.randint(left, right), random.randint(top, bottom)
    else:
        if not isinstance(variation, int) or isinstance(variation, bool) or variation < 0:
            raise ValueError('variation must be a non-negative integer or "all"')
        click_x = random.randint(max(left, center_x - variation), min(right, center_x + variation))
        click_y = random.randint(max(top, center_y - variation), min(bottom, center_y + variation))
%s`, imports, options.Name, variationDefault, gridDefault, pythonSecondsRange(options.Delay), baseLeft, baseRight, baseTop, baseBottom,
		runtimeClick("click_x", "click_y", "delay", "hold", "    ")), nil
}

func Image(options model.ImageOptions) (string, error) {
	if options.Delay == nil {
		options.Delay = options.Stall
	}
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
	imports := ""
	if options.IncludeImports {
		imports = "import base64\nimport io\n"
		if !options.ReturnExists {
			imports += "import random\n"
			imports += "import time\n"
		}
		imports += "\nimport pyautogui\nfrom PIL import Image\n\n\n"
	}

	encoded := base64.StdEncoding.EncodeToString(options.PNG)
	var imageData strings.Builder
	imageData.WriteString("    image_data = (\n")
	for len(encoded) > 0 {
		length := min(76, len(encoded))
		fmt.Fprintf(&imageData, "        %s\n", strconv.Quote(encoded[:length]))
		encoded = encoded[length:]
	}
	imageData.WriteString("    )\n")
	if options.ReturnExists {
		return fmt.Sprintf(`%sdef %s(confidence=%s) -> bool:
    """Return whether the captured image is visible at runtime confidence."""
    if not isinstance(confidence, (int, float)) or isinstance(confidence, bool) or not 0 < confidence <= 1:
        raise ValueError("confidence must be greater than zero and at most one")
%s    image = Image.open(io.BytesIO(base64.b64decode(image_data))).convert("RGB")
    screenshot = pyautogui.screenshot().convert("RGB")
    screen_size = tuple(int(value) for value in pyautogui.size())
    screens = [screenshot]
    if screenshot.size != screen_size:
        screens = [
            screenshot.resize(screen_size, Image.Resampling.LANCZOS),
            screenshot.resize(screen_size, Image.Resampling.BICUBIC),
            screenshot.resize(screen_size, Image.Resampling.BILINEAR),
        ]
    for screen in screens:
        try:
            if pyautogui.locate(image, screen, confidence=confidence) is not None:
                return True
        except pyautogui.ImageNotFoundException:
            pass
    return False
`, imports, options.Name, number(options.Confidence), imageData.String()), nil
	}

	variationDefault := strconv.Itoa(options.Variation.Pixels)
	if options.Variation.All {
		variationDefault = `"all"`
	}
	waitForDefault, waitGoneDefault := pythonBool(options.WaitFor), pythonBool(options.WaitUntilGone)
	clickDefault := pythonBool(!options.NoClick && !options.WaitUntilGone)
	return fmt.Sprintf(`%sdef %s(*, confidence=%s, wait_for=%s, wait_until_gone=%s, timeout=%s,
        poll_interval=0.25, click=%s, all_matches=%s, order=%s,
        position="center", variation=%s, delay=%s, gap=%s, hold=%s) -> None:
    """Find the captured image; runtime arguments control waiting and clicking."""
    if not isinstance(confidence, (int, float)) or isinstance(confidence, bool) or not 0 < confidence <= 1:
        raise ValueError("confidence must be greater than zero and at most one")
    if wait_for and wait_until_gone:
        raise ValueError("wait_for and wait_until_gone cannot both be true")
    if not isinstance(timeout, (int, float)) or isinstance(timeout, bool) or timeout <= 0:
        raise ValueError("timeout must be greater than zero")
    if not isinstance(poll_interval, (int, float)) or isinstance(poll_interval, bool) or poll_interval <= 0:
        raise ValueError("poll_interval must be greater than zero")
    if order not in ("linear", "backwards", "random"):
        raise ValueError("order must be linear, backwards, or random")
    if not all_matches and (order != "linear" or gap is not None):
        raise ValueError("order and gap require all_matches=True")
    if wait_until_gone and click:
        raise ValueError("wait_until_gone cannot click")

    def duration_range(value, name):
        if value is None:
            return None
        if isinstance(value, (int, float)) and not isinstance(value, bool):
            result = (value, value)
        elif isinstance(value, (tuple, list)) and len(value) == 2:
            result = tuple(value)
        else:
            raise ValueError(f"{name} must be a non-negative number or a two-number range")
        if any(not isinstance(item, (int, float)) or isinstance(item, bool) for item in result):
            raise ValueError(f"{name} must be a non-negative number or a two-number range")
        if result[0] < 0 or result[1] < result[0]:
            raise ValueError(f"expected 0 <= {name} minimum <= {name} maximum")
        return result

    delay_range = duration_range(delay, "delay")
    gap_range = duration_range(gap, "gap")
%s    image = Image.open(io.BytesIO(base64.b64decode(image_data))).convert("RGB")
    screen_size = tuple(int(value) for value in pyautogui.size())

    def find_images():
        screenshot = pyautogui.screenshot().convert("RGB")
        screens = [screenshot]
        if screenshot.size != screen_size:
            screens = [
                screenshot.resize(screen_size, Image.Resampling.LANCZOS),
                screenshot.resize(screen_size, Image.Resampling.BICUBIC),
                screenshot.resize(screen_size, Image.Resampling.BILINEAR),
            ]
        for screen in screens:
            try:
                matches = list(pyautogui.locateAll(image, screen, confidence=confidence))
            except pyautogui.ImageNotFoundException:
                matches = []
            if matches:
                return matches
        return []

    deadline = time.monotonic() + timeout
    matches = find_images()
    if wait_until_gone:
        while matches:
            if time.monotonic() >= deadline:
                raise RuntimeError(f"embedded image was still visible at confidence {confidence} before timeout")
            time.sleep(poll_interval)
            matches = find_images()
        return
    if wait_for:
        while not matches:
            if time.monotonic() >= deadline:
                raise RuntimeError(f"embedded image was not found at confidence {confidence} before timeout")
            time.sleep(poll_interval)
            matches = find_images()
    elif not matches:
        raise RuntimeError(f"embedded image was not found on screen at confidence {confidence}")
    if not click:
        return

    matches.sort(key=lambda match: (int(match.top), int(match.left)))
    if not all_matches:
        matches = matches[:1]
    elif order == "backwards":
        matches.reverse()
    elif order == "random":
        random.shuffle(matches)
    anchors = {
        "top_left": (0.0, 0.0), "top": (0.5, 0.0), "top_right": (1.0, 0.0),
        "left": (0.0, 0.5), "center": (0.5, 0.5), "right": (1.0, 0.5),
        "bottom_left": (0.0, 1.0), "bottom": (0.5, 1.0), "bottom_right": (1.0, 1.0),
    }
    if isinstance(position, str):
        if position not in anchors:
            raise ValueError("position must be a named anchor or an (x, y) ratio")
        position_x, position_y = anchors[position]
    elif isinstance(position, (tuple, list)) and len(position) == 2:
        position_x, position_y = position
        if any(not isinstance(value, (int, float)) or isinstance(value, bool) or not 0 <= value <= 1 for value in position):
            raise ValueError("position ratios must be numbers from 0 through 1")
    else:
        raise ValueError("position must be a named anchor or an (x, y) ratio")

    if isinstance(variation, str) and variation.lower() in ("all", "*"):
        variation_all = True
    elif isinstance(variation, int) and not isinstance(variation, bool) and variation >= 0:
        variation_all = False
    else:
        raise ValueError('variation must be a non-negative integer or "all"')
    if isinstance(hold, str):
        hold = (hold,)
    else:
        hold = tuple(hold)
    invalid_keys = [key for key in hold if key not in pyautogui.KEYBOARD_KEYS]
    if invalid_keys:
        raise ValueError(f"unsupported hold key(s): {', '.join(invalid_keys)}")
    held_keys = []
    try:
        for key in hold:
            pyautogui.keyDown(key)
            held_keys.append(key)
        for index, match in enumerate(matches):
            left, top = int(match.left), int(match.top)
            right = left + int(match.width) - 1
            bottom = top + int(match.height) - 1
            if variation_all:
                click_x, click_y = random.randint(left, right), random.randint(top, bottom)
            else:
                anchor_x = round(left + (right - left) * position_x)
                anchor_y = round(top + (bottom - top) * position_y)
                click_x = random.randint(max(left, anchor_x - variation), min(right, anchor_x + variation))
                click_y = random.randint(max(top, anchor_y - variation), min(bottom, anchor_y + variation))
            if delay_range is None:
                pyautogui.click(x=click_x, y=click_y)
            else:
                pyautogui.moveTo(x=click_x, y=click_y)
                time.sleep(random.uniform(*delay_range))
                pyautogui.click()
            if gap_range is not None and index < len(matches) - 1:
                time.sleep(random.uniform(*gap_range))
    finally:
        release_error = None
        for key in reversed(held_keys):
            try:
                pyautogui.keyUp(key)
            except Exception as error:
                if release_error is None:
                    release_error = error
        if release_error is not None:
            raise release_error
`, imports, options.Name, number(options.Confidence), waitForDefault, waitGoneDefault,
		number(options.Timeout), clickDefault, pythonBool(options.ClickAll), strconv.Quote(options.Order),
		variationDefault, pythonSecondsRange(options.Delay), pythonSecondsRange(options.Gap),
		pythonTuple(options.Hold), imageData.String()), nil
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
	if options.WaitFor && options.WaitUntilGone {
		return fmt.Errorf("wait for and wait until gone cannot be used together")
	}
	if options.NoClick && !options.WaitFor {
		return fmt.Errorf("no click requires wait for")
	}
	if options.NoClick && (options.Variation.All || options.Variation.Pixels > 0 || options.Delay != nil || options.ClickAll || options.Order != "linear" || options.Gap != nil || len(options.Hold) > 0) {
		return fmt.Errorf("no click cannot be combined with click options")
	}
	if options.ReturnExists && (options.WaitFor || options.NoClick || options.WaitUntilGone || options.Variation.All || options.Variation.Pixels > 0 || options.Delay != nil || options.ClickAll || options.Order != "linear" || options.Gap != nil || len(options.Hold) > 0) {
		return fmt.Errorf("image existence checks cannot be combined with click or wait options")
	}
	if options.WaitUntilGone && (options.Variation.All || options.Variation.Pixels > 0 || options.Delay != nil || options.ClickAll || options.Order != "linear" || options.Gap != nil || len(options.Hold) > 0) {
		return fmt.Errorf("wait until gone cannot be combined with click options")
	}
	for label, value := range map[string]*model.SecondsRange{"delay": options.Delay, "gap": options.Gap} {
		if !validSecondsRange(value) {
			return fmt.Errorf("%s must be a finite non-negative range", label)
		}
	}
	return nil
}

func pythonTuple(values []string) string {
	values = unique(values)
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = strconv.Quote(value)
	}
	result := "(" + strings.Join(quoted, ", ")
	if len(quoted) == 1 {
		result += ","
	}
	return result + ")"
}

func pythonBool(value bool) string {
	if value {
		return "True"
	}
	return "False"
}

func pythonSecondsRange(value *model.SecondsRange) string {
	if value == nil {
		return "None"
	}
	if value.Min == value.Max {
		return number(value.Min)
	}
	return fmt.Sprintf("(%s, %s)", number(value.Min), number(value.Max))
}

// runtimeClick emits validation, an optional move-before-click delay, and
// guaranteed key release. Its inputs are Python expressions generated above.
func runtimeClick(x, y, delay, hold, indentation string) string {
	template := `DELAY_RANGE = None
if DELAY is not None:
    if isinstance(DELAY, (int, float)) and not isinstance(DELAY, bool):
        DELAY_RANGE = (DELAY, DELAY)
    elif isinstance(DELAY, (tuple, list)) and len(DELAY) == 2:
        DELAY_RANGE = tuple(DELAY)
    else:
        raise ValueError("delay must be a non-negative number or a two-number range")
    if any(not isinstance(value, (int, float)) or isinstance(value, bool) for value in DELAY_RANGE):
        raise ValueError("delay must be a non-negative number or a two-number range")
    if DELAY_RANGE[0] < 0 or DELAY_RANGE[1] < DELAY_RANGE[0]:
        raise ValueError("expected 0 <= delay minimum <= delay maximum")
if isinstance({{HOLD}}, str):
    {{HOLD}} = ({{HOLD}},)
else:
    {{HOLD}} = tuple({{HOLD}})
invalid_keys = [key for key in {{HOLD}} if key not in pyautogui.KEYBOARD_KEYS]
if invalid_keys:
    raise ValueError(f"unsupported hold key(s): {', '.join(invalid_keys)}")
held_keys = []
try:
    for key in {{HOLD}}:
        pyautogui.keyDown(key)
        held_keys.append(key)
    if DELAY_RANGE is None:
        pyautogui.click(x={{X}}, y={{Y}})
    else:
        pyautogui.moveTo(x={{X}}, y={{Y}})
        time.sleep(random.uniform(*DELAY_RANGE))
        pyautogui.click()
finally:
    release_error = None
    for key in reversed(held_keys):
        try:
            pyautogui.keyUp(key)
        except Exception as error:
            if release_error is None:
                release_error = error
    if release_error is not None:
        raise release_error
`
	replacer := strings.NewReplacer("DELAY_RANGE", "delay_range", "DELAY", delay, "{{HOLD}}", hold, "{{X}}", x, "{{Y}}", y)
	return indent(replacer.Replace(template), indentation)
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

func finite(value float64) bool   { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func number(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

func validSecondsRange(value *model.SecondsRange) bool {
	return value == nil || finite(value.Min) && finite(value.Max) && value.Min >= 0 && value.Max >= value.Min
}
