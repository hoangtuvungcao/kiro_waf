# Domain and Backend Mapping

The design supports multiple deployment shapes.

## One Domain, Multiple Backends

```yaml
backend_pools:
  - id: main_app_pool
    load_balancing: round_robin
    upstreams:
      - id: app_1
        url: http://127.0.0.1:3000
      - id: app_2
        url: http://127.0.0.1:3001

sites:
  - id: main_site
    domains:
      - example.com
    default_backend_pool: main_app_pool
```

## Multiple Domains, One Backend

```yaml
backend_pools:
  - id: shared_app
    upstreams:
      - id: app
        url: http://127.0.0.1:3000

sites:
  - id: brand_a
    domains: [a.example.com]
    default_backend_pool: shared_app
  - id: brand_b
    domains: [b.example.com]
    default_backend_pool: shared_app
```

## One Domain, Path-Based Backends

```yaml
sites:
  - id: main_site
    domains: [example.com, www.example.com]
    default_backend_pool: frontend_pool
    routes:
      - path: /api/
        backend_pool: api_pool
      - path: /login
        backend_pool: api_pool
        rpm_per_ip: 10
```

