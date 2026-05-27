# Domain Và Backend

Thiết kế phải hỗ trợ nhiều mô hình triển khai website, không chỉ một domain trỏ
vào một backend.

## Khái niệm

```text
site
  Nhóm cấu hình bảo vệ cho một hoặc nhiều domain.

domain
  Hostname public như example.com, www.example.com, api.example.com.

backend_pool
  Nhóm backend thật phía sau proxy.

upstream
  Một backend cụ thể, ví dụ http://127.0.0.1:3000.

route
  Rule theo path, ví dụ /login, /api/search, /upload.
```

## Mô hình 1 domain -> nhiều backend

Dùng khi một website cần load balance nhiều instance app.

```yaml
backend_pools:
  - id: main_app_pool
    load_balancing: round_robin
    upstreams:
      - id: app_1
        url: http://127.0.0.1:3000
        weight: 1
      - id: app_2
        url: http://127.0.0.1:3001
        weight: 1

sites:
  - id: main_site
    domains:
      - example.com
    default_backend_pool: main_app_pool
```

## Mô hình nhiều domain -> một backend

Dùng khi nhiều domain cùng chạy một ứng dụng.

```yaml
backend_pools:
  - id: shared_app
    upstreams:
      - id: app
        url: http://127.0.0.1:3000

sites:
  - id: brand_a
    domains:
      - a.example.com
    default_backend_pool: shared_app

  - id: brand_b
    domains:
      - b.example.com
    default_backend_pool: shared_app
```

## Mô hình một domain, nhiều path, nhiều backend

Dùng khi frontend, API, upload, admin nằm ở các service khác nhau.

```yaml
backend_pools:
  - id: frontend_pool
    upstreams:
      - id: frontend
        url: http://127.0.0.1:3000

  - id: api_pool
    upstreams:
      - id: api
        url: http://127.0.0.1:4000

sites:
  - id: main_site
    domains:
      - example.com
      - www.example.com
    default_backend_pool: frontend_pool
    routes:
      - path: /api/
        backend_pool: api_pool
        rpm_per_ip: 60
      - path: /login
        backend_pool: api_pool
        rpm_per_ip: 10
        challenge_score: 50
```

## Mô hình nhiều domain dùng chung policy

Dùng khi khách hàng có nhiều domain cùng mức bảo vệ.

```yaml
policies:
  - id: default_web_policy
    waf: true
    bot: true
    default_rpm_per_ip: 120

sites:
  - id: customer_site
    domains:
      - example.com
      - www.example.com
      - example.net
    default_backend_pool: main_app_pool
    policy: default_web_policy
```

## Nguyên tắc generate proxy config

Agent không nên bắt người vận hành viết Nginx thủ công. Agent đọc `sites` và
`backend_pools`, sau đó generate:

- `upstream` block.
- `server_name`.
- `location` theo route.
- Rate limit theo route.
- Cache theo route.
- WAF/bot setting theo policy.
- Cloudflare real IP setting nếu bật.

