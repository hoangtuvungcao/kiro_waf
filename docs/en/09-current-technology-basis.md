# Current Technology Basis

This document records the main technology assumptions.

## Ubuntu 22.04 LTS

Ubuntu 22.04 LTS is still a reasonable target for the current product phase. It
has standard security maintenance until May 2027, with extended options through
Ubuntu Pro/ESM. The product roadmap should also prepare Ubuntu 24.04 LTS support.

Reference: https://ubuntu.com/about/release-cycle

## Cloudflare Free

Cloudflare DDoS Protection is available on all plans, but Free-plan use must be
positioned carefully:

- It protects traffic that goes through Cloudflare.
- The origin must lock 80/443 to Cloudflare IP ranges.
- Both IPv4 and IPv6 ranges must be maintained.
- It does not protect SSH, game ports, or arbitrary TCP/UDP services that do not
  pass through Cloudflare.

References:

- https://developers.cloudflare.com/ddos-protection/
- https://www.cloudflare.com/ips/

## Open-source WAF

Coraza and OWASP CRS are suitable open-source choices for local WAF protection.

References:

- https://owasp.org/www-project-coraza-web-application-firewall/
- https://owasp.org/www-project-modsecurity-core-rule-set/

## Security governance

NIST CSF 2.0 is a useful governance model, and OWASP ASVS is useful for web
application security verification requirements.

References:

- https://www.nist.gov/publications/nist-cybersecurity-framework-csf-20
- https://owasp.org/www-project-application-security-verification-standard/

