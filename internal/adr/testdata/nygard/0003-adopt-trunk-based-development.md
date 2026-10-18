# 3. Adopt trunk-based development

Date: 2018-05-20

## Status

Proposed

## Context

Long-lived branches make merges painful and hide integration problems.

## Decision

We will merge to the main branch at least daily and use feature flags.

## Consequences

Unfinished work ships behind flags, which need cleaning up.
