package handlers

import "html/template"

var enTroubleshooting template.HTML = `<h2>Troubleshooting</h2>
<p>Common issues and their resolutions when operating Kiro WAF.</p>

<h3>1. Service Fails to Start: Missing License Key</h3>
<p><strong>Symptom:</strong> Service exits immediately after start with error in journal.</p>
<p><strong>Error:</strong> <code>missing required config: license_key</code></p>
<p><strong>Resolution:</strong> Ensure <code>license_key</code> is set in <code>/etc/kiro/kiro.yaml</code>. The key format is <code>KIRO-XXXX-XXXX</code>.</p>
<pre><code>sudo journalctl -u kiro-client-waf --no-pager -n 20
# Check and fix the config:
sudo nano /etc/kiro/kiro.yaml
sudo systemctl restart kiro-client-waf</code></pre>

<h3>2. Service Fails to Start: Missing Backend URL</h3>
<p><strong>Symptom:</strong> Service exits with error about missing backend configuration.</p>
<p><strong>Error:</strong> <code>missing required config: backend_url</code></p>
<p><strong>Resolution:</strong> Configure at least one site with a backend URL in the <code>website.sites</code> section.</p>
<pre><code>website:
  sites:
    - domains:
        - yourdomain.com
      backend: http://127.0.0.1:3000</code></pre>

<h3>3. 502 Bad Gateway Errors</h3>
<p><strong>Symptom:</strong> Visitors see 502 errors when accessing your site.</p>
<p><strong>Cause:</strong> The backend server is unreachable or not responding within 5 seconds.</p>
<p><strong>Resolution:</strong></p>
<ul>
<li>Verify your backend is running: <code>curl -I http://127.0.0.1:3000</code></li>
<li>Check the backend URL in your config matches the actual listening address</li>
<li>Ensure the backend port is not blocked by local firewall rules</li>
</ul>

<h3>4. 503 Service Unavailable Under Load</h3>
<p><strong>Symptom:</strong> Site returns 503 during traffic spikes.</p>
<p><strong>Cause:</strong> Connection pool exhausted or goroutine limit reached.</p>
<p><strong>Resolution:</strong> This is expected behavior during extreme load to protect the backend. If legitimate traffic is being rejected, consider upgrading your plan or adjusting rate limits in the protection profile.</p>

<h3>5. Installation Script Fails: Unsupported OS</h3>
<p><strong>Symptom:</strong> Install script exits with "unsupported operating system" error.</p>
<p><strong>Resolution:</strong> Kiro WAF supports Ubuntu 20.04+, Debian 11+, CentOS 8+, Rocky 8+, Fedora 36+, and Arch Linux. Check your OS version:</p>
<pre><code>cat /etc/os-release</code></pre>

<h3>6. SHA-256 Checksum Verification Failed</h3>
<p><strong>Symptom:</strong> Installation or OTA update aborts with checksum mismatch.</p>
<p><strong>Cause:</strong> Download was corrupted or intercepted.</p>
<p><strong>Resolution:</strong></p>
<ul>
<li>Retry the installation — network issues may have caused partial download</li>
<li>Verify your network connection is not being intercepted by a proxy</li>
<li>If the issue persists, contact support with the expected vs actual checksum values</li>
</ul>

<h3>7. OTA Update Rolled Back Automatically</h3>
<p><strong>Symptom:</strong> Journal shows "rollback to previous version" after an update.</p>
<p><strong>Cause:</strong> The new binary failed health check within 30 seconds of restart.</p>
<p><strong>Resolution:</strong> This is a safety mechanism. The previous working version is restored automatically. Check the journal for the specific failure reason:</p>
<pre><code>sudo journalctl -u kiro-client-waf --since "30 minutes ago"</code></pre>

<h3>8. Cannot Connect to Management Server</h3>
<p><strong>Symptom:</strong> Heartbeat failures in logs, license validation fails.</p>
<p><strong>Cause:</strong> Network connectivity issue to the management server.</p>
<p><strong>Resolution:</strong></p>
<ul>
<li>Verify outbound HTTPS connectivity: <code>curl -I https://firewall.vpsgen.com</code></li>
<li>Check if a firewall is blocking outbound port 443</li>
<li>Verify DNS resolution is working</li>
<li>The client will automatically retry at the next poll interval</li>
</ul>

<h3>9. High Memory Usage</h3>
<p><strong>Symptom:</strong> Kiro WAF process consuming more memory than expected.</p>
<p><strong>Resolution:</strong></p>
<ul>
<li>Memory usage under 512 MB at 100K rps is normal</li>
<li>Rate-limit entries are cleaned every 120 seconds automatically</li>
<li>Challenge tokens are cleaned every 60 seconds</li>
<li>If memory grows unbounded, restart the service and report the issue</li>
</ul>
<pre><code>sudo systemctl restart kiro-client-waf</code></pre>

<h3>10. XDP Filter Not Loading</h3>
<p><strong>Symptom:</strong> XDP mode enabled but no packet filtering occurring.</p>
<p><strong>Resolution:</strong></p>
<ul>
<li>Verify XDP dependencies are installed: <code>which clang llvm-strip</code></li>
<li>Check kernel version supports XDP (4.8+ for generic, 4.18+ for native)</li>
<li>Verify the network interface supports XDP native mode</li>
<li>Check journal for BPF loading errors</li>
</ul>
<pre><code>sudo journalctl -u kiro-client-waf | grep -i xdp</code></pre>

<h3>11. Legitimate Traffic Being Blocked</h3>
<p><strong>Symptom:</strong> Real users receiving challenge pages or being rate-limited.</p>
<p><strong>Resolution:</strong></p>
<ul>
<li>Switch to a lighter protection profile: set <code>protection.profile: light</code></li>
<li>Add trusted IPs to the admin allow list</li>
<li>Check if <code>auto_attack_mode</code> escalated protection during a false positive</li>
<li>Review rate limit settings for specific routes</li>
</ul>

<h3>12. Configuration File Syntax Error</h3>
<p><strong>Symptom:</strong> Service fails to start after config change.</p>
<p><strong>Resolution:</strong> Validate your YAML syntax:</p>
<ul>
<li>Ensure consistent indentation (2 spaces, no tabs)</li>
<li>Check for missing colons after keys</li>
<li>Verify string values with special characters are quoted</li>
</ul>
<pre><code># Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('/etc/kiro/kiro.yaml'))"</code></pre>
`

var enFAQ template.HTML = `<h2>Frequently Asked Questions</h2>

<h3>What is Kiro WAF?</h3>
<p>Kiro WAF is a high-performance Web Application Firewall that combines XDP/eBPF packet filtering at the kernel level with a Go-based reverse proxy for comprehensive DDoS protection and web security.</p>

<h3>What protection does Kiro WAF provide?</h3>
<ul>
<li><strong>Layer 3/4:</strong> XDP/eBPF packet filtering at 10M packets per second</li>
<li><strong>Layer 7:</strong> HTTP request inspection, rate limiting, bot detection</li>
<li><strong>WAF Rules:</strong> OWASP CRS-based rule engine for SQL injection, XSS, etc.</li>
<li><strong>Bot Protection:</strong> Cookie challenges, JavaScript challenges, Proof-of-Work</li>
</ul>

<h3>How do automatic updates work?</h3>
<p>Kiro WAF checks for updates by polling the management server at a configurable interval (default: every 5 minutes). When an update is available, it downloads the new binary, verifies its SHA-256 checksum, and performs an atomic replacement. If the new version fails health checks within 30 seconds, it automatically rolls back to the previous version.</p>

<h3>Can I disable automatic updates?</h3>
<p>Yes. Set <code>updates.auto_security_updates: false</code> in your configuration file. However, we strongly recommend keeping automatic updates enabled for security patches.</p>

<h3>What happens during a DDoS attack?</h3>
<p>When <code>auto_attack_mode</code> is enabled, Kiro WAF automatically escalates protection levels based on detected traffic patterns. The XDP filter handles volumetric attacks at the kernel level without impacting application performance.</p>

<h3>Does Kiro WAF work with Cloudflare?</h3>
<p>Yes. Set <code>website.cloudflare: true</code> to enable Cloudflare integration. Kiro WAF will restore real visitor IPs from Cloudflare headers and can be configured to only accept traffic from Cloudflare IP ranges.</p>

<h3>What are the protection profiles?</h3>
<ul>
<li><strong>light:</strong> Minimal filtering, suitable for APIs with known clients</li>
<li><strong>balanced:</strong> Standard protection for most websites (recommended)</li>
<li><strong>strict:</strong> Aggressive filtering for high-value targets</li>
<li><strong>lockdown:</strong> Emergency mode — only admin and known clients allowed</li>
</ul>

<h3>How do I check the service status?</h3>
<pre><code>sudo systemctl status kiro-client-waf
sudo journalctl -u kiro-client-waf -f</code></pre>

<h3>How do I update my license key?</h3>
<p>Edit <code>/etc/kiro/kiro.yaml</code>, update the <code>license_key</code> value, and restart the service:</p>
<pre><code>sudo systemctl restart kiro-client-waf</code></pre>

<h3>What ports does Kiro WAF use?</h3>
<ul>
<li><strong>Inbound:</strong> Ports 80 and 443 for web traffic (configurable)</li>
<li><strong>Outbound:</strong> Port 443 (HTTPS) for management server communication</li>
<li><strong>Local:</strong> Proxies to your backend on the configured port</li>
</ul>

<h3>Is my data sent anywhere?</h3>
<p>By default, telemetry is disabled (<code>telemetry.enabled: false</code>). Only heartbeat signals (license validation and basic health status) are sent to the management server. No request content, visitor data, or sensitive information is transmitted.</p>
`
