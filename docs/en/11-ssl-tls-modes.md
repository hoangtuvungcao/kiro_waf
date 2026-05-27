# SSL/TLS Modes

## Summary

When Cloudflare Free is enabled, the easiest setup is to run the origin on HTTP
port 80 and let Cloudflare provide HTTPS at the edge. In `kiro_waf`, this mode is
called `flexible_http`.

This is easy, but not end-to-end encrypted. Production systems with sensitive
data should use `full_strict`.

## Modes

### flexible_http

```text
Browser --HTTPS--> Cloudflare --HTTP:80--> Origin
```

No origin certificate or private key is required.

```yaml
website_protection:
  tls:
    origin_mode: flexible_http
    origin_http_port: 80
    certificate_file: ""
    private_key_file: ""
```

Use for simple school/SMB websites where ease of setup matters and there is no
highly sensitive data.

### full_tls

```text
Browser --HTTPS--> Cloudflare --HTTPS:443--> Origin
```

Requires an origin certificate/key, but does not provide the same strict
certificate validation as `full_strict`.

### full_strict

```text
Browser --HTTPS--> Cloudflare --HTTPS:443 + cert validation--> Origin
```

Recommended for production and sensitive systems.

```yaml
website_protection:
  tls:
    origin_mode: full_strict
    certificate_file: /etc/kiro/certs/example.com.pem
    private_key_file: /etc/kiro/certs/example.com.key
    cloudflare_origin_ca: true
```

## Required validation

The agent should enforce:

- `flexible_http`: cert/key are not required.
- `full_tls`: cert/key must exist.
- `full_strict`: cert/key must exist and should be valid for Cloudflare strict
  origin validation.
- Cloudflare origin lock is required when Cloudflare is enabled.
- Warn when Flexible mode is used with login/admin/sensitive routes.

References:

- https://developers.cloudflare.com/ssl/origin-configuration/ssl-modes/flexible/
- https://developers.cloudflare.com/ssl/get-started/
- https://developers.cloudflare.com/ssl/origin-configuration/origin-ca

