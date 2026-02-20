# Admin UI

CacheStorm includes a modern, web-based admin interface for monitoring and managing your cache.

## Access

By default, the admin UI is available at:

```
http://localhost:8080
```

## Features

### Dashboard

The dashboard provides a real-time overview of your CacheStorm instance:

- **Total Keys**: Number of keys stored
- **Memory Usage**: Current memory consumption
- **Tags**: Number of tags registered
- **Uptime**: Server uptime

Additional sections:
- **Recent Activity**: Live feed of cache operations
- **Top Tags**: Most used tags by key count

### Keys Browser

Browse, search, and manage all keys in your cache:

- **Search**: Filter keys by pattern or name
- **View**: See key details (type, value, TTL, tags, size)
- **Add**: Create new keys with type selection
- **Delete**: Remove individual keys

Supported key types:
- String
- Hash
- List
- Set

### Tags Management

Manage tag-based cache invalidation:

- **View Tags**: List all tags with key counts
- **View Keys**: See all keys associated with a tag
- **Invalidate**: Delete all keys with a specific tag

### Namespaces

Manage multiple namespaces:

- **View**: List all namespaces with stats
- **Create**: Add new namespaces
- **Delete**: Remove namespaces (except default)

### Cluster View

Monitor and manage cluster nodes:

- **Cluster Status**: Overall cluster health
- **Node List**: All nodes with roles and slots
- **Join Cluster**: Add new nodes to the cluster

### Console

Execute Redis commands directly from the browser:

```redis
SET mykey "Hello"
GET mykey
KEYS *
DBSIZE
INFO
```

### Slow Log

View slow queries for performance analysis:

- Query duration
- Command executed
- Timestamp

## Authentication

### Enable Password Protection

In your configuration:

```yaml
http:
  enabled: true
  port: 8080
  password: "your-secret-password"
```

### Login

When password protection is enabled:

1. Open `http://localhost:8080`
2. Enter your password
3. You'll be redirected to the dashboard

The authentication token is stored in a cookie for subsequent requests.

## Configuration

### Basic Configuration

```yaml
http:
  enabled: true        # Enable/disable admin UI
  port: 8080           # Port for HTTP server
  password: ""         # Optional password protection
```

### Disable Admin UI

```yaml
http:
  enabled: false
```

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+K` | Focus search in Keys |
| `Enter` | Execute command in Console |
| `Esc` | Close modals |

## Browser Support

- Chrome 90+
- Firefox 88+
- Safari 14+
- Edge 90+

## Security Considerations

1. **Password Protection**: Always set a password in production
2. **HTTPS**: Use a reverse proxy with HTTPS in production
3. **Network Access**: Restrict access via firewall rules
4. **Rate Limiting**: Consider rate limiting at the reverse proxy level

## Reverse Proxy Setup

### Nginx

```nginx
server {
    listen 443 ssl;
    server_name cachestorm.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Caddy

```
cachestorm.example.com {
    reverse_proxy localhost:8080
}
```

### Traefik

```yaml
http:
  routers:
    cachestorm:
      rule: "Host(`cachestorm.example.com`)"
      service: cachestorm
      tls: {}
  services:
    cachestorm:
      loadBalancer:
        servers:
          - url: "http://localhost:8080"
```

## Screenshots

### Login Screen
```
┌─────────────────────────────────────────┐
│          ⚡ CacheStorm Admin            │
│                                         │
│         Enter password to continue      │
│                                         │
│         ┌─────────────────────┐         │
│         │ ••••••••••          │         │
│         └─────────────────────┘         │
│                                         │
│         [      Sign In      ]          │
│                                         │
└─────────────────────────────────────────┘
```

### Dashboard
```
┌─────────────────────────────────────────────────────────────┐
│  CacheStorm Admin                            ● Connected    │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │
│  │   Keys   │ │  Memory  │ │   Tags   │ │  Uptime  │      │
│  │  12,345  │ │  256 MB  │ │   127    │ │  2d 4h   │      │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘      │
│                                                             │
│  Recent Activity              Top Tags                     │
│  ─────────────────           ─────────                     │
│  ● SET user:1                 user:*  ========  4521      │
│  ● INCR counter               cache:* ======    3212      │
│  ● TAGKEYS session            sess:*  ====      1892      │
│  ● INVALIDATE old                                        │
└─────────────────────────────────────────────────────────────┘
```

### Keys Browser
```
┌─────────────────────────────────────────────────────────────┐
│  Keys                    [Search keys...        ] [Add Key]│
├─────────────────────────────────────────────────────────────┤
│  Key             Type      TTL        Size      Tags       │
│  ────────────────────────────────────────────────────────  │
│  user:1          string    -1         128 B     [user]     │
│  user:2          hash      1h 30m     256 B     [user]     │
│  session:abc     string    30m        64 B      [session]  │
│  cache:product   string    -1         1.2 KB    [cache]    │
│                                                             │
│  Showing 4 of 12,345 keys                                  │
└─────────────────────────────────────────────────────────────┘
```

### Console
```
┌─────────────────────────────────────────────────────────────┐
│  Console                                                    │
├─────────────────────────────────────────────────────────────┤
│  > SET mykey "Hello CacheStorm"                            │
│  OK                                                         │
│  > GET mykey                                                │
│  "Hello CacheStorm"                                        │
│  > DBSIZE                                                   │
│  12345                                                      │
│  >                                                          │
│  ─────────────────────────────────────────────────────────  │
│  [Enter command...                          ] [Execute]    │
└─────────────────────────────────────────────────────────────┘
```

## Troubleshooting

### UI Not Loading

1. Check if HTTP server is enabled
2. Verify the port is not blocked
3. Check browser console for errors

### Authentication Issues

1. Verify password in configuration
2. Clear browser cookies
3. Check server logs for auth errors

### Slow Performance

1. Check key count - large datasets may slow down key listing
2. Use pattern filtering to reduce data
3. Consider pagination for large datasets
