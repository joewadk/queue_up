package com.queueup.sanitizer.web;

import com.fasterxml.jackson.annotation.JsonProperty;

public record SubmissionSanitizerRequest(
        @JsonProperty("user_id") String userId,
        @JsonProperty("problem_id") Long problemId,
        @JsonProperty("expected_slug") String expectedSlug,
        @JsonProperty("submission_url") String submissionUrl
) {
}
