package handlers

import "html/template"

var enConfigReference template.HTML = `<h2>Configuration Reference</h2>
<p>Kiro WAF uses two types of configuration: <strong>environment variables</strong> for the binaries (<code>kiro-master</code>, <code>kiro-client-waf</code>) and <strong>YAML config files</strong> for <code>kiro-cli</code> and advanced features.</p>

<h3>Master Server Environment Variables</h3>
<p>File: <code>/etc/kiro-master/master.env</code></p>
<table>
<tr><th>Variable</th><th>Default</th><th>Required</th><th>Description</th></tr>
<tr><td><code>KIRO_MASTER_ADDR</code></td><td>:8080</td><td>No</td><td>Listen address (host:port)</td></tr>
<tr><td><code>KIRO_MASTER_DB</code></td><td>/var/lib/kiro-master/master.db</td><td>No</td><td>SQLite database path</td></tr>
<tr><td><code>KIRO_MASTER_ADMIN_KEY</code></td><td>—</td><td><strong>Yes</strong></td><td>Admin API key (fatal if empty)</td></tr>
<tr><td><code>KIRO_MASTER_ADMIN_IPS</code></td><td>(empty)</td><td>No</td><td>Comma-separated admin IP allowlist</td></tr>
<tr><td><code>KIRO_MASTER_SESSION_TTL</code></td><td>12h</td><td>No</td><td>Admin session TTL (Go duration format)</td></tr>
</table>

<h3>Client WAF Environment Variables</h3>
<p>File: <code>/etc/kiro/client-waf.env</code></p>
<table>
<tr><th>Variable</th><th>Default</th><th>Required</th><th>Description</th></tr>
<tr><td><code>KIRO_LICENSE_KEY</code></td><td>—</td><td><strong>Yes</strong></td><td>License key (fatal if empty)</td></tr>
<tr><td><code>KIRO_CLIENT_COOKIE_SECRET</code></td><td>—</td><td><strong>Yes</strong></td><td>HMAC cookie secret (fatal if empty)</td></tr>
<tr><td><code>KIRO_BACKEND_URL</code></td><td>—</td><td><strong>Yes</strong></td><td>Backend URL to proxy to (fatal if empty)</td></tr>
<tr><td><code>KIRO_MASTER_URL</code></td><td>—</td><td><strong>Yes</strong></td><td>Master server URL for heartbeat/updates (fatal if empty)</td></tr>
<tr><td><code>KIRO_CLIENT_LISTEN</code></td><td>:8090</td><td>No</td><td>Listen address for WAF proxy</td></tr>
<tr><td><code>KIRO_NODE_ID</code></td><td>hostname</td><td>No</td><td>Node identifier for heartbeat</td></tr>
<tr><td><code>KIRO_POW_DIFFICULTY</code></td><td>4</td><td>No</td><td>Proof-of-Work difficulty (leading zeros)</td></tr>
<tr><td><code>KIRO_HOLD_SECONDS</code></td><td>2</td><td>No</td><td>Hold page duration in seconds</td></tr>
<tr><td><code>KIRO_RPM_PER_IP</code></td><td>120</td><td>No</td><td>Requests per minute per IP (soft threshold)</td></tr>
<tr><td><code>KIRO_SUBNET_RPM</code></td><td>1800</td><td>No</td><td>Requests per minute per /24 subnet</td></tr>
<tr><td><code>KIRO_HARD_BLOCK_AFTER</code></td><td>360</td><td>No</td><td>RPM threshold for hard block</td></tr>
<tr><td><code>KIRO_BLOCK_TTL_SECONDS</code></td><td>900</td><td>No</td><td>Ban duration in seconds (15 min)</td></tr>
<tr><td><code>KIRO_XDP_BLOCKLIST_FILE</code></td><td>/var/lib/kiro/xdp-blocklist.txt</td><td>No</td><td>XDP blocklist file path</td></tr>
<tr><td><code>KIRO_XDP_SYNC_COMMAND</code></td><td>(empty)</td><td>No</td><td>Command to sync XDP blocklist</td></tr>
<tr><td><code>KIRO_HEARTBEAT_SECONDS</code></td><td>60</td><td>No</td><td>Heartbeat interval to master</td></tr>
<tr><td><code>KIRO_UPDATE_SECONDS</code></td><td>300</td><td>No</td><td>Update check interval (5 min)</td></tr>
<tr><td><code>KIRO_ADMIN_IPS</code></td><td>(empty)</td><td>No</td><td>Comma-separated admin IPs (bypass lockdown)</td></tr>
</table>

<h3>YAML Configuration (kiro.yaml)</h3>
<p>The YAML config at <code>/etc/kiro/kiro.yaml</code> is used by <code>kiro-cli</code> commands. Below is a complete reference.</p>

<h4>Top-Level Options</h4>
<table>
<tr><th>Option</th><th>Type</th><th>Default</th><th>Range/Values</th><th>Description</th></tr>
<tr><td><code>mode</code></td><td>string</td><td>full</td><td>server, full</td><td>Operating mode. "server" for firewall only, "full" for firewall + web protection</td></tr>
<tr><td><code>plan</code></td><td>string</td><td>-</td><td>community, school_smb, professional, enterprise_lite</td><td>License plan tier determining available features</td></tr>
<tr><td><code>license_key</code></td><td>string</td><td>-</td><td>Format: KIRO-XXXX-XXXX</td><td>Your Kiro WAF license key for authentication</td></tr>
</table>

<h3>Admin Section</h3>
<table>
<tr><th>Option</th><th>Type</th><th>Default</th><th>Range/Values</th><th>Description</th></tr>
<tr><td><code>admin.allow_ips</code></td><td>[]string</td><td>[]</td><td>CIDR notation</td><td>IP addresses allowed to access admin/SSH</td></tr>
</table>
<p><strong>Example:</strong></p>
<pre><code>admin:
  allow_ips:
    - 203.0.113.10/32
    - 10.0.0.0/8</code></pre>

<h3>Server Section</h3>
<table>
<tr><th>Option</th><th>Type</th><th>Default</th><th>Range/Values</th><th>Description</th></tr>
<tr><td><code>server.interface</code></td><td>string</td><td>eth0</td><td>Network interface name</td><td>Primary network interface for packet filtering</td></tr>
<tr><td><code>server.ssh_port</code></td><td>integer</td><td>22</td><td>1-65535</td><td>SSH port to keep open in firewall rules</td></tr>
</table>

<h3>Website Section</h3>
<table>
<tr><th>Option</th><th>Type</th><th>Default</th><th>Range/Values</th><th>Description</th></tr>
<tr><td><code>website.enabled</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Enable web application protection</td></tr>
<tr><td><code>website.cloudflare</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Enable Cloudflare integration for real IP restoration</td></tr>
<tr><td><code>website.tls_mode</code></td><td>string</td><td>flexible_http</td><td>flexible_http, full_tls, full_strict</td><td>TLS mode between Cloudflare and origin server</td></tr>
</table>

<h3>Website Sites</h3>
<table>
<tr><th>Option</th><th>Type</th><th>Default</th><th>Range/Values</th><th>Description</th></tr>
<tr><td><code>website.sites[].domains</code></td><td>[]string</td><td>-</td><td>Valid domain names</td><td>List of domains this site responds to</td></tr>
<tr><td><code>website.sites[].backend</code></td><td>string</td><td>-</td><td>URL format</td><td>Backend server URL to proxy requests to</td></tr>
<tr><td><code>website.sites[].routes[].path</code></td><td>string</td><td>/</td><td>URL path prefix</td><td>URL path prefix for route matching</td></tr>
<tr><td><code>website.sites[].routes[].backend</code></td><td>string</td><td>-</td><td>URL format</td><td>Override backend for this specific route</td></tr>
<tr><td><code>website.sites[].routes[].protection</code></td><td>string</td><td>balanced</td><td>light, balanced, strict</td><td>Protection level for this route</td></tr>
</table>
<p><strong>Example:</strong></p>
<pre><code>website:
  sites:
    - domains:
        - example.com
        - www.example.com
      backend: http://127.0.0.1:3000
      routes:
        - path: /api/
          backend: http://127.0.0.1:4000
        - path: /login
          protection: strict</code></pre>

<h3>Protection Section</h3>
<table>
<tr><th>Option</th><th>Type</th><th>Default</th><th>Range/Values</th><th>Description</th></tr>
<tr><td><code>protection.profile</code></td><td>string</td><td>balanced</td><td>light, balanced, strict, lockdown</td><td>Overall protection aggressiveness level</td></tr>
<tr><td><code>protection.waf</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Enable Web Application Firewall rules</td></tr>
<tr><td><code>protection.bot</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Enable bot detection and challenge system</td></tr>
<tr><td><code>protection.auto_attack_mode</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Automatically escalate protection during detected attacks</td></tr>
</table>

<h3>Updates Section</h3>
<table>
<tr><th>Option</th><th>Type</th><th>Default</th><th>Range/Values</th><th>Description</th></tr>
<tr><td><code>updates.auto_security_updates</code></td><td>boolean</td><td>true</td><td>true, false</td><td>Enable automatic OTA security updates</td></tr>
<tr><td><code>updates.channel</code></td><td>string</td><td>stable</td><td>stable, beta</td><td>Update channel to follow</td></tr>
</table>

<h3>Telemetry Section</h3>
<table>
<tr><th>Option</th><th>Type</th><th>Default</th><th>Range/Values</th><th>Description</th></tr>
<tr><td><code>telemetry.enabled</code></td><td>boolean</td><td>false</td><td>true, false</td><td>Enable sending health reports to provider</td></tr>
</table>
`
