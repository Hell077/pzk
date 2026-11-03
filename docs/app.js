'use strict';
const demos = {
  list: { command: 'pzk list --range 3000-9000', description: 'See every visible TCP listener and bound UDP socket in an inclusive port range. Add --json for scripts and tooling.', output: 'PROTO  ENDPOINT          PID    PROCESS\nTCP    127.0.0.1:3000    42819  node\nTCP    127.0.0.1:5432    9132   postgres\nTCP    0.0.0.0:6379      0      unknown (docker:cache)' },
  inspect: { command: 'pzk inspect 3000', description: 'Follow the port back to its owner. See its parent, executable, service context, and bind exposure, with container metadata when available.', output: 'TCP 127.0.0.1:3000\n  node owns this tcp socket\n  Parent: npm (PID 42801)\n  Executable: /usr/local/bin/node\n  Exposure: loopback' },
  watch: { command: 'pzk watch --range 7999-8080 --interval 1s', description: 'Keep a port or an inclusive port range in view as your development environment changes. Refreshes never overlap. Ctrl+C ends the session.', output: '◈ PZK · PortZapKill     ● LIVE · ports 7999–8080\n\n127.0.0.1:8000  TCP  42819  node\nParent  npm (42801)\n\n↑↓ navigate   / filter   r refresh   q quit' },
  kill: { command: 'pzk kill 3000', description: 'Review all visible TCP and UDP owners and type yes to request shutdown. Identity and ownership are rechecked. --force and --yes are separate, explicit choices.', output: 'Request graceful shutdown for ALL 1 owners\nof TCP/UDP port 3000:\n  PID 42819  node  parent npm (42801)\nThis affects every port the process owns.\nType yes to continue: ▏' }
};
const installs = {
  go: { command: 'go install github.com/Hell077/pzk@latest', note: 'Requires Go 1.27.3+. Add your Go bin directory to PATH, then run pzk.' },
  unix: { command: 'curl -fsSL https://raw.githubusercontent.com/Hell077/pzk/main/scripts/install.sh | sh', note: 'Downloads a checksummed release to ~/.local/bin. Add that directory to PATH if needed. No Go toolchain required.' },
  windows: { command: 'iwr https://raw.githubusercontent.com/Hell077/pzk/main/scripts/install.ps1 -UseBasicParsing | iex', note: 'Run in PowerShell. Downloads a checksummed release to %LOCALAPPDATA%\\PortZapKill. Add that directory to PATH if needed.' }
};
function setupTabs(selector, onSelect) {
  const tabs = [...document.querySelectorAll(selector)];
  function select(tab) {
    tabs.forEach(item => { item.setAttribute('aria-selected', String(item === tab)); item.tabIndex = item === tab ? 0 : -1; });
    onSelect(tab);
  }
  tabs.forEach((tab, index) => {
    tab.addEventListener('click', () => select(tab));
    tab.addEventListener('keydown', event => {
      let next;
      if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = (index + 1) % tabs.length;
      if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = (index + tabs.length - 1) % tabs.length;
      if (event.key === 'Home') next = 0;
      if (event.key === 'End') next = tabs.length - 1;
      if (next !== undefined) { event.preventDefault(); select(tabs[next]); tabs[next].focus(); }
    });
  });
}
setupTabs('[data-command]', tab => {
  const demo = demos[tab.dataset.command];
  document.querySelector('#demo-command').textContent = demo.command;
  document.querySelector('#demo-description').textContent = demo.description;
  document.querySelector('#demo-output').textContent = demo.output;
  document.querySelector('#command-panel').setAttribute('aria-labelledby', tab.id);
});
setupTabs('[data-install]', tab => {
  const install = installs[tab.dataset.install];
  document.querySelector('#install-command').textContent = install.command;
  document.querySelector('#install-note').textContent = install.note;
  document.querySelector('#install-panel').setAttribute('aria-labelledby', tab.id);
  document.querySelector('#copy-status').textContent = '';
});
document.querySelector('#copy').addEventListener('click', async () => {
  const status = document.querySelector('#copy-status');
  try {
    await navigator.clipboard.writeText(document.querySelector('#install-command').textContent);
    status.textContent = 'Copied. Paste it into your terminal when you are ready.';
  } catch {
    status.textContent = 'Clipboard is unavailable. Select and copy the command above.';
  }
});

// Reveal content once as it enters the viewport; never hide content without JS support.
const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
if ('IntersectionObserver' in window && !reducedMotion.matches) {
  const sections = document.querySelectorAll('.product, .workflow-heading, .command-workbench, .safety > div, .install-heading, .install-box, .downloads');
  sections.forEach(section => section.classList.add('reveal'));
  document.documentElement.classList.add('motion-ready');
  const reveals = new IntersectionObserver(entries => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('is-visible');
        reveals.unobserve(entry.target);
      }
    });
  }, { threshold: 0.08 });
  sections.forEach(section => reveals.observe(section));
}

// One frame per pointer event burst; no animation loop runs while the pointer is idle.
const finePointer = window.matchMedia('(hover: hover) and (pointer: fine)');
let stopPointerEffects = () => {};
function syncPointerEffects() {
  stopPointerEffects();
  stopPointerEffects = () => {};
  if (reducedMotion.matches || !finePointer.matches) return;
  const layer = document.createElement('div');
  layer.className = 'pointer-layer';
  layer.setAttribute('aria-hidden', 'true');
  const halo = document.createElement('div');
  halo.className = 'pointer-halo';
  layer.append(halo);
  document.body.append(layer);
  const terminal = document.querySelector('.terminal');
  let frame = 0;
  let x = 0;
  let y = 0;
  function reset() {
    cancelAnimationFrame(frame);
    frame = 0;
    layer.classList.remove('is-active');
    terminal?.classList.remove('pointer-active');
  }
  function render() {
    frame = 0;
    const rect = terminal?.getBoundingClientRect();
    const inside = rect && x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom;
    halo.style.transform = `translate3d(${x - 220}px, ${y - 220}px, 0)`;
    layer.classList.add('is-active');
    if (inside) {
      const rx = (x - rect.left) / rect.width;
      const ry = (y - rect.top) / rect.height;
      terminal.style.setProperty('--tilt-x', `${(0.5 - ry) * 4}deg`);
      terminal.style.setProperty('--tilt-y', `${(rx - 0.5) * 4}deg`);
      terminal.style.setProperty('--shine-x', `${rx * 100}%`);
      terminal.style.setProperty('--shine-y', `${ry * 100}%`);
      terminal.classList.add('pointer-active');
    } else {
      terminal?.classList.remove('pointer-active');
    }
  }
  function move(event) {
    if (event.pointerType === 'touch') return;
    x = event.clientX;
    y = event.clientY;
    if (!frame) frame = requestAnimationFrame(render);
  }
  document.addEventListener('pointermove', move, { passive: true });
  document.addEventListener('pointerleave', reset);
  window.addEventListener('blur', reset);
  stopPointerEffects = () => {
    reset();
    document.removeEventListener('pointermove', move);
    document.removeEventListener('pointerleave', reset);
    window.removeEventListener('blur', reset);
    layer.remove();
  };
}
finePointer.addEventListener('change', syncPointerEffects);
reducedMotion.addEventListener('change', syncPointerEffects);
syncPointerEffects();
