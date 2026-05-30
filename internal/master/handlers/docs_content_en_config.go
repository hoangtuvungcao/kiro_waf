package handlers

import "html/template"

var enConfigReference template.HTML = `<h2>Configuration Reference</h2>
<p>All configuration is stored in <code>/etc/kiro/kiro.yaml</code>. Below is a complete reference of all available options.</p>

<h3>Top-Level Options</h3>
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
