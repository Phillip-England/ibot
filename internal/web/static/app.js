const form = document.querySelector('#generator-form');
const modeInputs = [...document.querySelectorAll('input[name="mode"]')];
const imageOptions = document.querySelector('#image-options');
const nameInput = document.querySelector('#name');
const varyInput = document.querySelector('#vary');
const anywhereRow = document.querySelector('#click-anywhere-row');
const anywhereInput = document.querySelector('#click-anywhere');
const holdInput = document.querySelector('#hold');
const confidenceInput = document.querySelector('#confidence');
const waitInput = document.querySelector('#wait-for');
const timeoutInput = document.querySelector('#timeout');
const stallInput = document.querySelector('#stall');
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

const modeConfig = {
  click: {
    name: 'click_position',
    description: 'Capture one coordinate and click it later.',
    steps: ['Move to the target point', 'Press 0 to record', 'Review generated source'],
  },
  box: {
    name: 'click_box',
    description: 'Capture four corners and click within the resulting box.',
    steps: ['Capture four corners clockwise', 'Press 0 at each corner', 'Review generated source'],
  },
  click_image: {
    name: 'click_image',
    description: 'Embed a screenshot, locate every requested match, and click.',
    steps: ['Capture image corners clockwise', 'Press 0 at each corner', 'Review embedded source'],
  },
};

let previousDefault = modeConfig.click.name;

function currentMode() {
  return modeInputs.find(input => input.checked).value;
}

function updateMode() {
  const mode = currentMode();
  const config = modeConfig[mode];
  imageOptions.hidden = mode !== 'click_image';
  if (!nameInput.value || nameInput.value === previousDefault) nameInput.value = config.name;
  previousDefault = config.name;
  modeDescription.textContent = config.description;
  captureSteps.innerHTML = config.steps.map((step, index) => `<li class="${index === 0 ? 'active' : ''}">${step}</li>`).join('');
  varyInput.placeholder = mode === 'click' ? '0 or 5' : '0, 5, or all';
  anywhereRow.hidden = mode === 'click';
  if (mode === 'click') anywhereInput.checked = false;
  varyInput.disabled = anywhereInput.checked;
}

function updateDependencies() {
  timeoutInput.disabled = !waitInput.checked;
  orderInput.disabled = !allInput.checked;
  gapInput.disabled = !allInput.checked;
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
  return {
    mode: currentMode(),
    name: nameInput.value.trim(),
    vary: anywhereInput.checked ? 'all' : (varyInput.value.trim() || '0'),
    hold: holdInput.value.trim(),
    noImports: noImportsInput.checked,
    confidence: Number(confidenceInput.value),
    waitFor: waitInput.checked,
    timeout: Number(timeoutInput.value),
    stall: stallInput.value.trim(),
    all: allInput.checked,
    order: orderInput.value,
    gap: gapInput.value.trim(),
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
    }
    if (done) break;
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
anywhereInput.addEventListener('change', () => {
  varyInput.disabled = anywhereInput.checked;
});
waitInput.addEventListener('change', updateDependencies);
allInput.addEventListener('change', updateDependencies);
updateMode();
updateDependencies();
