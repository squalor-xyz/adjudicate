# 2. Use PostgreSQL

Date: 2017-03-01

## Status

Superseded by [4. Use CockroachDB](0004-use-cockroachdb.md)

## Context

The service needs a relational store with transactions.

## Decision

We will store all service data in PostgreSQL.

## Consequences

We run and patch PostgreSQL ourselves, and a single node limits availability.
