# Security Policy

Report suspected vulnerabilities to:

```text
security@example.com
```

Include the affected version, impact, minimal reproduction steps, and redacted
logs or screenshots. Do not send request bodies, cookies, authorization headers,
license keys, provider private keys, or customer secrets unless explicitly
requested through a secure channel.

Target response times:

| Severity | Examples | Target response |
| --- | --- | --- |
| Critical | RCE, private key leak, update signature bypass | 24 hours |
| High | Privilege escalation, auth bypass, serious data exposure | 2 business days |
| Medium | Local DoS, limited info leak, conditional policy bypass | 5 business days |
| Low | Hardening issue, minor disclosure | 10 business days |

Please avoid public disclosure of exploit details until a patch or mitigation is
available.
