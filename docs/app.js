'use strict';
const demos = {
  list: { command: 'portguard list --range 3000-9000', description: 'See every visible TCP listener and bound UDP socket in an inclusive port range. Add --json for scripts and tooling.', output: 'PROTO  ENDPOINT          PID    PROCESS\nTCP    127.0.0.1:3000    42819  node\nTCP    127.0.0.1:5432    9132   postgres\nTCP    0.0.0.0:6379      0      unknown (docker:cache)' },
  inspect: { command: 'portguard inspect 3000', description: 'Follow the port back to its owner. See its parent, executable, service context, and bind exposure, with container metadata when available.', output: 'TCP 127.0.0.1:3000\n  node owns this tcp socket\n  Parent: npm (PID 42801)\n  Executable: /usr/local/bin/node\n  Exposure: loopback' },
  watch: { command: 'portguard watch 3000 --interval 1s', description: 'Keep one port in view as your development environment changes. Refreshes never overlap. Ctrl+C ends the session.', output: '◈ PORTGUARD     ● LIVE\n\n127.0.0.1:3000  TCP  42819  node\nParent  npm (42801)\n\n↑↓ navigate   / filter   r refresh   q quit' },
  kill: { command: 'portguard kill 3000', description: 'Review all visible TCP and UDP owners and type yes to request shutdown. Identity and ownership are rechecked. --force and --yes are separate, explicit choices.', output: 'Request graceful shutdown for ALL 1 owners\nof TCP/UDP port 3000:\n  PID 42819  node  parent npm (42801)\nThis affects every port the process owns.\nType yes to continue: ▏' }
};
const installs = {
  go: { command: 'go install github.com/Hell077/portguard@latest', note: 'Requires Go 1.27.3+. Add your Go bin directory to PATH, then run portguard.' },
  unix: { command: 'curl -fsSL https://raw.githubusercontent.com/Hell077/portguard/main/scripts/install.sh | sh', note: 'Downloads a checksummed release to ~/.local/bin. Add that directory to PATH if needed. No Go toolchain required.' },
  windows: { command: 'iwr https://raw.githubusercontent.com/Hell077/portguard/main/scripts/install.ps1 -UseBasicParsing | iex', note: 'Run in PowerShell. Downloads a checksummed release to %LOCALAPPDATA%\\Portguard. Add that directory to PATH if needed.' }
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
