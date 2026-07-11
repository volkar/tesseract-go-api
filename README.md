# Tesseract
### Go API Boilerplate

A robust, production-ready backend template designed for modern SPAs (Nuxt, Next.js). This boilerplate focuses on extreme security, high-load performance, and clean code principles. Built for scale from day one.

### Tech Stack

-   **Language**: Go (Golang) 1.22+
-   **Database**: PostgreSQL + SQLC (Type-safe SQL)
-   **Cache**: Redis (Cache-aside pattern)
-   **Auth**: PASETO (Platform-Agnostic Security Tokens)
-   **Migrations**: goose (or your preferred tool)

### Key Features & Strengths

#### 🛡️ Security First & Resilience

- **Fully Stateless PASETO Authentication**: Uses PASETO (V4) instead of JWT to eliminate common header/algorithm vulnerabilities. Optimized Access & Refresh token flow.
- **Concurrent Refresh Token Protection**: Implements a strict "Grace Period" mechanism to handle network retries elegantly, preventing false-positive token theft detections and race conditions on unstable mobile connections.
- **Opaque Secure Cursors (AES-GCM)**: Pagination cursors are cryptographically sealed using AES-GCM encryption. This completely hides underlying database logic (like UUIDs or timestamps) from clients, preventing IDOR, cursor tampering, and malicious data scraping.
- **Advanced CSRF Protection**: Combined `SameSite=Lax` cookie policy with custom header validation (`X-Requested-With`) and strict `Origin/Referer` checks.
- **Smart Session Management**: Track active sessions with metadata (IP, Browser, OS). Includes "Logout from other devices" functionality by invalidating specific refresh token families.
- **Payload & DDoS Protection**: Global body size limits (preventing "Terabyte" attacks) and strict timeouts for all network operations.

#### ⚡ Extreme Performance & High-Load Caching

- **Cache Stampede (Thundering Herd) Protection**: Utilizes Go's `singleflight` pattern. If 1,000 users request the same missing cache key simultaneously, only one database query is executed, instantly sharing the result across all goroutines.
- **Entity Caching & Normalized Aliases**: Avoids combinatorial cache bloat (OOM risk) by caching pure entities via MGET and resolving slugs/relations through atomic Lua scripts. Deeply nested permission checks are offloaded to PostgreSQL's GIN indexes, while Redis handles ultra-fast entity delivery.
- **Zero-Allocation ETag Middleware**: Built-in hashing of GET responses to support HTTP 304 Not Modified. Leverages sync.Pool for byte buffers, completely eliminating Garbage Collection (GC) spikes and heap allocations when hashing massive JSON payloads.
- **Asynchronous, Leak-Free Invalidation**: Cache writes and invalidations are detached from HTTP contexts and executed in background goroutines (Fire-and-Forget). Uses a "Lazy Deletion / TTL" approach for heavy relational data, preventing Redis Orphaned Set Members leaks and keeping invalidation complexity at O(1).
- **O(1) Deep Pagination**: Pure Cursor-based pagination utilizing composite B-Tree indexes in PostgreSQL. Zero usage of the slow $O(N) OFFSET operator, guaranteeing flat response times even at millions of records.

#### 🏗️ Architecture

- **Strict Clean Architecture**: Clear separation between `Handlers` (HTTP), `Services` (Business Logic), and `Repositories` (Data Access).

- **Interface-Driven Design**: Decoupled packages to prevent circular dependencies and enable easy Unit Testing with mocks.

- **Type-Safe SQL**: Thanks to SQLC, your Go code always stays in sync with your schema. No more `interface{}` or reflection-heavy ORM magic.

- **Graceful Shutdown**: Handles OS signals to close DB and Redis connections cleanly without dropping active requests or losing background cache writes.

#### 🚀 Additional Enterprise Features
- **Integrated OAuth 2.0 Flow**: Seamless social authentication (Google, GitHub, etc.) with automated account linking and session creation.

- **Header-based Internationalization (i18n)**: Automated localization of error messages and system notifications based on the `Accept-Language` header.

- **Non-destructive Data Management (Soft Deletes)**: Robust protection against accidental data loss. Users and entities utilize a `deleted_at` pattern, maintaining relational integrity without permanent removal.

- **Granular Access Control (RBAC)**: Built-in support for multiple user roles and flexible pre-route authorization middlewares.


## Data Architecture & Atlas Structure

At the core of the system lies a highly scalable, hybrid data model designed to balance strict relational integrity with schema-less flexibility. The primary entity is the **User**, who acts as the owner of multiple **Albums**. Each Album is safeguarded by a granular, domain-level Access Control layer. Rather than relying on simple public/private toggles, Albums support complex permission matrices—including specific allowed emails (`shared_emails`), ownership validation, direct token access, and active state flags. This ensures that sensitive data is strictly gated, effectively obfuscating the existence of private albums from unauthorized requests (returning a 404 instead of a 403).

The true power of the Album resides in its **Atlas** — a dynamic structural core stored as native PostgreSQL `JSONB`. While the overarching architecture benefits from type-safe SQL, the Atlas serves as a flexible canvas containing an array of customizable "blocks" (e.g., text, galleries, interactive media). This hybrid approach allows modern frontend frameworks (like Nuxt or Next.js) to render deeply nested, complex UI components seamlessly, without requiring continuous backend database migrations for every new block type or feature iteration.

## Authentication endpoints

**GET** `/auth/google/provider`
Redirects to Google OAuth page

**POST** `/auth/refresh`
Exchange refresh token to new refresh and access tokens

**POST** `/auth/logout`
Deletes token cookies and refresh token from database

**POST** `/auth/logout-others`
Deletes all other refresh tokens from database (logout from other devices)

**GET** `/auth/sessions`
Get active refresh tokens for user, identifying the current one

**DELETE** `/auth/sessions/{token_id}`
Delete refresh token

## Authenticated user endpoints

**GET** `/me/info`
Authenticated user info

**GET** `/me/albums/list` (same as `/users/{my_user_slug}/albums`)
Authenticated user list of all albums

**GET** `/me/albums/trashed`
Authenticated user list of trashed albums

**GET** `/me/albums/{album_id}`
Authenticated user any album

## Data endpoints

**GET** `/health`
Simple health check

**GET** `/users/{user_slug}/info`
Get user info by user slug

**GET** `/users/{user_slug}/albums`
Get user list of available albums by user slug

**GET** `/albums/{user_slug}/{album_slug}`
Get the album data from user slug and album slug

**GET** `/albums/{direct_token}`
Get the album data from direct token

## Modify endpoints

**PUT** `/users/{user_id}`
Update user info

**DELETE** `/users/{user_id}`
Delete user

**POST** `/albums`
Create new album

**PUT** `/albums/{album_id}`
Update album

**PUT** `/albums/{album_id}/active`
Update album visibility (is_active flag)

**POST** `/albums/{album_id}/direct`
Generate new direct share token for album

**DELETE** `/albums/{album_id}/direct`
Revoke direct share token for album

**DELETE** `/albums/{album_id}`
Delete album

**POST** `/albums/{album_id}/restore`
Restore deleted album

**DELETE** `/albums/{album_id}/purge`
Purge deleted album

## Admin endpoints

**POST** `/admin/users/{user_id}/restore`
Restore deleted user. Requires admin role

**DELETE** `/admin/users/{user_id}`
Purge user with all albums and tokens. Requires admin role

## Playground endpoints

For development purposes there is `/cmd/api/routed_playground.go` file with `Playground endpoints`. Should be deleted in production

**GET** `/playground/create_admin`
Creates `admin` user with 4 albums (public, private, shared for user and inactive) (`/users/admin/info` and `/users/admin/albums`)

**GET** `/playground/create_user`
Creates `user` user with 4 albums (public, private, shared for admin and inactive) (`/users/user/info` and `/users/user/albums`)

**GET** `/playground/get_admin_cookies`
Get access and refresh tokens for admin

**GET** `/playground/get_user_cookies`
Get access and refresh tokens for user

**GET** `/playground/clear_cookies`
Deletes all token cookies

**GET** `/playground/clear_cache`
Flushes all redis cache data

## Installation

1. Clone the project:

```
git clone https://github.com/volkar/go-api.git
```

2. Go to the project's folder

```
cd go-api-bolierplate
```

3. Copy .env.example to .env

```
cp .env.example .env
```

4. Edit .env

```
Generate required random strings
Fill Postgres credentials
Fill Google OAuth credentials
```

5. Run database migration (with [goose](https://github.com/pressly/goose) for example)

```
goose up
```

6. Run your golang app (via [air](https://github.com/air-verse/air) for example):

```
air
```

7. Open address in curl or Yaak/Insomnium.

```
curl -X GET http://localhost:8000/health
```

8. Now you can use `Playground routes` to create `admin` and `user` and test this API.

## ❤️ Support

Tesseract is completely free and open-source. If it made your backend work a little brighter, you can fuel its future updates via the **Sponsor** button or through my [Support Page](https://support.syntheticsymbiosis.com).

Your contributions go directly towards project maintenance, late-night caffeine, and I will *definitely* not use them to save up for a 1969 Ford Mustang. Promise.

## License

Released under the [MIT License](LICENSE).

## Contact & Contributions

Feel free to reach out to me!

-   Email: sergey@volkar.ru
-   Telegram: @sergeyvolkar

All pull requests are welcome!