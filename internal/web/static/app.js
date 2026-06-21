const form = document.querySelector('#generator-form');
const modeInputs = [...document.querySelectorAll('input[name="mode"]')];
const imageOptions = document.querySelector('#image-options');
const nameInput = document.querySelector('#name');
const nameLabel = document.querySelector('#name-label');
const outputKind = document.querySelector('#output-kind');
const importsRow = document.querySelector('#imports-row');
const outputEyebrow = document.querySelector('#output-eyebrow');
const outputHeading = document.querySelector('#output-heading');
const variationField = document.querySelector('#variation-field');
const varyInput = document.querySelector('#vary');
const anywhereRow = document.querySelector('#click-anywhere-row');
const anywhereInput = document.querySelector('#click-anywhere');
const gridOptions = document.querySelector('#grid-options');
const gridEnabledInput = document.querySelector('#grid-enabled');
const gridRowsInput = document.querySelector('#grid-rows');
const gridColumnsInput = document.querySelector('#grid-columns');
const gridCellInput = document.querySelector('#grid-cell');
const holdInput = document.querySelector('#hold');
const holdField = document.querySelector('#hold-field');
const delayInput = document.querySelector('#delay');
const delayField = document.querySelector('#delay-field');
const confidenceInput = document.querySelector('#confidence');
const recognitionHeading = document.querySelector('#recognition-heading');
const waitForRow = document.querySelector('#wait-for-row');
const noClickRow = document.querySelector('#no-click-row');
const waitGoneRow = document.querySelector('#wait-gone-row');
const timeoutField = document.querySelector('#timeout-field');
const multipleHeading = document.querySelector('#multiple-heading');
const multipleOptions = document.querySelector('#multiple-options');
const waitInput = document.querySelector('#wait-for');
const noClickInput = document.querySelector('#no-click');
const waitUntilGoneInput = document.querySelector('#wait-until-gone');
const timeoutInput = document.querySelector('#timeout');
const allInput = document.querySelector('#click-all');
const orderInput = document.querySelector('#order');
const gapInput = document.querySelector('#gap');
const noImportsInput = document.querySelector('#no-imports');
const captureButton = document.querySelector('#capture-button');
const copyButton = document.querySelector('#copy-button');
const copyInstallButton = document.querySelector('#copy-install-button');
const sourceOutput = document.querySelector('#source');
const installSection = document.querySelector('#install-section');
const installCommand = document.querySelector('#install-command');
const status = document.querySelector('#status');
const modeDescription = document.querySelector('#mode-description');
const captureSteps = document.querySelector('#capture-steps');
const savedTargets = document.querySelector('#saved-targets');
const targetsList = document.querySelector('#targets-list');
const targetsEmpty = document.querySelector('#targets-empty');
const targetsStatus = document.querySelector('#targets-status');

const modeConfig = {
  click: {
    name: 'click_position',
    description: 'Capture one coordinate; choose click behavior when the function runs.',
    steps: ['Move to the target point', 'Press 0 to record', 'Review generated source'],
  },
  box: {
    name: 'click_box',
    description: 'Capture four corners; choose the box grid and click behavior at runtime.',
    steps: ['Capture four corners clockwise', 'Press 0 at each corner', 'Review generated source'],
  },
  click_image: {
    name: 'click_image',
    description: 'Embed a screenshot; control matching, waiting, placement, and clicks at runtime.',
    steps: ['Capture image corners clockwise', 'Press 0 at each corner', 'Review embedded source'],
  },
  image_exists: {
    name: 'image_exists',
    description: 'Capture an image and generate a function that returns whether it is visible.',
    steps: ['Capture image corners clockwise', 'Press 0 at each corner', 'Review boolean function'],
  },
};

let previousDefault = modeConfig.click.name;

function currentMode() {
  return modeInputs.find(input => input.checked).value;
}

function updateMode() {
  const mode = currentMode();
  const config = modeConfig[mode];
  const usesImage = mode === 'click_image' || mode === 'image_exists';
  const checksExistence = mode === 'image_exists';
  const choices = mode === 'click'
    ? [['function', 'Python function'], ['json', 'Point JSON']]
    : mode === 'box'
      ? [['function', 'Python function'], ['json', 'Box JSON']]
      : [['function', 'Python function'], ['image', 'PNG image']];
  const previousOutput = outputKind.value;
  outputKind.replaceChildren(...choices.map(([value, label]) => {
    const option = document.createElement('option');
    option.value = value;
    option.textContent = label;
    return option;
  }));
  if (choices.some(([value]) => value === previousOutput)) outputKind.value = previousOutput;
  imageOptions.hidden = !usesImage;
  gridOptions.hidden = mode !== 'box';
  variationField.hidden = checksExistence;
  holdField.hidden = checksExistence;
  delayField.hidden = checksExistence;
  waitForRow.hidden = checksExistence;
  noClickRow.hidden = checksExistence;
  waitGoneRow.hidden = checksExistence;
  timeoutField.hidden = checksExistence;
  multipleHeading.hidden = checksExistence;
  multipleOptions.hidden = checksExistence;
  recognitionHeading.textContent = checksExistence ? 'Match settings' : 'Find and schedule';
  if (!nameInput.value || nameInput.value === previousDefault) nameInput.value = config.name;
  previousDefault = config.name;
  modeDescription.textContent = config.description;
  captureSteps.innerHTML = config.steps.map((step, index) => `<li class="${index === 0 ? 'active' : ''}">${step}</li>`).join('');
  varyInput.placeholder = mode === 'click' ? '0 or 5' : '0, 5, or all';
  anywhereRow.hidden = mode === 'click' || checksExistence;
  if (mode === 'click') anywhereInput.checked = false;
  varyInput.disabled = anywhereInput.checked;
  updateGridDependencies();
  updateDependencies();
  updateOutput();
}

function updateOutput() {
  const isFunction = outputKind.value === 'function';
  const mode = currentMode();
  importsRow.hidden = !isFunction;
  variationField.hidden = !isFunction || mode === 'image_exists';
  holdField.hidden = !isFunction || mode === 'image_exists';
  delayField.hidden = !isFunction || mode === 'image_exists';
  gridOptions.hidden = !isFunction || mode !== 'box';
  imageOptions.hidden = !isFunction || (mode !== 'click_image' && mode !== 'image_exists');
  nameLabel.textContent = isFunction ? 'Function name' : (outputKind.value === 'image' ? 'Image file name' : 'JSON file name');
  outputEyebrow.textContent = isFunction ? 'Generated source' : 'Saved asset';
  outputHeading.textContent = isFunction ? 'Python output' : 'Project output';
  copyButton.hidden = !isFunction;
}

function updateGridDependencies() {
  const enabled = currentMode() === 'box' && gridEnabledInput.checked;
  const cellCount = Number(gridRowsInput.value) * Number(gridColumnsInput.value);
  gridRowsInput.disabled = !enabled;
  gridColumnsInput.disabled = !enabled;
  gridCellInput.disabled = !enabled;
  gridCellInput.max = String(Math.max(0, cellCount - 1));
  anywhereRow.querySelector('small').textContent = enabled
    ? 'Choose a random point anywhere within the selected grid cell.'
    : 'Choose a random point anywhere within the captured target.';
}

function updateDependencies() {
  if (waitInput.checked) waitUntilGoneInput.checked = false;
  const waitUntilGone = currentMode() === 'click_image' && waitUntilGoneInput.checked;
  if (!waitInput.checked) noClickInput.checked = false;
  noClickInput.disabled = !waitInput.checked || waitUntilGone;
  const noClick = noClickInput.checked;
  timeoutInput.disabled = !waitInput.checked && !waitUntilGone;
  varyInput.disabled = waitUntilGone || noClick || anywhereInput.checked;
  anywhereInput.disabled = waitUntilGone || noClick;
  holdInput.disabled = waitUntilGone || noClick;
  delayInput.disabled = waitUntilGone || noClick;
  allInput.disabled = waitUntilGone || noClick;
  orderInput.disabled = waitUntilGone || noClick || !allInput.checked;
  gapInput.disabled = waitUntilGone || noClick || !allInput.checked;
  if (!allInput.checked) {
    orderInput.value = 'linear';
    gapInput.value = '';
  }
}

function setStatus(message, state = 'working') {
  status.className = `status ${state}`;
  status.querySelector('span').textContent = message;
}

function payload() {
  const useGrid = currentMode() === 'box' && gridEnabledInput.checked;
  const checksExistence = currentMode() === 'image_exists';
  const waitUntilGone = currentMode() === 'click_image' && waitUntilGoneInput.checked;
  const noClick = currentMode() === 'click_image' && noClickInput.checked;
  return {
    mode: currentMode(),
    output: outputKind.value,
    name: nameInput.value.trim(),
    vary: checksExistence || waitUntilGone || noClick ? '0' : (anywhereInput.checked ? 'all' : (varyInput.value.trim() || '0')),
    gridRows: useGrid ? Number(gridRowsInput.value) : 0,
    gridColumns: useGrid ? Number(gridColumnsInput.value) : 0,
    gridCell: useGrid ? Number(gridCellInput.value) : 0,
    hold: checksExistence || waitUntilGone || noClick ? '' : holdInput.value.trim(),
    delay: checksExistence || waitUntilGone || noClick ? '' : delayInput.value.trim(),
    noImports: noImportsInput.checked,
    confidence: Number(confidenceInput.value),
    waitFor: checksExistence ? false : waitInput.checked,
    noClick: checksExistence ? false : noClick,
    waitUntilGone: checksExistence ? false : waitUntilGone,
    timeout: Number(timeoutInput.value),
    all: checksExistence || waitUntilGone || noClick ? false : allInput.checked,
    order: checksExistence || waitUntilGone || noClick ? 'linear' : orderInput.value,
    gap: checksExistence || waitUntilGone || noClick ? '' : gapInput.value.trim(),
  };
}

async function readEvents(response) {
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(error.error || 'Capture request failed');
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
    const lines = buffer.split('\n');
    buffer = lines.pop();
    for (const line of lines) {
      if (!line.trim()) continue;
      const event = JSON.parse(line);
      if (event.type === 'prompt' || event.type === 'status') setStatus(event.message);
      if (event.type === 'error') throw new Error(event.message);
      if (event.type === 'source') {
        sourceOutput.textContent = event.source;
        installCommand.textContent = event.installCommand;
        installSection.hidden = !event.installCommand;
        copyButton.disabled = false;
        setStatus(event.message || 'Generated successfully', 'idle');
      }
      if (event.type === 'asset') {
        sourceOutput.textContent = event.message;
        installSection.hidden = true;
        setStatus(event.message, 'idle');
        loadTargets();
      }
    }
    if (done) break;
  }
}

function targetValue(target) {
  if (target.kind === 'point') return `x=${target.point.x}, y=${target.point.y}`;
  return target.corners.map(point => `(${point.x}, ${point.y})`).join(' ');
}

async function loadTargets() {
  try {
    const response = await fetch('/api/targets');
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Could not load JSON targets');
    const configured = data.pointsConfigured || data.boxesConfigured;
    savedTargets.hidden = !configured;
    targetsList.replaceChildren();
    targetsEmpty.hidden = data.targets.length !== 0;
    data.targets.forEach(target => {
      const row = document.createElement('div');
      row.className = 'target-row';
      const meta = document.createElement('div');
      meta.className = 'target-meta';
      const name = document.createElement('strong');
      name.textContent = target.name;
      const value = document.createElement('code');
      value.textContent = `${target.kind} / ${targetValue(target)}`;
      meta.append(name, value);
      const button = document.createElement('button');
      button.className = 'copy-button target-edit';
      button.type = 'button';
      button.textContent = 'Edit with 0';
      button.addEventListener('click', () => editTarget(target, button));
      row.append(meta, button);
      targetsList.append(row);
    });
  } catch (error) {
    savedTargets.hidden = false;
    targetsStatus.textContent = error.message;
  }
}

async function editTarget(target, button) {
  button.disabled = true;
  document.querySelectorAll('.target-edit').forEach(item => { item.disabled = true; });
  targetsStatus.textContent = `Move to the ${target.kind} target and press 0`;
  try {
    const response = await fetch(`/api/targets/${target.kind}/${encodeURIComponent(target.name)}/capture`, { method: 'POST' });
    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      throw new Error(data.error || 'Target update failed');
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
      const lines = buffer.split('\n');
      buffer = lines.pop();
      for (const line of lines) {
        if (!line.trim()) continue;
        const event = JSON.parse(line);
        if (event.type === 'prompt' || event.type === 'status') targetsStatus.textContent = event.message;
        if (event.type === 'error') throw new Error(event.message);
        if (event.type === 'target') targetsStatus.textContent = event.message;
      }
      if (done) break;
    }
    await loadTargets();
  } catch (error) {
    targetsStatus.textContent = error.message;
  } finally {
    document.querySelectorAll('.target-edit').forEach(item => { item.disabled = false; });
  }
}

form.addEventListener('submit', async event => {
  event.preventDefault();
  captureButton.disabled = true;
  copyButton.disabled = true;
  installSection.hidden = true;
  setStatus('Starting local capture...');
  try {
    const response = await fetch('/api/capture', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload()),
    });
    await readEvents(response);
  } catch (error) {
    setStatus(error.message, 'error');
  } finally {
    captureButton.disabled = false;
  }
});

copyButton.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(sourceOutput.textContent);
    setStatus('Copied generated source', 'idle');
  } catch {
    setStatus('Browser clipboard permission was denied', 'error');
  }
});

copyInstallButton.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(installCommand.textContent);
    setStatus('Copied uv install command', 'idle');
  } catch {
    setStatus('Browser clipboard permission was denied', 'error');
  }
});

modeInputs.forEach(input => input.addEventListener('change', updateMode));
outputKind.addEventListener('change', updateOutput);
anywhereInput.addEventListener('change', () => {
  varyInput.disabled = anywhereInput.checked;
});
gridEnabledInput.addEventListener('change', updateGridDependencies);
gridRowsInput.addEventListener('input', updateGridDependencies);
gridColumnsInput.addEventListener('input', updateGridDependencies);
waitInput.addEventListener('change', updateDependencies);
noClickInput.addEventListener('change', updateDependencies);
waitUntilGoneInput.addEventListener('change', () => {
  if (waitUntilGoneInput.checked) waitInput.checked = false;
  updateDependencies();
});
allInput.addEventListener('change', updateDependencies);
updateMode();
updateDependencies();
loadTargets();
