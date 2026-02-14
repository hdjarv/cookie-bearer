# cookie-bearer

**cookie-bearer** is a lightweight Go-based HTTP reverse proxy server that:

- Forwards requests to a target server.
- Extracts an access token from JSON responses at `/login` or `/refresh-token` (configurable) and returns it as a cookie.
- Uses the token from that cookie to authorize further requests to the target server with a _Bearer_ token.
- Clears the cookie with response from `/logout` (configurable).

## Features

- 🛡️ Adds `Authorization: Bearer <token>` header from a specified cookie.
- 🔐 Intercepts login and refresh responses (configurable paths, default `/login` and `/refresh-token`), extracts `accessToken`, and returns it as a cookie.
- 🚀 Streams all other HTTP requests and responses efficiently.
- 📋 Configurable via command-line flags and environment variables.

## VSCode Debugging

> Note: Before you can start debugging, you must supply at least the required [options](#options) for the program in the [.vscode/launch.json](.vscode/launch.json) file.

You can debug or run **cookie-bearer.go** directly in VSCode:

1. Open the Run and Debug panel (Ctrl+Shift+D or ⌘+Shift+D).
2. Select **Debug cookie-bearer.go** from the configuration dropdown.
3. Press **Start Debugging** (F5) to launch the program with breakpoints and full debugging support.

The configuration is defined in [.vscode/launch.json](.vscode/launch.json).

## Usage

### Build

Use the provided `Makefile` to build the binary with embedded version info:

```bash
make
```

### Docker

#### Using the Makefile

You can use the provided `Makefile` to build and clean the Docker image:

```bash
make docker         # Build the Docker image (tags as cookie-bearer:<version>)
make docker-clean   # Remove the Docker image
```

The image is tagged as `cookie-bearer:<version>`, where `<version>` is determined from the current Git tag or commit.

**Version, build date, and git commit information are automatically injected into the Docker image at build time using build arguments.** The Makefile handles this for you, so the resulting image will always have the correct version info embedded.

You can build and run `cookie-bearer` using Docker:

#### Build the Docker image (manual example)

If you are not using the Makefile, you can build the Docker image and inject version info manually:

```bash
docker build \
  --build-arg VERSION=$(git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t cookie-bearer .
```

#### Run the container

```bash
docker run --rm -it \
  -p 8080:8080 \
  cookie-bearer \
  -target=http://localhost:3000 -cookie=Auth
```

Replace the flags as needed for your setup. If your app listens on a different port, adjust the `-p` flag accordingly.

You can also print version info from the container:

```bash
docker run --rm cookie-bearer -version
```

### Run

```bash
./cookie-bearer -target=http://localhost:3000 -cookie=Auth
```

### Options

| Flag                     | Env Variable               | Description                                                              | Default        | Required |
| ------------------------ | -------------------------- | ------------------------------------------------------------------------ | -------------- | -------- |
| `-target`                | `CB_TARGET`                | Target server URL to proxy requests to                                   | _(none)_       | ✅       |
| `-cookie-name`           | `CB_COOKIE_NAME`           | Name of the cookie to read/write the token from/to                       | _(none)_       | ✅       |
| `-cookie-secure`         | `CB_COOKIE_SECURE`         | Set Secure flag on the cookie (`true`/`false` or `1`/`0`)                | false          | ❌       |
| `-cookie-max-age`        | `CB_COOKIE_MAX_AGE`        | Max-Age (in seconds) for the cookie (0 = session cookie)                 | 0              | ❌       |
| `-cookie-same-site`      | `CB_COOKIE_SAME_SITE`      | SameSite setting for the cookie (`strict`, `lax` or `none`)              | strict         | ❌       |
| `-access-token-property` | `CB_ACCESS_TOKEN_PROPERTY` | JSON property to extract access token from login response                | accessToken    | ❌       |
| `-login-path`            | `CB_LOGIN_PATH`            | Path to intercept for login (sets the cookie from JSON response)         | /login         | ❌       |
| `-logout-path`           | `CB_LOGOUT_PATH`           | Path to intercept for logout (removes the authentication cookie)         | /logout        | ❌       |
| `-refresh-path`          | `CB_REFRESH_PATH`          | Path to intercept for token refresh (sets the cookie from JSON response) | /refresh-token | ❌       |
| `-host`                  | `CB_HOST`                  | Host address for the proxy server to listen on                           | 127.0.0.1      | ❌       |
| `-port`                  | `CB_PORT`                  | Port for the proxy server to listen on                                   | 8080           | ❌       |
| `-version`               | (n/a)                      | Show version information and exit                                        | -              | ❌       |

**Precedence:**  
Command-line flags take priority over environment variables, which take priority over the built-in default.
This applies to all options, including `-login-path`/`CB_LOGIN_PATH` and `-logout-path`/`CB_LOGOUT_PATH`.

**Cookie Security:**
The authentication cookie is by default set with the `SameSite=Strict` attribute for improved security. The Secure flag on the authentication cookie is controlled by the `-cookieSecure` flag or `CB_COOKIE_SECURE` environment variable. By default, the Secure flag is not set. Set this to `true` (or `1`) to require HTTPS for cookie transmission.

## Example

```bash
./cookie-bearer -target=http://localhost:4000 -cookie=Auth -port=8081
```

You can also specify a custom property for the access token in the `/login` JSON response (default: `accessToken`):

```bash
./cookie-bearer -target=http://localhost:4000 -cookie=Auth -access-token-property=jwt
```

### `/login` and `/refresh-token` Routes

> **Note:** The responses must have a `Content-Type` of `application/json`. Parameters such as `charset` are supported (e.g., `application/json; charset=utf-8`).

- For a `/login` or `/refresh-token` POST response like `{ "accessToken": "abc123" }`, `cookie-bearer` sets:

  ```
  Set-Cookie: Auth=abc123
  ```

- On subsequent requests, it reads the `Auth` cookie and sends:

  ```
  Authorization: Bearer abc123
  ```

### `/logout` Route

The logout path is configurable via the `-logout-path` flag or `CB_LOGOUT_PATH` environment variable (default: `/logout`).

When you send a request to the configured logout path, `cookie-bearer` will:

- Forward the request to the target server's logout endpoint.
- Return the backend's response.
- **Remove the authentication cookie** from the browser by setting it with an expired date and `Max-Age=0`.

This allows clients to log out by simply calling the configured logout path on the proxy.

**Example:**

```bash
curl -i -X POST http://localhost:8081/logout
# Response will include:
# Set-Cookie: auth=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0; HttpOnly; SameSite=Strict
```

## Version Info

You can print version information using:

```bash
./cookie-bearer -version
# cookie-bearer
#  Version: 1.0.0
#  Build Date: 2025-06-12T18:02:58Z
#  Git Commit: 31a6226
#  Go Version: go1.24.1
```

Or from the Docker image:

```bash
docker run --rm cookie-bearer -version
# cookie-bearer
#  Version: 1.0.0
#  Build Date: 2025-06-12T18:02:58Z
#  Git Commit: 31a6226
#  Go Version: go1.24.1
```

Build info is injected into both the binary and the Docker image via:

```bash
go build -ldflags "-X main.version=1.0.0 -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.gitCommit=$(git rev-parse --short HEAD)"
```

When building the Docker image, these values are passed as build arguments and set automatically by the Makefile.

## License

This software is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

cookie-bearer is a project by Henrik Djärv.
