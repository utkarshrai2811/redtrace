package scanner

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// maxScanBody caps how much of a response body the signature checks lowercase
// and scan; the markers they look for appear early or in small responses.
const maxScanBody = 1 << 20

// Passive runs every passive check against a captured exchange and returns the
// findings. It never sends a request.
func Passive(t Target) []Issue {
	req := parseMessage(t.RequestRaw)
	resp := parseMessage(t.ResponseRaw)
	resp.bodyLow = lowerForScan(resp)
	var out []Issue
	for _, check := range passiveChecks {
		out = append(out, check(t, req, resp)...)
	}
	return out
}

// binaryContentTypes are response content types whose bodies cannot contain the
// ASCII signatures the body checks look for; their bodies are not scanned.
var binaryContentTypes = []string{
	"image/", "video/", "audio/", "font/",
	"application/octet-stream", "application/pdf", "application/zip", "application/gzip",
}

// lowerForScan returns a lowercased copy of the response body for the signature
// checks, once per exchange. It returns "" for clearly-binary content types and
// caps the scanned length, so a large image/binary body is not copied on every
// captured exchange.
func lowerForScan(resp message) string {
	ct := strings.ToLower(resp.headers.Get("Content-Type"))
	for _, bin := range binaryContentTypes {
		if strings.HasPrefix(ct, bin) {
			return ""
		}
	}
	b := resp.body
	if len(b) > maxScanBody {
		b = b[:maxScanBody]
	}
	return strings.ToLower(string(b))
}

// newIssue fills the fields common to a passive finding and computes its
// fingerprint from the distinguishing parts.
func (t Target) newIssue(typ, name string, sev Severity, conf Confidence, detail, evidence, remediation string, fpParts ...string) Issue {
	return Issue{
		Type: typ, Name: name, Severity: sev, Confidence: conf,
		Scheme: t.Scheme, Host: t.Host, Port: t.Port, Path: t.Path, Method: t.Method,
		Detail: detail, Evidence: truncate(evidence, 512), Remediation: remediation,
		Origin:      OriginPassive,
		Fingerprint: fingerprint(append([]string{typ}, fpParts...)...),
		RequestRaw:  t.RequestRaw, ResponseRaw: t.ResponseRaw,
	}
}

func isHTML(h http.Header) bool {
	return strings.Contains(strings.ToLower(h.Get("Content-Type")), "text/html")
}

// passiveChecks is the registry of passive analyzers. Each returns zero or more
// findings for the given exchange.
var passiveChecks = []func(t Target, req, resp message) []Issue{
	checkHSTS,
	checkContentTypeOptions,
	checkCSP,
	checkClickjacking,
	checkInsecureCookies,
	checkServerBanner,
	checkCORS,
	checkDirectoryListing,
	checkErrorDisclosure,
	checkPasswordOverHTTP,
}

func checkHSTS(t Target, _, resp message) []Issue {
	if t.Scheme != "https" || resp.headers.Get("Strict-Transport-Security") != "" {
		return nil
	}
	return []Issue{t.newIssue(
		"missing_hsts", "Missing Strict-Transport-Security header", SeverityLow, ConfidenceFirm,
		"The HTTPS response does not set Strict-Transport-Security (HSTS), so a browser may fall back to plaintext HTTP and be exposed to SSL-stripping.",
		"", "Send `Strict-Transport-Security: max-age=31536000; includeSubDomains` on all HTTPS responses.",
		t.Host, t.Path)}
}

func checkContentTypeOptions(t Target, _, resp message) []Issue {
	if strings.EqualFold(resp.headers.Get("X-Content-Type-Options"), "nosniff") {
		return nil
	}
	return []Issue{t.newIssue(
		"missing_xcto", "Missing X-Content-Type-Options header", SeverityInfo, ConfidenceFirm,
		"The response does not set `X-Content-Type-Options: nosniff`, allowing browsers to MIME-sniff the body and potentially treat it as a different content type.",
		resp.headers.Get("X-Content-Type-Options"),
		"Send `X-Content-Type-Options: nosniff` on responses.",
		t.Host, t.Path)}
}

func checkCSP(t Target, _, resp message) []Issue {
	if !isHTML(resp.headers) || resp.headers.Get("Content-Security-Policy") != "" {
		return nil
	}
	return []Issue{t.newIssue(
		"missing_csp", "Missing Content-Security-Policy header", SeverityLow, ConfidenceFirm,
		"The HTML response does not define a Content-Security-Policy, the primary defence-in-depth control against cross-site scripting and data injection.",
		"", "Define a restrictive Content-Security-Policy appropriate to the application.",
		t.Host, t.Path)}
}

func checkClickjacking(t Target, _, resp message) []Issue {
	if !isHTML(resp.headers) {
		return nil
	}
	xfo := resp.headers.Get("X-Frame-Options")
	csp := strings.ToLower(resp.headers.Get("Content-Security-Policy"))
	if xfo != "" || strings.Contains(csp, "frame-ancestors") {
		return nil
	}
	return []Issue{t.newIssue(
		"clickjacking", "Missing clickjacking protection", SeverityLow, ConfidenceFirm,
		"The HTML response sets neither X-Frame-Options nor a CSP frame-ancestors directive, so it can be framed by any site and is exposed to clickjacking.",
		"", "Send `X-Frame-Options: DENY` (or `SAMEORIGIN`) and/or a CSP `frame-ancestors` directive.",
		t.Host, t.Path)}
}

func checkInsecureCookies(t Target, _, resp message) []Issue {
	var out []Issue
	for _, sc := range resp.headers.Values("Set-Cookie") {
		// Parse the attribute list (segments after the name=value pair) so a
		// cookie name or value containing "secure"/"samesite"/etc. cannot be
		// mistaken for the attribute being set (e.g. a __Secure-prefixed cookie).
		segs := strings.Split(sc, ";")
		name := strings.TrimSpace(segs[0])
		if i := strings.IndexByte(name, '='); i >= 0 {
			name = name[:i]
		}
		name = strings.TrimSpace(name)
		var hasSecure, hasHTTPOnly, hasSameSite bool
		for _, attr := range segs[1:] {
			a := strings.ToLower(strings.TrimSpace(attr))
			switch {
			case a == "secure":
				hasSecure = true
			case a == "httponly":
				hasHTTPOnly = true
			case strings.HasPrefix(a, "samesite"):
				hasSameSite = true
			}
		}
		var missing []string
		secureMissing := t.Scheme == "https" && !hasSecure
		if secureMissing {
			missing = append(missing, "Secure")
		}
		if !hasHTTPOnly {
			missing = append(missing, "HttpOnly")
		}
		if !hasSameSite {
			missing = append(missing, "SameSite")
		}
		if len(missing) == 0 {
			continue
		}
		sev := SeverityLow
		if secureMissing {
			sev = SeverityMedium
		}
		out = append(out, t.newIssue(
			"insecure_cookie", "Cookie set without "+strings.Join(missing, ", "),
			sev, ConfidenceFirm,
			fmt.Sprintf("Cookie %q is set without the %s attribute(s), weakening its protection against theft or cross-site use.", name, strings.Join(missing, ", ")),
			truncate(sc, 256),
			"Set Secure (over HTTPS), HttpOnly, and an appropriate SameSite attribute on sensitive cookies.",
			t.Host, name))
	}
	return out
}

func checkServerBanner(t Target, _, resp message) []Issue {
	var out []Issue
	for _, h := range []string{"Server", "X-Powered-By", "X-AspNet-Version", "X-AspNetMvc-Version"} {
		v := resp.headers.Get(h)
		if v == "" {
			continue
		}
		// Only flag values that disclose a version (contain a digit) or the
		// X-Powered-By technology banner, to avoid noise from a bare "Server: nginx".
		if h != "X-Powered-By" && !strings.ContainsAny(v, "0123456789") {
			continue
		}
		out = append(out, t.newIssue(
			"info_disclosure", "Technology/version disclosure in "+h+" header",
			SeverityInfo, ConfidenceFirm,
			fmt.Sprintf("The %s response header discloses software or version information (%q) that helps an attacker fingerprint the stack.", h, v),
			h+": "+v,
			"Suppress or genericise version banners in response headers.",
			t.Host, h))
	}
	return out
}

func checkCORS(t Target, req, resp message) []Issue {
	acao := resp.headers.Get("Access-Control-Allow-Origin")
	if acao == "" {
		return nil
	}
	creds := strings.EqualFold(resp.headers.Get("Access-Control-Allow-Credentials"), "true")
	origin := req.headers.Get("Origin")

	// Origin reflected back together with credentials is an exploitable
	// cross-origin trust of an arbitrary site.
	if creds && origin != "" && acao == origin && acao != "*" {
		return []Issue{t.newIssue(
			"cors_misconfig", "CORS reflects arbitrary origin with credentials", SeverityHigh, ConfidenceFirm,
			"The response reflects the request Origin in Access-Control-Allow-Origin while also allowing credentials, letting any site read authenticated responses cross-origin.",
			"Access-Control-Allow-Origin: "+acao+" + Access-Control-Allow-Credentials: true",
			"Reflect only an allowlist of trusted origins, and never combine a reflected/wildcard origin with Access-Control-Allow-Credentials.",
			t.Host, t.Path)}
	}
	if acao == "*" {
		return []Issue{t.newIssue(
			"cors_wildcard", "Wildcard CORS policy", SeverityLow, ConfidenceFirm,
			"The response sets `Access-Control-Allow-Origin: *`, allowing any site to read non-credentialed responses cross-origin.",
			"Access-Control-Allow-Origin: *",
			"Restrict Access-Control-Allow-Origin to the specific origins that need cross-origin access.",
			t.Host, t.Path)}
	}
	return nil
}

func checkDirectoryListing(t Target, _, resp message) []Issue {
	low := resp.bodyLow
	if !strings.Contains(low, "<title>index of /") && !strings.Contains(low, "directory listing for") {
		return nil
	}
	return []Issue{t.newIssue(
		"directory_listing", "Directory listing enabled", SeverityMedium, ConfidenceFirm,
		"The server returned an automatic directory listing, which can expose files that were not meant to be enumerable.",
		"", "Disable automatic directory indexing on the web server.",
		t.Host, t.Path)}
}

// errorSignatures are framework/language error fragments that indicate an
// unhandled exception or stack trace leaked into the response body.
var errorSignatures = []string{
	"traceback (most recent call last)",
	"exception in thread",
	"java.lang.",
	"system.nullreferenceexception",
	"at java.",
	"fatal error:",
	"warning: include(",
	"undefined index:",
	"call to a member function",
	"org.apache.",
	".java line",
	"stack trace:",
}

func checkErrorDisclosure(t Target, _, resp message) []Issue {
	low := resp.bodyLow
	for _, sig := range errorSignatures {
		if strings.Contains(low, sig) {
			return []Issue{t.newIssue(
				"error_disclosure", "Verbose error / stack trace disclosed", SeverityLow, ConfidenceTentative,
				"The response body contains what looks like a server-side stack trace or framework error, which can reveal internal paths, libraries, and logic.",
				sig, "Return generic error pages and log details server-side instead of returning them to the client.",
				t.Host, t.Path)}
		}
	}
	return nil
}

var passwordInputRe = regexp.MustCompile(`type\s*=\s*['"]?password`)

func checkPasswordOverHTTP(t Target, _, resp message) []Issue {
	if t.Scheme != "http" || !isHTML(resp.headers) {
		return nil
	}
	// Tolerate single/double/unquoted attributes and whitespace around '='.
	if !passwordInputRe.MatchString(resp.bodyLow) {
		return nil
	}
	return []Issue{t.newIssue(
		"password_over_http", "Password field served over cleartext HTTP", SeverityMedium, ConfidenceFirm,
		"An HTML form with a password input was served over plaintext HTTP, so submitted credentials can be intercepted on the network.",
		"", "Serve all pages with credential inputs exclusively over HTTPS.",
		t.Host, t.Path)}
}
