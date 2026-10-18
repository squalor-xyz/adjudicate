# Evaluate GraalVM native images

Status: proposed
Date: 2023-02-14

## Context and Problem Statement

Start-up time matters for our functions. Should we build native images?

## Considered Options

* Native images with GraalVM
* Keep the JVM

## Decision Outcome

Chosen option: "Native images with GraalVM", because start-up drops from seconds to milliseconds.
