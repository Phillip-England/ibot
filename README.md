# ibot

`ibot` is a Go desktop-capture CLI and local web application that generates
reusable Python automation functions. The installed Go binary captures screen
positions, regions, and images; manages every generation option; embeds the web
interface; and emits portable Python source that uses
[PyAutoGUI](https://pyautogui.readthedocs.io/).

The application itself does not require a Python environment. Python and the
generated function's imported libraries are needed only where that generated
source will run.

## Install

### Requirements

- Go 1.25 or newer
- A working C compiler because desktop input hooks use CGO
- macOS: Xcode Command Line Tools
- Linux: the development libraries required by robotgo/gohook for X11

Install from this checkout:

```sh
make install
```

This builds and installs one `ibot` executable into `$(go env GOPATH)/bin`.
Ensure that directory is on `PATH`. Once the module is published, it can also
be installed directly:

```sh
go install github.com/phillip-england/ibot/cmd/ibot@latest
```

Confirm the installation:

```sh
ibot --version
ibot --help
```

## Command Reference

| Command | Generated behavior | Available flags |
| --- | --- | --- |
| `ibot click [name]` | Click a captured coordinate | `--vary`, `--hold`, `--no-imports` |
| `ibot box name` | Click the center or a random point in a captured box | `--vary`, `--no-imports` |
| `ibot click_image [name]` | Locate and click an embedded image | `--vary`, `--confidence`, `--wait-for`, `--timeout`, `--stall`, `--all`, `--order`, `--gap`, `--hold`, `--no-imports` |
| `ibot serve` | Start the embedded local web application | `--addr`, `--open` |

Flags can be placed before or after the optional function name. All meaningful
flag combinations are supported. Two dependency rules are enforced before
screen capture begins:

- `--order` requires `--all`, unless its default value is `linear`.
- `--gap` requires `--all`.

`--timeout` is accepted without `--wait-for` but has no effect because a
non-waiting function performs only one search. All other `click_image` flags
can be combined directly.

## Web Application

Start the browser interface with the same installed binary:

```sh
ibot serve
```

The server listens on `127.0.0.1:8787` and opens the default browser. To keep
the browser closed or select another loopback port:

```sh
ibot serve --open=false
ibot serve --addr=127.0.0.1:9000
```

Non-loopback addresses are rejected because the application can observe global
keyboard input and capture the local screen. The HTTP layer also rejects
cross-origin browser requests, limits request bodies, and sets a restrictive
Content Security Policy.

The web interface exposes the same generator as the CLI:

1. Select **Point**, **Box**, or **Image**.
2. Configure the function name and flags.
3. Select **Start capture**.
4. Follow the live prompt and press `0` at the requested point or four corners.
5. Review and copy the generated Python source.

Capture prompts are streamed from Go to the browser while the global input
listener is active. No screenshot or generated source is uploaded anywhere;
all processing remains in the local `ibot` process.

The UI supports image confidence, waiting and timeout, fixed or ranged stalls,
all-match clicking, linear/backwards/random order, fixed or ranged gaps,
position variation, held keys, and import omission. CLI and web requests call
the same typed Go generator, so their output behavior is identical.

## Generated Python Runtime

The Go application does not execute generated functions. Install their runtime
libraries in the Python environment where the output will run:

```sh
python -m pip install pyautogui pillow opencv-python-headless
```

`opencv-python-headless` enables confidence-based image matching; it does not
prevent PyAutoGUI from capturing or controlling the local desktop. Generated
point and box functions that do not use image matching need only `pyautogui`.

## Point Clicks

```sh
ibot click click_submit
```

1. Move the pointer over the target position.
2. Press the `0` key. You do not need to click the mouse.
3. Paste the generated function into your project.

Vary the click position at execution time with `--vary`:

```sh
ibot click click_submit --vary=5
```

This chooses a new random X coordinate and Y coordinate each time
`click_submit()` runs. Each coordinate can be up to 5 pixels below or above the
captured coordinate. For example, a captured position of `(5, 5)` with
`--vary=5` can click any integer X and Y coordinate from `0` through `10`.
`--vary=0` is the default and always clicks the exact captured position.

The generated output for the command above includes the runtime randomness:

```python
import pyautogui
import random


def click_submit() -> None:
    """Click within 5 pixels of (800, 450) on each axis."""
    click_x = random.randint(795, 805)
    click_y = random.randint(445, 455)
    pyautogui.click(x=click_x, y=click_y)
```

Omit imports from the printed and clipboard output when they already exist in
the destination module:

```sh
ibot click --no-imports
ibot click click_submit --vary=5 --no-imports
```

With `--no-imports`, the destination must provide `pyautogui`. It must also
provide `random` when `--vary` is greater than zero.

Example output without variation:

```python
import pyautogui


def click_submit() -> None:
    """Click the screen at (800, 450)."""
    pyautogui.click(x=800, y=450)
```

If the function name is omitted, `ibot click` uses `click_position`.

### Holding Keys

Point and image click functions can hold one or more keyboard keys while they
click:

```sh
ibot click click_submit --hold=shift
ibot click click_submit --hold=shift+control+cmd
ibot click_image click_icon --hold=shift+control+cmd
```

Separate keys with `+`. Key names are case-insensitive. Common aliases are
normalized as follows:

- `control` and `ctl` become PyAutoGUI's `ctrl`
- `cmd` becomes `command`
- `windows` becomes `win`

Other values must be names from PyAutoGUI's `KEYBOARD_KEYS`, such as `alt`,
`option`, `shiftleft`, `ctrlright`, or a letter. The generated function checks
all names before pressing anything.

Keys are pressed in the listed order and released in reverse order. Generated
functions use `try/finally`, track each successful `keyDown`, and attempt every
corresponding `keyUp` even if a later key press or click raises an exception.

For `click_image`, image polling and `--stall` finish before keys are pressed.
With `--all`, keys remain held across the complete click sequence, including
`--gap` delays, and release after the last click or any failure.

## Box Clicks

Capture a rectangular region and generate a named function:

```sh
ibot box click_inventory
```

After starting the command, position the pointer and press `0` once for each
corner in this order:

1. Top-left
2. Top-right
3. Bottom-right
4. Bottom-left

The four captured points define an axis-aligned box. Without `--vary`, the
generated function clicks the integer center of that box:

```python
import pyautogui


def click_inventory() -> None:
    """Click the center of the captured box at (500, 350)."""
    pyautogui.click(x=500, y=350)
```

Use a number to select a new random point near the center every time the
generated function runs:

```sh
ibot box click_inventory --vary=4
```

Each axis can vary by up to 4 pixels from the center. The range is clamped to
the captured box, so even a large variation cannot click outside it.

Use `*` to select a new random point anywhere in the box every time the
function runs:

```sh
ibot box click_inventory --vary=all
```

Quoted `--vary='*'` is also accepted, but `all` avoids shell wildcard handling.
The generated function contains the random selection; the box is not
randomized while the code is being generated:

```python
import pyautogui
import random


def click_inventory() -> None:
    """Click a random point in the box (400, 300) to (600, 400)."""
    click_x = random.randint(400, 600)
    click_y = random.randint(300, 400)
    pyautogui.click(x=click_x, y=click_y)
```

`--no-imports` is also available for box functions:

```sh
ibot box click_inventory --vary=all --no-imports
```

The destination module must already provide `pyautogui`, plus `random` when
using numeric variation or `--vary=all`.

Run `ibot click --help` or `ibot box --help` for complete command-line help.

## Image Clicks

Capture an image and generate a function that finds and clicks it at runtime:

```sh
ibot click_image click_chrome_icon --vary=all
```

As with `box`, move the pointer to each corner and press `0` in this order:

1. Top-left
2. Top-right
3. Bottom-right
4. Bottom-left

`ibot` screenshots the resulting rectangle, encodes the PNG bytes as base64,
and places that data directly inside `click_chrome_icon()`. The generated
function does not depend on a separate image file. At runtime it decodes the
image in memory, locates it with PyAutoGUI, and clicks the detected region.

The click location follows the same rules as `box`:

```sh
# Click the detected image's center.
ibot click_image click_chrome_icon

# Randomize each axis by up to 1 pixel around its center.
ibot click_image click_chrome_icon --vary=1

# Click a random point anywhere in the detected image.
ibot click_image click_chrome_icon --vary=all
```

Random choices happen each time the generated function runs. Numeric ranges
are clamped to the detected image, so they cannot click outside it. Quoted
`--vary='*'` remains available as an alias for `--vary=all`.

`--all` and `--vary=all` are different:

- `--all` means click every matching image.
- `--vary=all` means choose any point inside each image that is clicked.
- Using both clicks every match at a separately randomized point.

Image matching defaults to 90% similarity rather than exact pixel equality.
Lower the threshold when the image rendering differs slightly:

```sh
ibot click_image click_chrome_icon --confidence=0.8
```

Lower values tolerate more differences but increase the risk of matching the
wrong image. `--confidence` must be greater than `0` and at most `1`.

### Waiting And Stalling

By default, the generated function checks once and raises an error when the
image is absent. Use `--wait-for` to poll every 0.25 seconds until it appears:

```sh
ibot click_image click_chrome_icon --wait-for
```

Waiting times out after 30 seconds by default. Set another positive duration
with `--timeout`:

```sh
ibot click_image click_chrome_icon --wait-for --timeout=20
```

`--timeout` only affects functions generated with `--wait-for`. If the image
does not appear before the deadline, the generated function raises an error
and does not click.

Use `--stall` to delay after the image is found but before it is clicked. A
single value always waits exactly that many seconds:

```sh
ibot click_image click_chrome_icon --wait-for --stall=1
```

A range chooses a new random delay within the inclusive range each time the
generated function runs:

```sh
ibot click_image click_chrome_icon --wait-for --timeout=20 --stall=10-20
```

Decimal durations such as `--stall=0.5-1.5` are supported. Stalling also works
without `--wait-for`; in that case the function checks once, then stalls only
if that immediate check finds the image.

### Clicking Every Match

Use `--all` when the captured image can appear multiple times and every match
should be clicked. The function name is optional in this form and defaults to
`click_image`:

```sh
ibot click_image --all
ibot click_image click_all_icons --all
```

Matches use `linear` order by default. Linear order sorts them from top to
bottom and then left to right. Choose another order with `--order`:

```sh
# Top-to-bottom, then left-to-right.
ibot click_image click_all_icons --all --order=linear

# Reverse linear order.
ibot click_image click_all_icons --all --order=backwards

# Choose a new random order each time the function runs.
ibot click_image click_all_icons --all --order=random
```

Add an exact delay between clicks with `--gap=1`, or choose a new random delay
for every gap from an inclusive range:

```sh
ibot click_image click_all_icons --all --gap=1
ibot click_image click_all_icons --all --order=random --gap=0.5-3.8
```

There is no gap after the final click. `--order` and `--gap` require `--all`.
The existing `--vary` rule is calculated separately for every match.
`--stall` happens once before the first click, while `--gap` happens between
clicks.

With `--wait-for --all`, the generated function waits until at least one match
exists, then clicks every match visible in that screenshot. It cannot wait for
a specific number of matches because no expected count is provided.

All match coordinates come from one screenshot taken before clicking starts.
If clicking an early match moves, removes, or rearranges later matches, their
saved coordinates can become stale. The function does not rescan between
clicks.

### Flag Compatibility

| Flag | Default | Works with | Execution behavior |
| --- | --- | --- | --- |
| `--vary=N` | `0` | Single or `--all` | Randomizes each axis by at most `N`, clamped inside each match |
| `--vary=all` | Center | Single or `--all` | Chooses any point inside each clicked match |
| `--confidence=N` | `0.9` | Everything | Sets OpenCV similarity from greater than `0` through `1` |
| `--wait-for` | Off | Everything | Polls every 0.25 seconds until at least one match appears |
| `--timeout=N` | `30` | `--wait-for` | Limits polling; otherwise has no effect |
| `--stall=N` | None | Everything | Waits once after matching and before keys are held |
| `--stall=MIN-MAX` | None | Everything | Chooses one random pre-click delay per function call |
| `--all` | Off | Everything | Uses every match from the successful screenshot |
| `--order=linear` | `linear` | `--all` | Sorts top-to-bottom, then left-to-right |
| `--order=backwards` | `linear` | `--all` | Reverses linear order |
| `--order=random` | `linear` | `--all` | Shuffles once per function call |
| `--gap=N` | None | `--all` | Waits exactly `N` seconds between clicks only |
| `--gap=MIN-MAX` | None | `--all` | Chooses a new random delay for each gap |
| `--hold=KEY+KEY` | None | Single or `--all` | Holds keys during clicks and gaps, then releases in reverse |
| `--no-imports` | Off | Everything | Omits imports but does not change runtime behavior |

### Execution Order

A generated image function always runs features in this order:

1. Decode the embedded PNG and determine logical screen size.
2. Search once, or poll when `--wait-for` is enabled.
3. Raise without pressing keys if no image is found before the timeout.
4. Sort or shuffle matches when `--all` is enabled.
5. Apply `--stall` once.
6. Validate every `--hold` key, then press keys in the listed order.
7. For each match, calculate `--vary`, click, and apply `--gap` unless it was the final match.
8. Release every successfully held key in reverse order through `finally`.

This means keys are never held while polling or stalling. With `--all`, they
remain held during gaps so every click uses the requested modifiers.

All options can be combined in one command:

```sh
ibot click_image click_targets \
  --confidence=0.85 \
  --wait-for --timeout=20 \
  --stall=1-2 \
  --all --order=random --gap=0.5-3.8 \
  --vary=all \
  --hold=shift+control+cmd \
  --no-imports
```

This waits up to 20 seconds, stalls for one random duration, randomizes the
match order, holds the normalized `shift+ctrl+command` combination, clicks a
random point in every match with a separately randomized gap, and releases all
keys afterward. Because `--no-imports` is present, the destination must provide
the dependencies listed below.

The generated function has this simplified structure; the actual base64 value
contains the complete captured PNG and generated error handling is omitted
here for readability:

```python
import base64
import io
import random

import pyautogui
from PIL import Image


def click_chrome_icon() -> None:
    """Locate the embedded image on screen and click it."""
    image_data = (
        "iVBORw0KGgoAAA...complete PNG data..."
    )
    image = Image.open(io.BytesIO(base64.b64decode(image_data)))
    screen_size = tuple(int(value) for value in pyautogui.size())

    def find_images():
        screen = pyautogui.screenshot().convert("RGB")
        if screen.size != screen_size:
            screen = screen.resize(screen_size, Image.Resampling.LANCZOS)
        match = pyautogui.locate(image, screen, confidence=0.9)
        return [] if match is None else [match]

    matches = find_images()
    # The generated function applies waiting, ordering, stalling, held keys,
    # click variation, gaps, and guaranteed key release as requested.
```

Use `--no-imports` when the destination already has the required imports:

```sh
ibot click_image click_chrome_icon --vary=all --no-imports
```

In that case, the destination must provide `base64`, `io`, `pyautogui`, and an
`Image` symbol from PIL. It must also provide `time` when waiting, stalling, or
using a gap, and `random` when click variation, random ordering, ranged
stalling, or ranged gaps are enabled.

Embedding makes the function portable as a single source artifact, but image
recognition is not guaranteed across different systems. Retina screenshots are
normalized to PyAutoGUI's logical coordinate size automatically. Browser zoom,
themes, font rendering, animation, major scaling differences, and application
updates can still prevent a match. If no match is found, the generated function
raises an error instead of clicking an unrelated location.

Run `ibot click_image --help` for complete command-line help.

Both help styles are supported: `ibot --help` and `ibot help`, or
`ibot click_image --help` and `ibot click_image help`.

On macOS, the terminal running `ibot` may need permission under **System
Settings > Privacy & Security > Accessibility** to observe the global key
press. Screen Recording permission may also be required to capture images.

## Development

```sh
make test
make build
make run
```

`make test` runs the Go suite, including 720 meaningful `click_image` option
combinations, CLI forwarding tests, generated-Python syntax checks when
`python3` is available, streamed HTTP tests, embedded-asset tests, and loopback
security tests.

### Architecture

- `cmd/ibot`: executable wiring, signal handling, clipboard, and server startup
- `internal/model`: typed options and shared CLI/web parsing
- `internal/generator`: Go-managed Python source generation
- `internal/capture`: global `0` listener, pointer capture, screenshot capture,
  Retina/logical-coordinate normalization, and PNG encoding
- `internal/app`: shared capture-to-generation workflows
- `internal/cli`: Cobra commands for `click`, `box`, `click_image`, and `serve`
- `internal/web`: loopback HTTP server, streamed capture API, and embedded UI

The generated Python uses `pyautogui`, Pillow, and OpenCV-backed PyScreeze
matching where image confidence is enabled. `random`, `time`, `base64`, and
`io` are emitted only when the selected flags require them.
