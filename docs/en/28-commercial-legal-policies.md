# Commercial And Legal Policies

This document is a commercial-gate policy draft. Before broad sales, the provider
should review it with counsel for the countries/regions where the service is
sold.

## Service Plans

| Plan | Mode | Server/domain | Main features | Support | Updates | Rebind |
| --- | --- | --- | --- | --- | --- | --- |
| Community | `server` | 1 server, no SLA | nftables dry-run, local CLI, docs | Community/best effort | Manual/stable | No commitment |
| School/SMB | `server`, `full` | 1-3 servers, 1-10 domains | Nginx generator, WAF, bot, Cloudflare origin lock, signed updates | Business hours | Stable/security | 1/month after verification |
| Professional | `server`, `full` | 1-10 servers, multiple domains | Runtime security, incident report, priority update, route policy | Priority business hours | Stable/security/optional beta | Faster, with audit reason |
| Enterprise-lite | `server`, `full` | Contract-defined | Audit log, staged rollout, periodic reports, custom warranty workflow | Contract-defined | Stable/security/staged | Contract-defined |

## Realistic SLA/SLO

Do not promise protection from every DDoS attack. Commit only to what the
provider controls:

- License activation when the provider is online: target under 5 minutes.
- Security update delivery: 24-72 hours depending on severity and impact.
- Config/update rollback in lab: target under 30 seconds after command start.
- Local support bundle: target under 60 seconds for normal configs.
- Incident report template: created locally in under 60 seconds.

Reference support response targets:

| Plan | Target response |
| --- | --- |
| Community | No SLA |
| School/SMB | 1-2 business days |
| Professional | 4-8 business hours |
| Enterprise-lite | Contract-defined |

## Product Limits

- This does not replace upstream bandwidth DDoS protection.
- This does not guarantee blocking every botnet, network-layer attack, or attack class.
- Cloudflare Free protects only traffic that actually passes through Cloudflare with correct DNS/proxy config.
- Flexible HTTP does not encrypt the Cloudflare-to-origin hop.
- WAF does not fix application logic bugs, auth bugs, or business workflow flaws.
- Effectiveness depends on server resources, kernel, configuration, traffic profile, and attack type.
- Local/lab benchmarks must not be used as public claims without an isolated lab and clear measurement method.

## Privacy

By default:

- Telemetry is off.
- Request bodies are not sent.
- Cookies are not sent.
- Authorization headers are not sent.
- Tokens/passwords/license keys are not sent.
- Support bundles redact secrets before writing files.

Health/support data may include:

- Version, mode, plan.
- Module status.
- Aggregate CPU/RAM/load/counters.
- Health/preflight status.
- Redacted runtime alerts.
- Operator-entered incident timeline.

Recommended retention:

| Data | Default retention |
| --- | --- |
| Health report | 180 days |
| Incident report | 365 days |
| Support bundle | Delete after ticket close or by contract |
| License audit | As required for accounting/legal obligations |

Customers may request deletion of support bundles and incident attachments that
are no longer required for warranty/support, unless legal obligations require
retention.

## Data Processing Note

If telemetry or support bundles are sent to the provider:

- The provider processes data only for support, warranty, updates, and incident investigation.
- The provider does not sell request/client data.
- Request bodies/cookies/tokens are not required for normal support.
- Sensitive data should be redacted before it is sent through support channels.
- Unredacted data for severe incidents requires separate customer confirmation.

## Security Vulnerability Policy

Report channel:

```text
security@example.com
```

Reports should include:

- Affected version.
- Vulnerability description.
- Minimal reproduction steps.
- Practical impact.
- Redacted logs/screenshots.

Severity:

| Severity | Example | Target response |
| --- | --- | --- |
| Critical | RCE, private key leak, update signature bypass | 24 hours |
| High | Privilege escalation, auth bypass, serious data exposure | 2 business days |
| Medium | Local DoS, limited info leak, conditional policy bypass | 5 business days |
| Low | Hardening issue, minor disclosure | 10 business days |

The provider should not publish exploit details before a patch or reasonable
mitigation is available.

## Acceptable Use Policy

Do not use `kiro_waf`, benchmarks, or related tooling to:

- Attack systems you do not own or are not authorized to test.
- Generate or coordinate public DDoS traffic.
- Bypass third-party rate limits.
- Collect data without authorization.
- Hide malware/phishing/spam activity.

The provider may deny support or terminate service when a customer violates the
AUP.

## Terms Of Service Draft

- Customers are responsible for providing accurate admin IP/domain/backend/license information.
- Customers must have lawful control over protected servers/domains.
- The provider is not responsible for downtime caused by unsupported manual config changes.
- Real apply operations require preflight, dry-run, rollback, and operator confirmation.
- The provider does not promise complete removal of DDoS risk or origin application vulnerabilities.

## Refund And Warranty

Recommended minimum policy:

- Refund during onboarding if installation fails because of a product defect and support cannot resolve it.
- No refund for servers that do not meet published system requirements.
- No warranty when customers manually change firewall/proxy state outside runbooks and lose access.
- Provider-caused update failures require rollback/mitigation and an incident note.
- License rebind requires an audit reason; abusive rebind patterns may be denied.

## Commercial-Ready Checklist

- Public service plans.
- Public SLA/SLO that does not overpromise.
- Public privacy statement.
- Public security report contact.
- Reviewed AUP/ToS draft.
- DDoS limitation disclaimer in sales material.
- Clear refund/warranty policy.
