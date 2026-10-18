---
status: accepted
date: 2022-09-01
decision-makers: Alice Example
---
# Use Quarkus

## Context and Problem Statement

Which application framework should the services use, now that start-up time matters?

## Considered Options

* Quarkus
* Spring Boot

## Decision Outcome

Chosen option: "Quarkus", because it starts fast and fits our native-image plan.

### Consequences

* Good, because start-up is fast
* Bad, because few engineers know it yet
