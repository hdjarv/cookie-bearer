package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	version   = "development"
	buildDate = "unknown"
	gitCommit = "unknown"
)

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getenvIntDefault returns the integer value of the environment variable, or the default if unset or invalid
func getenvIntDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

// getenvBoolDefault returns true if the environment variable is set to "1", "true", or "TRUE"
func getenvBoolDefault(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if v == "1" || v == "true" || v == "TRUE" {
			return true
		}
		if v == "0" || v == "false" || v == "FALSE" {
			return false
		}
	}
	return def
}

func main() {
	showVersion := flag.Bool("version", false, "Show version information")
	targetStr := flag.String("target", getenvDefault("CB_TARGET", ""), "Target server URL to proxy requests to (env: CB_TARGET)")
	cookieName := flag.String("cookie-name", getenvDefault("CB_COOKIE_NAME", ""), "Name of the cookie to read/write the token from/to (env: CB_COOKIE_NAME)")
	cookieSecure := flag.Bool("cookie-secure", getenvBoolDefault("CB_COOKIE_SECURE", false), "Set Secure flag on cookie (env: CB_COOKIE_SECURE)")
	cookieMaxAge := flag.Int("cookie-max-age", getenvIntDefault("CB_COOKIE_MAX_AGE", 0), "Max-Age (in seconds) for the cookie (env: CB_COOKIE_MAX_AGE)")
	cookieSameSite := flag.String("cookie-same-site", getenvDefault("CB_COOKIE_SAME_SITE", "strict"), "SameSite setting for the cookie (env: CB_COOKIE_SAME_SITE)")
	accessTokenProperty := flag.String("access-token-property", getenvDefault("CB_ACCESS_TOKEN_PROPERTY", "accessToken"), "JSON property to extract access token from login response (env: CB_ACCESS_TOKEN_PROPERTY)")
	loginPath := flag.String("login-path", getenvDefault("CB_LOGIN_PATH", "/login"), "Path to intercept for login (env: CB_LOGIN_PATH)")
	logoutPath := flag.String("logout-path", getenvDefault("CB_LOGOUT_PATH", "/logout"), "Path to intercept for logout (env: CB_LOGOUT_PATH)")
	refreshPath := flag.String("refresh-path", getenvDefault("CB_REFRESH_PATH", "/refresh-token"), "Path to intercept for token refresh requests (env: CB_REFRESH_PATH)")
	listenHost := flag.String("host", getenvDefault("CB_HOST", "127.0.0.1"), "Host address for the proxy server to listen on (env: CB_HOST)")
	port := flag.String("port", getenvDefault("CB_PORT", "8080"), "Port for the proxy server to listen on (env: CB_PORT)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cookie-bearer\n Version: %s \n Build Date: %s\n Git Commit: %s\n Go Version: %s\n", version, buildDate, gitCommit, runtime.Version())
		os.Exit(0)
	}

	if *targetStr == "" || *cookieName == "" {
		fmt.Fprintln(os.Stderr, "Both -target and -cookie flags are required.")
		flag.Usage()
		os.Exit(1)
	}

	targetURL, err := url.Parse(*targetStr)
	if err != nil {
		log.Printf("Invalid target URL: %v", err)
		os.Exit(1)
	}
	var sameSite http.SameSite
	switch strings.ToLower(*cookieSameSite) {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "lax":
		sameSite = http.SameSiteLaxMode
	case "none":
		sameSite = http.SameSiteNoneMode
	default:
		fmt.Fprintf(os.Stderr, "Invalid value '%s' for -cookie-same-site. Valid values are: strict, lax, none\n", *cookieSameSite)
		flag.Usage()
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", proxyHandler(targetURL, *cookieName, *accessTokenProperty, *cookieSecure, *cookieMaxAge, sameSite, *loginPath, *logoutPath, *refreshPath))

	listenAddr := *listenHost + ":" + *port
	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Printf("cookie-bearer proxy server version %s (pid: %d) listening on %s and forwarding to %s\n", version, os.Getpid(), listenAddr, *targetStr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %s", err)
		}
	}()
	// Wait for signal
	var signal = <-sigs
	log.Println("Stopping server " + signal.String())

	// Create a deadline to wait for current connections to finish
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Gracefully shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v\n", err)
	}
	log.Println("Server stopped")
}

func proxyHandler(targetURL *url.URL, cookieName string, accessTokenProperty string, cookieSecure bool, cookieMaxAge int,
	cookieSameSite http.SameSite, loginPath string, logoutPath string, refreshPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("→ %s %s", r.Method, r.URL.Path)

		proxyURL := targetURL.ResolveReference(r.URL)

		req, err := http.NewRequest(r.Method, proxyURL.String(), r.Body)
		if err != nil {
			log.Printf("✖ Failed to create request: %v", err)
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()

		if authCookie, err := r.Cookie(cookieName); err == nil && authCookie != nil && authCookie.Value != "" {
			req.Header.Set("Authorization", "Bearer "+authCookie.Value)
			log.Printf("✓ Using %s cookie for Bearer authentication", cookieName)
		}

		log.Printf("→ Proxying to: %s", proxyURL.String())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("✖ Failed to reach backend: %v", err)
			http.Error(w, "Failed to reach backend", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(contentType)
		if (r.URL.Path == loginPath || r.URL.Path == refreshPath) && err == nil && mediaType == "application/json" {
			var jsonBody map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&jsonBody); err == nil {
				if token, ok := jsonBody[accessTokenProperty].(string); ok {
					log.Printf("✓ Extracted %s from %s response", accessTokenProperty, loginPath)
					http.SetCookie(w, &http.Cookie{
						Name:     cookieName,
						Value:    token,
						Path:     "/",
						HttpOnly: true,
						Secure:   cookieSecure,
						SameSite: cookieSameSite,
						MaxAge:   cookieMaxAge,
					})
				} else {
					log.Printf("⚠ Property '%s' not found in %s response", accessTokenProperty, loginPath)
				}
			} else {
				log.Printf("⚠ Failed to parse JSON from %s response: %v", loginPath, err)
			}

			// Copy headers excluding Content-Length
			for key, values := range resp.Header {
				if key == "Content-Length" {
					continue
				}
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.WriteHeader(resp.StatusCode)
			log.Printf("✓ Sent empty response for %s with status %d (%s)", loginPath, resp.StatusCode, http.StatusText(resp.StatusCode))
			return
		}

		// Handle logoutPath: proxy the request, return the result, and remove the cookie
		if r.URL.Path == logoutPath {
			// Stream response from backend
			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			// Remove the cookie by setting MaxAge=0 and Expires in the past
			http.SetCookie(w, &http.Cookie{
				Name:     cookieName,
				Value:    "",
				Path:     "/",
				HttpOnly: true,
				Secure:   cookieSecure,
				SameSite: cookieSameSite,
				MaxAge:   -1,
				Expires:  time.Unix(0, 0),
			})
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			log.Printf("✓ Proxied %s and cleared cookie %s", logoutPath, cookieName)
			return
		}

		// Stream other responses normally and copy all headers including Content-Length
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)

		log.Printf("✓ Proxied %s %s - Status %d (%s) [%v]", r.Method, r.URL.Path, resp.StatusCode, http.StatusText(resp.StatusCode), time.Since(start))
	}
}
