# 1. Record architecture decisions

Date: 2018-11-07

## Status

Accepted

## Context

The GRPC default message size of 4 mb currently causing a bottleneck between cc-route-syncer and copilot. As our message sizes increased with scale this prevents us from sending messages to copilot.

## Decision

We have decided to reduce the message size by enabling GRPC's GZIP compression between cc-route-syncer and copilot.

## Consequences

This could be pottentially a temporary fix as the message size gets bigger. The GZIP compression message size could still go above the default message size in GRPC in the future.
