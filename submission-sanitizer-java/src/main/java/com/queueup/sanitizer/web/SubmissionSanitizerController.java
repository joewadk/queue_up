package com.queueup.sanitizer.web;

import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.net.URI;
import java.util.Locale;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

@RestController
@RequestMapping("/v1/submissions")
public class SubmissionSanitizerController {
    private static final Pattern LEETCODE_PATH_PATTERN =
            Pattern.compile("^/problems/([^/]+)/submissions/(\\d+)/?$");

    @GetMapping("/health")
    public ResponseEntity<?> health() {
        return ResponseEntity.ok().body(java.util.Map.of("status", "ok"));
    }

    @PostMapping("/sanitize")
    public ResponseEntity<SubmissionSanitizerResponse> sanitize(@RequestBody SubmissionSanitizerRequest req) {
        String rawUrl = trim(req.submissionUrl());
        if (rawUrl.isEmpty()) {
            return ResponseEntity.ok(SubmissionSanitizerResponse.invalid("submission_url is required"));
        }

        URI uri;
        try {
            uri = URI.create(rawUrl);
        } catch (IllegalArgumentException ex) {
            return ResponseEntity.ok(SubmissionSanitizerResponse.invalid("submission_url is invalid"));
        }

        String scheme = trim(uri.getScheme()).toLowerCase(Locale.ROOT);
        if (!scheme.equals("http") && !scheme.equals("https")) {
            return ResponseEntity.ok(SubmissionSanitizerResponse.invalid("submission_url must use http(s)"));
        }

        String host = trim(uri.getHost()).toLowerCase(Locale.ROOT);
        if (host.startsWith("www.")) {
            host = host.substring(4);
        }
        if (!host.equals("leetcode.com")) {
            return ResponseEntity.ok(SubmissionSanitizerResponse.invalid("submission_url must be from leetcode.com"));
        }

        String path = trim(uri.getPath());
        Matcher matcher = LEETCODE_PATH_PATTERN.matcher(path);
        if (!matcher.matches()) {
            return ResponseEntity.ok(SubmissionSanitizerResponse.invalid(
                    "submission_url must match /problems/{slug}/submissions/{id}"));
        }

        String slug = trim(matcher.group(1));
        String submissionId = trim(matcher.group(2));
        String expectedSlug = trim(req.expectedSlug());
        if (!expectedSlug.isEmpty() && !slug.equalsIgnoreCase(expectedSlug)) {
            return ResponseEntity.ok(SubmissionSanitizerResponse.invalid(
                    "submission_url problem does not match selected queue problem"));
        }

        String sanitized = "https://leetcode.com/problems/" + slug + "/submissions/" + submissionId + "/";
        return ResponseEntity.ok(SubmissionSanitizerResponse.ok(sanitized));
    }

    private static String trim(String s) {
        return s == null ? "" : s.trim();
    }
}
