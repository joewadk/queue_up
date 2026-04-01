package com.queueup.sanitizer.web;

import com.fasterxml.jackson.annotation.JsonProperty;

public record SubmissionSanitizerResponse(
        @JsonProperty("valid") boolean valid,
        @JsonProperty("sanitized_submission_url") String sanitizedSubmissionUrl,
        @JsonProperty("reason") String reason
) {
    public static SubmissionSanitizerResponse ok(String sanitizedUrl) {
        return new SubmissionSanitizerResponse(true, sanitizedUrl, "");
    }

    public static SubmissionSanitizerResponse invalid(String reason) {
        return new SubmissionSanitizerResponse(false, "", reason == null ? "" : reason.trim());
    }
}
