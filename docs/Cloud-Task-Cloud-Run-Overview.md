# GCP Cloud Tasks and Cloud Run Jobs: Comprehensive Research & Integration Guide

**Date**: March 2, 2026  
**Version**: 1.0  
**Context**: Jennah Cloud Service Integration (Task 4.1: Research & Understanding Phase)  
**Purpose**: Multi-Cloud Support Integration - GCP Services Research

---

## Executive Summary

This document provides comprehensive technical research on two GCP services critical for workload deployment:

- **Cloud Tasks**: Asynchronous task dispatch with explicit routing, rate control, and queue management
- **Cloud Run Jobs**: Serverless batch processing with native parallelism (up to 10,000 tasks) and container-based execution

Both services complement Jennah's current GCP Batch integration and offer distinct execution models suitable for different workload patterns.

---

## Table of Contents

1. [Architectural Overview & Differences](#architectural-overview--differences)
2. [Cloud Tasks: Complete Specification](#cloud-tasks-complete-specification)
3. [Cloud Run Jobs: Complete Specification](#cloud-run-jobs-complete-specification)
4. [Pricing Models](#pricing-models)
5. [API & Implementation Guides](#api--implementation-guides)
6. [Feature Comparison Matrix](#feature-comparison-matrix)
7. [Comparison with Alternative Services](#comparison-with-alternative-services)
8. [Execution Guarantees & Tradeoffs](#execution-guarantees--tradeoffs)
9. [Key Capabilities & Constraints](#key-capabilities--constraints)
10. [Decision Framework: When to Use Each Service](#decision-framework-when-to-use-each-service)
11. [Implementation Considerations](#implementation-considerations)
12. [Integration Recommendations for Jennah](#integration-recommendations-for-jennah)

---

## Architectural Overview & Differences

### GCP Cloud Tasks

**Purpose**: A fully managed service for asynchronous task dispatch and execution with explicit invocation and full queue management control. Tasks are sent to HTTP endpoints or App Engine handlers.

**Core Concept**: Producer-controlled message routing where the publisher specifies exactly where and when each task will be executed.

**Key Characteristics**:

- Explicit invocation (publisher controls execution target)
- Task-level routing configuration
- Queue-based task management with persistent storage
- Supports scheduled delivery times
- Full individual task access and management
- Task deduplication capability (24-hour window)
- At-least-once delivery guarantee

**Execution Model**:

- Tasks queued and dispatched based on rate limits
- HTTP handlers must respond with 2xx status code within timeout
- Failed tasks automatically retried per exponential backoff configuration
- Task age tracked for deduplication post-deletion
- Queue enforces rate limiting, parallelism, and retry policies

---

### GCP Cloud Run Jobs

**Purpose**: Serverless batch processing for parallelizable workloads that run to completion and exit, with automatic scaling and per-task status tracking.

**Core Concept**: Container-based execution environment where jobs consist of one or more independent tasks that execute in parallel, with per-task retry logic and completion guarantees.

**Introduced As**: New feature in Cloud Run specifically for one-off and scheduled batch executions (distinct from Cloud Run Services which handle continuous HTTP requests).

**Key Characteristics**:

- Array jobs (up to 10,000 parallel tasks per job)
- Per-task parallelism control
- Container-based execution with custom images
- Task-level retry configuration (default 3 retries)
- Automatic status tracking and logging (Cloud Logging integration)
- Integration with Cloud Scheduler for recurring execution
- Per-task index awareness (CLOUD_RUN_TASK_INDEX, CLOUD_RUN_TASK_COUNT)

**Execution Model**:

- Job execution succeeds only when ALL tasks succeed
- Tasks run independently with index awareness for distributed processing
- Single task or distributed array job pattern supported
- Real-time logs to Cloud Logging (stderr/stdout)
- Each task runs one container instance
- Task failures trigger automatic retries; eventual failure marks entire job FAILED

---

## Cloud Tasks: Complete Specification

### Task Properties & Constraints

| Property                 | Value                 | Notes                                                   |
| ------------------------ | --------------------- | ------------------------------------------------------- |
| **Maximum Message Size** | 1 MB                  | Payload chunked in 32 KB units for billing              |
| **Payload Methods**      | POST/PUT              | Tasks with payloads require POST/PUT                    |
| **Task Naming**          | Optional unique names | System-generated if not provided; enables deduplication |
| **Deduplication Window** | 24 hours              | After task deletion or completion                       |
| **Task Retention**       | 30 days               | Auto-deleted after retention period                     |

### Timeout Limits by Target Type

| Target Type                                   | Default Timeout | Maximum Timeout |
| --------------------------------------------- | --------------- | --------------- |
| HTTP Targets (Cloud Run, GKE, Compute Engine) | 10 minutes      | 30 minutes      |
| App Engine Standard (Auto Scaling)            | 10 minutes      | 10 minutes      |
| App Engine Standard (Manual/Basic)            | 10 minutes      | 24 hours        |
| App Engine Flexible                           | 10 minutes      | 60 minutes      |

### Delivery Guarantees & Ordering

| Aspect                   | Detail                                               |
| ------------------------ | ---------------------------------------------------- |
| **Delivery Guarantee**   | At-least-once delivery                               |
| **Message Ordering**     | Best-effort preservation (not guaranteed)            |
| **Idempotency Required** | YES - Handler must handle duplicate execution safely |
| **Geographic Scope**     | Regional (queue must be in same region as target)    |

### Queue Configuration Limits

| Parameter                     | Default Value | Max Value/Notes                 |
| ----------------------------- | ------------- | ------------------------------- |
| **Max Queues per Project**    | 1,000         | Higher via quota request        |
| **Max Concurrent Dispatches** | 1,000         | Simultaneous task executions    |
| **Max Dispatches Per Second** | 500/queue     | Token refill rate               |
| **Max Task Retention**        | 30 days       | Auto-deletion after this period |
| **Max Delivery Rate**         | 500 qps       | Per queue limit                 |

### Queue Retry Configuration

Queue-level retry settings apply to all tasks (can be overridden per task):
