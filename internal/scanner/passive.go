package scanner

import (
	"fmt"
	"net/http"
	"strings"
)

// Passive runs every passive check against a captured exchange and returns the
// findings. It never sends a request.
func Passive(t Target) []Issue {
	req := parseMessage(t.RequestRaw)
	resp := parseMessage(t.ResponseRaw)
	var out []Issue
	for _, check := range passiveChecks {
		out = append(out, check(t, req, resp)...)
	}
	return out
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
		t.Host)}
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
		t.Host)}
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
		name := sc
		if i := strings.IndexByte(sc, '='); i >= 0 {
			name = sc[:i]
		}
		name = strings.TrimSpace(name)
		low := strings.ToLower(sc)
		var missing []string
		if t.Scheme == "https" && !strings.Contains(low, "secure") {
			missing = append(missing, "Secure")
		}
		if !strings.Contains(low, "httponly") {
			missing = append(missing, "HttpOnly")
		}
		if !strings.Contains(low, "samesite") {
			missing = append(missing, "SameSite")
		}
		if len(missing) == 0 {
			continue
		}
		sev := SeverityLow
		if t.Scheme == "https" && !strings.Contains(low, "secure") {
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
	low := strings.ToLower(string(resp.body))
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
	low := strings.ToLower(string(resp.body))
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

func checkPasswordOverHTTP(t Target, _, resp message) []Issue {
	if t.Scheme != "http" || !isHTML(resp.headers) {
		return nil
	}
	low := strings.ToLower(string(resp.body))
	if !strings.Contains(low, `type="password"`) && !strings.Contains(low, "type=password") {
		return nil
	}
	return []Issue{t.newIssue(
		"password_over_http", "Password field served over cleartext HTTP", SeverityMedium, ConfidenceFirm,
		"An HTML form with a password input was served over plaintext HTTP, so submitted credentials can be intercepted on the network.",
		"", "Serve all pages with credential inputs exclusively over HTTPS.",
		t.Host, t.Path)}
}
