package handlers

import "html/template"

func buildEnglishPages() map[string]DocsContentPage {
	return map[string]DocsContentPage{
		"quick-start":      {Title: "Quick Start", Content: enQuickStart},
		"installation":     {Title: "Installation Guide", Content: enInstallation},
		"cli-commands":     {Title: "CLI Commands", Content: enCLICommands},
		"config-reference": {Title: "Configuration Reference", Content: enConfigReference},
		"common-issues":    {Title: "Common Issues", Content: enTroubleshooting},
		"faq":              {Title: "FAQ", Content: enFAQ},
		"cli":              {Title: "CLI Overview", Content: enCLIOverview},
		"cli/version":      {Title: "version Command", Content: enCLIVersion},
		"cli/license":      {Title: "license Command", Content: enCLILicense},
		"cli/status":       {Title: "status Command", Content: enCLIStatus},
		"cli/health":       {Title: "health Command", Content: enCLIHealth},
		"cli/preflight":    {Title: "preflight Command", Content: enCLIPreflight},
		"cli/mode":         {Title: "mode Command", Content: enCLIMode},
		"cli/install":      {Title: "install Command", Content: enCLIInstall},
		"cli/update":       {Title: "update Command", Content: enCLIUpdate},
		"cli/incident":     {Title: "incident Command", Content: enCLIIncident},
		"cli/pilot":        {Title: "pilot Command", Content: enCLIPilot},
		"cli/report":       {Title: "report Command", Content: enCLIReport},
	}
}

var enQuickStart template.HTML = `<div class="docs-welcome-card">
<h2>Quick Start Guide</h2>
<p>Get Kiro WAF installed and protecting your server in under 15 minutes.</p>
</div>

<h2>Prerequisites</h2>
<ul>
<li>A Linux server (Ubuntu 20.04+, Debian 11+, CentOS 8+, Rocky 8+, Fedora 36+, or Arch)</li>
<li>Root or sudo access</li>
<li>A valid Kiro WAF license key</li>
<li>Network connectivity to the Kiro management server</li>
</ul>

<h2>Step 1: Download and Run the Installer</h2>
<pre><code>curl -fsSL https://firewall.vpsgen.com/install.sh -o install.sh
sudo bash install.sh --license-key YOUR-LICENSE-KEY</code></pre>
<p>The installer automatically detects your OS, installs dependencies, downloads the client binary, and starts the service.</p>

<h2>Step 2: Configure Your Site</h2>
<p>Edit the configuration file at <code>/etc/kiro/kiro.yaml</code>:</p>
<pre><code>mode: full
license_key: YOUR-LICENSE-KEY

website:
  enabled: true
  cloudflare: true
  tls_mode: flexible_http
  sites:
    - domains:
        - example.com
      backend: http://127.0.0.1:3000

protection:
  profile: balanced
  waf: true
  bot: true</code></pre>

<h2>Step 3: Restart the Service</h2>
<pre><code>sudo systemctl restart kiro-client-waf</code></pre>

<h2>Step 4: Verify</h2>
<pre><code>sudo systemctl status kiro-client-waf</code></pre>
<p>You should see <code>active (running)</code>. Your site is now protected by Kiro WAF.</p>
`

var enInstallation template.HTML = `<h2>Installation Guide</h2>

<h3>Supported Operating Systems</h3>
<table>
<tr><th>Distribution</th><th>Minimum Version</th><th>Package Manager</th></tr>
<tr><td>Ubuntu</td><td>20.04 LTS</td><td>apt</td></tr>
<tr><td>Debian</td><td>11 (Bullseye)</td><td>apt</td></tr>
<tr><td>CentOS</td><td>8</td><td>yum/dnf</td></tr>
<tr><td>Rocky Linux</td><td>8</td><td>dnf</td></tr>
<tr><td>Fedora</td><td>36</td><td>dnf</td></tr>
<tr><td>Arch Linux</td><td>Rolling</td><td>pacman</td></tr>
</table>

<h3>System Requirements</h3>
<ul>
<li><strong>CPU:</strong> 1 core minimum, 2+ cores recommended</li>
<li><strong>RAM:</strong> 512 MB minimum, 1 GB recommended</li>
<li><strong>Disk:</strong> 100 MB free space</li>
<li><strong>Network:</strong> Outbound HTTPS access to the management server</li>
</ul>

<h3>Automatic Installation</h3>
<p>The recommended installation method uses the automated install script:</p>
<pre><code>curl -fsSL https://firewall.vpsgen.com/install.sh -o install.sh
sudo bash install.sh --license-key YOUR-LICENSE-KEY</code></pre>

<h3>XDP Mode Installation</h3>
<p>For high-performance packet filtering with XDP/eBPF support:</p>
<pre><code>sudo bash install.sh --license-key YOUR-LICENSE-KEY --xdp-mode</code></pre>
<p>This installs additional build dependencies (clang, llvm, libbpf-dev) for XDP filter compilation.</p>

<h3>What the Installer Does</h3>
<ol>
<li>Detects your OS distribution and version</li>
<li>Installs required dependencies (curl, sha256sum, systemctl)</li>
<li>Downloads the Kiro WAF client binary with SHA-256 verification</li>
<li>Creates the configuration directory at <code>/etc/kiro/</code></li>
<li>Installs and enables the systemd service</li>
<li>Starts the Kiro WAF client service</li>
</ol>

<h3>Post-Installation</h3>
<p>After installation, configure your site in <code>/etc/kiro/kiro.yaml</code> and restart the service:</p>
<pre><code>sudo systemctl restart kiro-client-waf
sudo systemctl status kiro-client-waf</code></pre>

<h3>Uninstallation</h3>
<pre><code>sudo systemctl stop kiro-client-waf
sudo systemctl disable kiro-client-waf
sudo rm /usr/local/bin/kiro-client-waf
sudo rm -rf /etc/kiro/</code></pre>
`

var enCLICommands template.HTML = `<h2>CLI Commands (kiro-cli)</h2>
<p>The Kiro CLI provides commands for managing and monitoring the WAF system from the command line. See the <a href="/docs/en/cli">CLI overview page</a> for full details on each command.</p>

<h3>Command List</h3>
<table>
<tr><th>Command</th><th>Description</th><th>Details</th></tr>
<tr><td><code>version</code></td><td>Display current version</td><td><a href="/docs/en/cli/version">View</a></td></tr>
<tr><td><code>license fingerprint</code></td><td>Generate machine fingerprint hash</td><td><a href="/docs/en/cli/license">View</a></td></tr>
<tr><td><code>status</code></td><td>Runtime system status</td><td><a href="/docs/en/cli/status">View</a></td></tr>
<tr><td><code>health</code></td><td>Comprehensive health check</td><td><a href="/docs/en/cli/health">View</a></td></tr>
<tr><td><code>preflight</code></td><td>Pre-deployment prerequisite checks</td><td><a href="/docs/en/cli/preflight">View</a></td></tr>
<tr><td><code>mode</code></td><td>Show/change operating mode</td><td><a href="/docs/en/cli/mode">View</a></td></tr>
<tr><td><code>install</code></td><td>Installation management (plan, stage, apply)</td><td><a href="/docs/en/cli/install">View</a></td></tr>
<tr><td><code>update</code></td><td>OTA update management (check, apply, rollback)</td><td><a href="/docs/en/cli/update">View</a></td></tr>
<tr><td><code>incident</code></td><td>Generate incident reports</td><td><a href="/docs/en/cli/incident">View</a></td></tr>
<tr><td><code>pilot</code></td><td>Generate pilot go/no-go reports</td><td><a href="/docs/en/cli/pilot">View</a></td></tr>
<tr><td><code>report</code></td><td>Comprehensive system report</td><td><a href="/docs/en/cli/report">View</a></td></tr>
</table>

<h3>Quick Usage</h3>
<pre><code>kiro-cli version
kiro-cli status --config /etc/kiro/kiro.yaml
kiro-cli health --config /etc/kiro/kiro.yaml
kiro-cli update check --master-url https://firewall.vpsgen.com</code></pre>

<h3>Common Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success</td></tr>
<tr><td>1</td><td>Execution error (validation, permission, runtime)</td></tr>
<tr><td>2</td><td>Usage error (invalid command, missing required parameter)</td></tr>
</table>
`

var enCLIOverview template.HTML = `<div class="docs-welcome-card">
<h2>CLI Overview (kiro-cli)</h2>
<p>The Kiro CLI tool provides administration and diagnostic commands for the WAF system.</p>
</div>

<div class="cli-search-box">
<input type="text" id="cli-search" placeholder="Search commands..." onkeyup="filterCLICommands()" aria-label="Search CLI commands">
</div>

<h2>Command Index</h2>
<div class="cli-toc" id="cli-command-list">
<table>
<tr><th>Command</th><th>Description</th><th>Details</th></tr>
<tr class="cli-cmd-row" data-cmd="version"><td><code>kiro-cli version</code></td><td>Display build version (semver X.Y.Z)</td><td><a href="/docs/en/cli/version">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="license fingerprint"><td><code>kiro-cli license fingerprint</code></td><td>Generate unique machine fingerprint hash</td><td><a href="/docs/en/cli/license">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="status"><td><code>kiro-cli status</code></td><td>Runtime status (mode, uptime, license, version)</td><td><a href="/docs/en/cli/status">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="health"><td><code>kiro-cli health</code></td><td>Comprehensive system health check</td><td><a href="/docs/en/cli/health">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="preflight"><td><code>kiro-cli preflight</code></td><td>Pre-deployment prerequisite checks</td><td><a href="/docs/en/cli/preflight">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="mode show set"><td><code>kiro-cli mode</code></td><td>Show/change operating mode (server/full)</td><td><a href="/docs/en/cli/mode">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="install plan stage-lab apply-lab"><td><code>kiro-cli install</code></td><td>Installation management (plan, stage-lab, apply-lab)</td><td><a href="/docs/en/cli/install">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="update check apply rollback"><td><code>kiro-cli update</code></td><td>OTA update management (check, apply, rollback)</td><td><a href="/docs/en/cli/update">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="incident report"><td><code>kiro-cli incident report</code></td><td>Generate incident reports (JSON + Markdown)</td><td><a href="/docs/en/cli/incident">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="pilot report"><td><code>kiro-cli pilot report</code></td><td>Generate pilot go/no-go reports</td><td><a href="/docs/en/cli/pilot">Details →</a></td></tr>
<tr class="cli-cmd-row" data-cmd="report"><td><code>kiro-cli report</code></td><td>Comprehensive system report</td><td><a href="/docs/en/cli/report">Details →</a></td></tr>
</table>
</div>

<h2>Installation</h2>
<p>The Kiro CLI is installed automatically with the Kiro WAF client. The binary is located at <code>/usr/local/bin/kiro-cli</code>.</p>

<h2>Common Exit Codes</h2>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — command completed without errors</td></tr>
<tr><td>1</td><td>Execution error — validation failed, permission denied, or runtime error</td></tr>
<tr><td>2</td><td>Usage error — invalid command or missing required parameter</td></tr>
</table>

<h2>Quick Usage</h2>
<pre><code># Check version
kiro-cli version

# View system status
kiro-cli status --config /etc/kiro/kiro.yaml

# Health check
kiro-cli health --config /etc/kiro/kiro.yaml

# Check for updates
kiro-cli update check --master-url https://firewall.vpsgen.com</code></pre>

<script>
function filterCLICommands() {
  var input = document.getElementById("cli-search").value.toLowerCase();
  var rows = document.querySelectorAll(".cli-cmd-row");
  rows.forEach(function(row) {
    var cmd = row.getAttribute("data-cmd");
    var text = row.textContent.toLowerCase();
    if (cmd.indexOf(input) > -1 || text.indexOf(input) > -1) {
      row.style.display = "";
    } else {
      row.style.display = "none";
    }
  });
}
</script>
`

var enCLIVersion template.HTML = `<h2>kiro-cli version</h2>
<p>Displays the current build version of Kiro CLI in semver format.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli version</code></pre>

<h3>Parameters</h3>
<p>This command takes no parameters.</p>

<h3>Output</h3>
<p>Returns a version string in semver format <code>X.Y.Z</code> (e.g., <code>1.2.3</code>) or <code>X.Y.Z-suffix</code> (e.g., <code>0.1.0-dev</code>).</p>

<h3>Example</h3>
<pre><code>$ kiro-cli version
1.0.0</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — version displayed</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/status">kiro-cli status</a> — view full status including version</li>
<li><a href="/docs/en/cli/report">kiro-cli report</a> — system report including version</li>
</ul>
`

var enCLILicense template.HTML = `<h2>kiro-cli license fingerprint</h2>
<p>Generates a unique machine fingerprint hash for the current server. The fingerprint is used to identify the machine when registering a license.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli license fingerprint [--salt &lt;value&gt;]</code></pre>

<h3>Parameters</h3>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--salt</code></td><td>No</td><td>string</td><td>(empty)</td><td>Custom salt value to generate a distinct fingerprint</td></tr>
</table>

<h3>Output</h3>
<p>Returns a 64-character lowercase hex hash string (SHA-256). The result is deterministic — the same machine and salt always produce the same result.</p>

<h3>Example</h3>
<pre><code># Default fingerprint
$ kiro-cli license fingerprint
a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd

# Fingerprint with custom salt
$ kiro-cli license fingerprint --salt "production-server-01"
f9e8d7c6b5a4321098765432109876543210987654321098765432109876fedc</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — fingerprint displayed</td></tr>
<tr><td>1</td><td>Error — unable to read hardware information</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/status">kiro-cli status</a> — view current license status</li>
</ul>
`

var enCLIStatus template.HTML = `<h2>kiro-cli status</h2>
<p>Displays the current runtime status of the Kiro WAF system as JSON.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli status --config &lt;path&gt;</code></pre>

<h3>Parameters</h3>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--config</code></td><td>Yes</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Path to YAML configuration file</td></tr>
</table>

<h3>JSON Output</h3>
<table>
<tr><th>Field</th><th>Type</th><th>Description</th></tr>
<tr><td><code>mode</code></td><td>string</td><td>Operating mode: "server" or "full"</td></tr>
<tr><td><code>uptime</code></td><td>string</td><td>Service uptime (e.g., "2h30m")</td></tr>
<tr><td><code>license_status</code></td><td>string</td><td>License status: active, suspended, downgraded</td></tr>
<tr><td><code>version</code></td><td>string</td><td>Current version (semver)</td></tr>
</table>

<h3>Example</h3>
<pre><code>$ kiro-cli status --config /etc/kiro/kiro.yaml
{
  "mode": "full",
  "uptime": "48h12m",
  "license_status": "active",
  "version": "1.0.0",
  "plan": "community",
  "sites": 1,
  "services": {
    "firewall": "active",
    "proxy": "active",
    "waf": "active",
    "bot_protection": "active"
  }
}</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — status displayed</td></tr>
<tr><td>1</td><td>Error — unable to read config or query status</td></tr>
<tr><td>2</td><td>Missing required --config parameter</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/health">kiro-cli health</a> — detailed health check</li>
<li><a href="/docs/en/cli/report">kiro-cli report</a> — comprehensive report</li>
</ul>
`

var enCLIHealth template.HTML = `<h2>kiro-cli health</h2>
<p>Performs a comprehensive health check including service status, preflight checks, and overall system health.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli health --config &lt;path&gt; [--os-release &lt;path&gt;] [--preflight-writable-root &lt;path&gt;] [--skip-command-checks]</code></pre>

<h3>Parameters</h3>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--config</code></td><td>Yes</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Path to configuration file</td></tr>
<tr><td><code>--os-release</code></td><td>No</td><td>string</td><td>/etc/os-release</td><td>Path to os-release file (for testing)</td></tr>
<tr><td><code>--preflight-writable-root</code></td><td>No</td><td>string</td><td>/</td><td>Root directory for writable checks (for testing)</td></tr>
<tr><td><code>--skip-command-checks</code></td><td>No</td><td>bool</td><td>false</td><td>Skip command availability checks</td></tr>
</table>

<h3>JSON Output</h3>
<table>
<tr><th>Field</th><th>Type</th><th>Description</th></tr>
<tr><td><code>overall_status</code></td><td>string</td><td>"healthy", "degraded", or "unhealthy"</td></tr>
<tr><td><code>service_status</code></td><td>string</td><td>"active" or "inactive"</td></tr>
<tr><td><code>preflight</code></td><td>object</td><td>Prerequisite check results</td></tr>
</table>

<h3>Example</h3>
<pre><code>$ kiro-cli health --config /etc/kiro/kiro.yaml
{
  "overall_status": "healthy",
  "service_status": "active",
  "preflight": {
    "os_compatible": true,
    "root_access": true,
    "commands_available": {
      "nft": true,
      "nginx": true,
      "systemctl": true
    }
  },
  "version": "1.0.0"
}</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — health check results displayed (regardless of overall_status)</td></tr>
<tr><td>1</td><td>Error — unable to perform health check</td></tr>
<tr><td>2</td><td>Missing required parameter</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/preflight">kiro-cli preflight</a> — prerequisite checks only</li>
<li><a href="/docs/en/cli/status">kiro-cli status</a> — runtime status</li>
</ul>
`

var enCLIPreflight template.HTML = `<h2>kiro-cli preflight</h2>
<p>Checks deployment prerequisites: OS compatibility, root access, and command availability.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli preflight --config &lt;path&gt; [--os-release &lt;path&gt;] [--preflight-writable-root &lt;path&gt;] [--skip-command-checks]</code></pre>

<h3>Parameters</h3>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--config</code></td><td>Yes</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Path to configuration file</td></tr>
<tr><td><code>--os-release</code></td><td>No</td><td>string</td><td>/etc/os-release</td><td>Path to os-release file</td></tr>
<tr><td><code>--preflight-writable-root</code></td><td>No</td><td>string</td><td>/</td><td>Root directory for writable checks</td></tr>
<tr><td><code>--skip-command-checks</code></td><td>No</td><td>bool</td><td>false</td><td>Skip command availability checks</td></tr>
</table>

<h3>JSON Output</h3>
<table>
<tr><th>Field</th><th>Type</th><th>Description</th></tr>
<tr><td><code>os_compatible</code></td><td>bool</td><td>Whether OS is supported (Ubuntu 22.04/24.04)</td></tr>
<tr><td><code>root_access</code></td><td>bool</td><td>Running as UID 0</td></tr>
<tr><td><code>commands_available</code></td><td>object</td><td>Status of each command (nft, nginx, systemctl)</td></tr>
</table>

<h3>Example</h3>
<pre><code>$ sudo kiro-cli preflight --config /etc/kiro/kiro.yaml
{
  "os_compatible": true,
  "os_id": "ubuntu",
  "os_version": "22.04",
  "root_access": true,
  "commands_available": {
    "nft": true,
    "nginx": true,
    "systemctl": true
  },
  "writable_paths": {
    "/usr/local/bin": true,
    "/etc/kiro": true
  }
}</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — preflight results displayed</td></tr>
<tr><td>1</td><td>Error — unable to perform checks</td></tr>
<tr><td>2</td><td>Missing required parameter</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/health">kiro-cli health</a> — health check includes preflight</li>
<li><a href="/docs/en/cli/install">kiro-cli install</a> — install after preflight passes</li>
</ul>
`

var enCLIMode template.HTML = `<h2>kiro-cli mode</h2>
<p>Show or change the Kiro WAF operating mode. Two modes: <code>server</code> (WAF proxy only) and <code>full</code> (WAF proxy + XDP filter).</p>

<h3>Syntax</h3>
<pre><code>kiro-cli mode show
kiro-cli mode set --mode &lt;value&gt;</code></pre>

<h3>Parameters</h3>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--mode</code></td><td>Yes (for set)</td><td>string</td><td>—</td><td>New mode: "server" or "full"</td></tr>
</table>

<h3>Valid Values</h3>
<table>
<tr><th>Value</th><th>Description</th></tr>
<tr><td><code>server</code></td><td>WAF reverse proxy only (Golang HTTP), no XDP</td></tr>
<tr><td><code>full</code></td><td>WAF reverse proxy + XDP/eBPF packet filter</td></tr>
</table>

<h3>Example</h3>
<pre><code># Show current mode
$ kiro-cli mode show
server

# Switch to full mode
$ kiro-cli mode set --mode full
Mode changed to: full

# Invalid value
$ kiro-cli mode set --mode turbo
Error: invalid mode "turbo", must be "server" or "full"</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — mode displayed or changed</td></tr>
<tr><td>1</td><td>Error — invalid --mode value (not "server" or "full")</td></tr>
<tr><td>2</td><td>Missing sub-command (show/set) or missing --mode for set</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/status">kiro-cli status</a> — view mode in overall status</li>
</ul>
`

var enCLIInstall template.HTML = `<h2>kiro-cli install</h2>
<p>Manage Kiro WAF client installation on the server. Includes three sub-commands: plan, stage-lab, and apply-lab.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli install plan [--config &lt;path&gt;]
kiro-cli install stage-lab --install-root &lt;path&gt; [--config &lt;path&gt;]
kiro-cli install apply-lab --ack KIRO_LAB_INSTALL_APPLY [--install-root &lt;path&gt;]</code></pre>

<h3>Sub-Commands</h3>

<h4>install plan</h4>
<p>Display the installation plan as JSON without making any changes.</p>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--config</code></td><td>No</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Path to configuration file</td></tr>
</table>

<h4>install stage-lab</h4>
<p>Stage the installation into a specified directory (safe dry-run).</p>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--install-root</code></td><td>Yes</td><td>string</td><td>—</td><td>Target directory for staging</td></tr>
<tr><td><code>--config</code></td><td>No</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Path to configuration file</td></tr>
</table>

<h4>install apply-lab</h4>
<p>Apply the actual installation. Requires confirmation and root privileges.</p>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--ack</code></td><td>Yes</td><td>string</td><td>—</td><td>Must be "KIRO_LAB_INSTALL_APPLY" to confirm</td></tr>
<tr><td><code>--install-root</code></td><td>No</td><td>string</td><td>/</td><td>Installation root directory</td></tr>
</table>

<h3>Example</h3>
<pre><code># View installation plan
$ kiro-cli install plan
{
  "binary_path": "/usr/local/bin/kiro-client-waf",
  "config_dir": "/etc/kiro",
  "service_name": "kiro-client-waf",
  "mode": "full"
}

# Stage into test directory
$ kiro-cli install stage-lab --install-root /tmp/kiro-test

# Apply installation (requires root)
$ sudo kiro-cli install apply-lab --ack KIRO_LAB_INSTALL_APPLY

# Error: wrong ack value
$ sudo kiro-cli install apply-lab --ack wrong-value
Error: --ack must be "KIRO_LAB_INSTALL_APPLY"

# Error: not root
$ kiro-cli install apply-lab --ack KIRO_LAB_INSTALL_APPLY
Error: install apply-lab requires root privileges (UID 0)</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success</td></tr>
<tr><td>1</td><td>Error — wrong ack value, not root, or installation failed</td></tr>
<tr><td>2</td><td>Missing required parameter</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/preflight">kiro-cli preflight</a> — check before installing</li>
<li><a href="/docs/en/cli/update">kiro-cli update</a> — update after installation</li>
</ul>
`

var enCLIUpdate template.HTML = `<h2>kiro-cli update</h2>
<p>Manage OTA updates for the Kiro WAF client. Includes three sub-commands: check, apply, and rollback.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli update check --master-url &lt;url&gt; [--component &lt;name&gt;] [--channel &lt;name&gt;]
kiro-cli update apply --master-url &lt;url&gt; --binary-path &lt;path&gt; --service &lt;name&gt; [--component &lt;name&gt;] [--channel &lt;name&gt;]
kiro-cli update rollback --binary-path &lt;path&gt; --service &lt;name&gt;</code></pre>

<h3>Sub-Commands</h3>

<h4>update check</h4>
<p>Check for new updates from the Master Server.</p>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--master-url</code></td><td>Yes</td><td>string</td><td>—</td><td>Master Server URL</td></tr>
<tr><td><code>--component</code></td><td>No</td><td>string</td><td>kiro-client-waf</td><td>Component name to check</td></tr>
<tr><td><code>--channel</code></td><td>No</td><td>string</td><td>stable</td><td>Update channel (stable, beta)</td></tr>
</table>

<h4>update apply</h4>
<p>Download and apply an update with SHA-256 verification. Auto-rollback if health check fails within 30 seconds.</p>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--master-url</code></td><td>Yes</td><td>string</td><td>—</td><td>Master Server URL</td></tr>
<tr><td><code>--binary-path</code></td><td>Yes</td><td>string</td><td>—</td><td>Path to current binary</td></tr>
<tr><td><code>--service</code></td><td>Yes</td><td>string</td><td>—</td><td>Systemd service name</td></tr>
<tr><td><code>--component</code></td><td>No</td><td>string</td><td>kiro-client-waf</td><td>Component name</td></tr>
<tr><td><code>--channel</code></td><td>No</td><td>string</td><td>stable</td><td>Update channel</td></tr>
</table>

<h4>update rollback</h4>
<p>Restore the previous version from the backup file (.bak).</p>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--binary-path</code></td><td>Yes</td><td>string</td><td>—</td><td>Path to current binary</td></tr>
<tr><td><code>--service</code></td><td>Yes</td><td>string</td><td>—</td><td>Systemd service name</td></tr>
</table>

<h3>Example</h3>
<pre><code># Check for updates
$ kiro-cli update check --master-url https://firewall.vpsgen.com
{
  "update_available": true,
  "current_version": "1.0.0",
  "new_version": "1.1.0",
  "artifact_url": "https://firewall.vpsgen.com/releases/kiro-client-waf-1.1.0",
  "sha256": "abc123..."
}

# Apply update
$ sudo kiro-cli update apply \
  --master-url https://firewall.vpsgen.com \
  --binary-path /usr/local/bin/kiro-client-waf \
  --service kiro-client-waf

# Rollback to previous version
$ sudo kiro-cli update rollback \
  --binary-path /usr/local/bin/kiro-client-waf \
  --service kiro-client-waf</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success</td></tr>
<tr><td>1</td><td>Error — SHA-256 mismatch, health check failed (auto-rolled back), or rollback failed</td></tr>
<tr><td>2</td><td>Missing required parameter (--master-url, --binary-path, --service)</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/version">kiro-cli version</a> — check current version</li>
<li><a href="/docs/en/cli/health">kiro-cli health</a> — health check after update</li>
</ul>
`

var enCLIIncident template.HTML = `<h2>kiro-cli incident report</h2>
<p>Generate a security incident report in both JSON and Markdown formats. Saves to the specified output directory.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli incident report --type &lt;type&gt; --severity &lt;level&gt; --status &lt;status&gt; --summary &lt;text&gt; [--output-dir &lt;path&gt;]</code></pre>

<h3>Parameters</h3>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--type</code></td><td>Yes</td><td>string</td><td>—</td><td>Incident type (see table below)</td></tr>
<tr><td><code>--severity</code></td><td>Yes</td><td>string</td><td>—</td><td>Severity: critical, high, medium, low</td></tr>
<tr><td><code>--status</code></td><td>Yes</td><td>string</td><td>—</td><td>Status: open, investigating, resolved, closed</td></tr>
<tr><td><code>--summary</code></td><td>Yes</td><td>string</td><td>—</td><td>Brief description of the incident</td></tr>
<tr><td><code>--output-dir</code></td><td>No</td><td>string</td><td>./incidents</td><td>Directory to save reports</td></tr>
</table>

<h4>Valid --type Values</h4>
<table>
<tr><th>Value</th><th>Description</th></tr>
<tr><td><code>attack</code></td><td>DDoS or brute-force attack</td></tr>
<tr><td><code>lost_ssh</code></td><td>Lost SSH connection to server</td></tr>
<tr><td><code>update_failed</code></td><td>OTA update failed</td></tr>
<tr><td><code>origin_ip_leaked</code></td><td>Origin IP exposed</td></tr>
<tr><td><code>license_rebind</code></td><td>License rebound to different machine</td></tr>
<tr><td><code>runtime_security</code></td><td>Runtime security error</td></tr>
<tr><td><code>other</code></td><td>Other incident</td></tr>
</table>

<h3>Example</h3>
<pre><code>$ kiro-cli incident report \
  --type attack \
  --severity high \
  --status investigating \
  --summary "DDoS attack detected, 50k rps from multiple IPs" \
  --output-dir /var/log/kiro/incidents

Created: /var/log/kiro/incidents/incident-2024-01-15T10-30-00Z.json
Created: /var/log/kiro/incidents/incident-2024-01-15T10-30-00Z.md</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — report generated</td></tr>
<tr><td>1</td><td>Error — unable to write file or invalid parameter value</td></tr>
<tr><td>2</td><td>Missing required parameter (--type, --severity, --status, --summary)</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/pilot">kiro-cli pilot report</a> — aggregate incidents into pilot report</li>
<li><a href="/docs/en/cli/report">kiro-cli report</a> — comprehensive system report</li>
</ul>
`

var enCLIPilot template.HTML = `<h2>kiro-cli pilot report</h2>
<p>Generate a pilot go/no-go report by aggregating evidence from health checks, benchmarks, and incident reports.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli pilot report --server-count &lt;n&gt; --started-at &lt;RFC3339&gt; --ended-at &lt;RFC3339&gt; [--health-file &lt;path&gt;] [--benchmark-file &lt;path&gt;] [--incident-dir &lt;path&gt;] [--output-dir &lt;path&gt;]</code></pre>

<h3>Parameters</h3>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--server-count</code></td><td>Yes</td><td>int</td><td>—</td><td>Number of servers in the pilot</td></tr>
<tr><td><code>--started-at</code></td><td>Yes</td><td>string</td><td>—</td><td>Pilot start time (RFC3339)</td></tr>
<tr><td><code>--ended-at</code></td><td>Yes</td><td>string</td><td>—</td><td>Pilot end time (RFC3339)</td></tr>
<tr><td><code>--health-file</code></td><td>No</td><td>string</td><td>—</td><td>JSON file with health check results</td></tr>
<tr><td><code>--benchmark-file</code></td><td>No</td><td>string</td><td>—</td><td>JSON file with benchmark results</td></tr>
<tr><td><code>--incident-dir</code></td><td>No</td><td>string</td><td>—</td><td>Directory containing incident reports</td></tr>
<tr><td><code>--output-dir</code></td><td>No</td><td>string</td><td>./pilot</td><td>Directory to save pilot report</td></tr>
</table>

<h3>Output</h3>
<p>Generates JSON and Markdown reports containing:</p>
<ul>
<li>Go/no-go decision based on evidence</li>
<li>Health status summary</li>
<li>Benchmark results (if provided)</li>
<li>List of incidents during pilot period</li>
<li>Pilot duration and server count</li>
</ul>

<h3>Example</h3>
<pre><code>$ kiro-cli pilot report \
  --server-count 3 \
  --started-at "2024-01-01T00:00:00Z" \
  --ended-at "2024-01-15T00:00:00Z" \
  --health-file /tmp/health.json \
  --benchmark-file /tmp/bench.json \
  --incident-dir /var/log/kiro/incidents \
  --output-dir /tmp/pilot-report

{
  "decision": "go",
  "server_count": 3,
  "duration_days": 14,
  "health_summary": {"healthy": 3, "degraded": 0, "unhealthy": 0},
  "incidents_total": 1,
  "critical_incidents": 0
}</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — pilot report generated</td></tr>
<tr><td>1</td><td>Error — unable to read evidence files or write output</td></tr>
<tr><td>2</td><td>Missing required parameter (--server-count, --started-at, --ended-at)</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/incident">kiro-cli incident report</a> — create incident reports as input</li>
<li><a href="/docs/en/cli/health">kiro-cli health</a> — create health file as input</li>
</ul>
`

var enCLIReport template.HTML = `<h2>kiro-cli report</h2>
<p>Generate a comprehensive system report including version information, runtime configuration, and component status.</p>

<h3>Syntax</h3>
<pre><code>kiro-cli report --config &lt;path&gt;</code></pre>

<h3>Parameters</h3>
<table>
<tr><th>Parameter</th><th>Required</th><th>Type</th><th>Default</th><th>Description</th></tr>
<tr><td><code>--config</code></td><td>Yes</td><td>string</td><td>/etc/kiro/kiro.yaml</td><td>Path to configuration file</td></tr>
</table>

<h3>JSON Output</h3>
<table>
<tr><th>Field</th><th>Type</th><th>Description</th></tr>
<tr><td><code>version</code></td><td>string</td><td>Kiro WAF version</td></tr>
<tr><td><code>go_version</code></td><td>string</td><td>Go runtime version</td></tr>
<tr><td><code>os</code></td><td>string</td><td>Operating system (e.g., linux)</td></tr>
<tr><td><code>arch</code></td><td>string</td><td>CPU architecture (e.g., amd64)</td></tr>
<tr><td><code>cpu_cores</code></td><td>int</td><td>Number of CPU cores</td></tr>
<tr><td><code>memory_mb</code></td><td>int</td><td>Memory usage (MB)</td></tr>
<tr><td><code>goroutines</code></td><td>int</td><td>Number of running goroutines</td></tr>
<tr><td><code>mode</code></td><td>string</td><td>Operating mode</td></tr>
<tr><td><code>uptime</code></td><td>string</td><td>Service uptime</td></tr>
</table>

<h3>Example</h3>
<pre><code>$ kiro-cli report --config /etc/kiro/kiro.yaml
{
  "version": "1.0.0",
  "go_version": "go1.22.0",
  "os": "linux",
  "arch": "amd64",
  "cpu_cores": 4,
  "memory_mb": 128,
  "goroutines": 42,
  "mode": "full",
  "uptime": "72h15m",
  "config": {
    "license_key": "***masked***",
    "master_url": "https://firewall.vpsgen.com",
    "sites_count": 2
  }
}</code></pre>

<h3>Exit Codes</h3>
<table>
<tr><th>Code</th><th>Meaning</th></tr>
<tr><td>0</td><td>Success — report displayed</td></tr>
<tr><td>1</td><td>Error — unable to read config or gather information</td></tr>
<tr><td>2</td><td>Missing required --config parameter</td></tr>
</table>

<h3>Related</h3>
<ul>
<li><a href="/docs/en/cli/status">kiro-cli status</a> — concise runtime status</li>
<li><a href="/docs/en/cli/health">kiro-cli health</a> — health check</li>
</ul>
`
