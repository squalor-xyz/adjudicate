# 4. Use CockroachDB

Date: 2019-07-09

## Status

Accepted

Supersedes [2. Use PostgreSQL](0002-use-postgresql.md)

## Context

A single PostgreSQL node cannot give the availability the service now needs.

## Decision

We will store all service data in CockroachDB.

## Consequences

Operations learn a new database; the wire protocol stays PostgreSQL's.
