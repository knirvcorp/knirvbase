# KNIRVQL Specification (Production-Ready)

**Version:** 2.0
**Date:** December 24, 2024
**Status:** Enhanced Specification
**Target:** KNIRVBASE Post-Implementation

---

## Executive Summary

This document specifies the **complete KNIRVQL query language** as it will exist after implementing all production-ready enhancements. KNIRVQL is a human-friendly, SQL-like query language designed for KNIRVBASE's distributed document database.

**Design Goals:**
- **Familiar Syntax:** SQL-like for easy adoption
- **NoSQL Power:** Native support for documents, arrays, nested objects
- **Vector Search:** First-class vector similarity queries
- **CRDT-Aware:** Integrates with KNIRVBASE's conflict resolution
- **Type-Safe:** Schema validation and type checking
- **Performance:** Query optimization and index-aware execution

---

## Table of Contents

1. [Language Overview](#1-language-overview)
2. [Data Definition Language (DDL)](#2-data-definition-language-ddl)
3. [Data Manipulation Language (DML)](#3-data-manipulation-language-dml)
4. [Query Language (DQL)](#4-query-language-dql)
5. [Vector Operations](#5-vector-operations)
6. [Index Management](#6-index-management)
7. [Transactions](#7-transactions)
8. [Advanced Features](#8-advanced-features)

---

## 1. Language Overview

### 1.1 Syntax Foundations

**Case Sensitivity:**
- **Keywords:** Case-insensitive (`GET` = `get` = `Get`)
- **Identifiers:** Case-sensitive (`username` ≠ `UserName`)
- **Strings:** Case-sensitive (`"Alice"` ≠ `"alice"`)

**Comments:**
```knirvql
-- Single-line comment

/* Multi-line
   comment */
```

**Data Types:**
- `string` - UTF-8 text
- `integer` - 64-bit signed integer
- `float` - 64-bit floating point
- `boolean` - true/false
- `binary` - Byte array
- `timestamp` - Unix timestamp (milliseconds)
- `uuid` - RFC 4122 UUID
- `object` - JSON object
- `array` - JSON array
- `vector` - Float32 array (for similarity search)

**Literals:**
```knirvql
"string literal"
42                  -- integer
3.14159            -- float
true, false        -- boolean
NOW()              -- current timestamp
UUID()             -- generate UUID
[0.1, 0.2, 0.3]    -- vector/array
{"key": "value"}   -- object
```

---

## 2. Data Definition Language (DDL)

### 2.1 Collection Management

**Create Collection:**
```knirvql
CREATE COLLECTION collection_name

-- With schema definition
CREATE COLLECTION credentials WITH SCHEMA {
    "entryType": "CREDENTIAL",
    "fields": {
        "username": {
            "type": "string",
            "required": true,
            "unique": true
        },
        "email": {
            "type": "string",
            "format": "email"
        },
        "age": {
            "type": "integer",
            "min": 0,
            "max": 150
        }
    }
}

-- With options
CREATE COLLECTION audit_log WITH OPTIONS {
    "immutable": true,
    "compression": "zstd",
    "encryption": "PQC-Kyber768"
}
```

**Alter Collection:**
```knirvql
-- Add field to schema
ALTER COLLECTION credentials
ADD FIELD mfa_enabled {
    "type": "boolean",
    "default": false
}

-- Modify field
ALTER COLLECTION credentials
MODIFY FIELD email {
    "type": "string",
    "required": true,
    "format": "email"
}

-- Drop field (soft removal, data preserved)
ALTER COLLECTION credentials
DROP FIELD old_field

-- Change collection options
ALTER COLLECTION audit_log
SET OPTION immutable = true
```

**Drop Collection:**
```knirvql
-- Soft drop (mark as deleted, data preserved)
DROP COLLECTION collection_name

-- Hard drop (permanent deletion)
DROP COLLECTION collection_name PERMANENT
```

**Show Collections:**
```knirvql
SHOW COLLECTIONS

SHOW COLLECTIONS LIKE "credential%"

DESCRIBE COLLECTION credentials
```

---

### 2.2 Index Management

**Create Index:**
```knirvql
-- Simple index
CREATE INDEX credentials:username
ON credentials (username)

-- Unique index
CREATE INDEX credentials:email
ON credentials (email)
UNIQUE

-- Composite index
CREATE INDEX credentials:status_created
ON credentials (status, created_at)

-- Partial index (filtered)
CREATE INDEX credentials:active_users
ON credentials (status)
WHERE status = "active"

-- Sorted index
CREATE INDEX credentials:last_used_desc
ON credentials (last_used)
ORDER BY DESC

-- GIN index (for JSONB)
CREATE INDEX credentials:metadata
ON credentials (metadata)
TYPE GIN

-- Vector index (HNSW)
CREATE INDEX memory:vector
ON memory (vector)
TYPE HNSW
WITH OPTIONS {
    "dimensions": 768,
    "metric": "cosine",
    "M": 16,
    "efConstruction": 200
}
```

**Drop Index:**
```knirvql
DROP INDEX credentials:username

DROP INDEX IF EXISTS credentials:old_index
```

**Show Indexes:**
```knirvql
SHOW INDEXES ON credentials

SHOW INDEX credentials:username
```

---

### 2.3 Triggers

**Create Trigger:**
```knirvql
-- Auto-update timestamp
CREATE TRIGGER credentials:auto_update
BEFORE UPDATE ON credentials
SET updated_at = NOW()

-- Audit log on changes
CREATE TRIGGER credentials:audit
AFTER INSERT, UPDATE, DELETE ON credentials
EXECUTE {
    INSERT INTO audit_log
    SET event_type = @trigger.operation,
        collection = @trigger.collection,
        document_id = @trigger.document_id,
        old_value = @trigger.old,
        new_value = @trigger.new,
        timestamp = NOW()
}

-- Validation trigger
CREATE TRIGGER credentials:validate_email
BEFORE INSERT, UPDATE ON credentials
WHEN email IS NOT NULL
EXECUTE {
    IF NOT email MATCHES "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$" THEN
        RAISE ERROR "Invalid email format"
    END IF
}
```

**Drop Trigger:**
```knirvql
DROP TRIGGER credentials:auto_update
```

---

## 3. Data Manipulation Language (DML)

### 3.1 Insert Operations

**Basic Insert:**
```knirvql
INSERT INTO credentials
SET username = "alice@example.com",
    email = "alice@example.com",
    hash = @encrypted_hash,
    salt = @random_salt,
    created_at = NOW()
```

**Insert with Nested Objects:**
```knirvql
INSERT INTO users
SET username = "bob",
    profile = {
        "name": "Bob Smith",
        "age": 30,
        "address": {
            "city": "San Francisco",
            "country": "USA"
        }
    },
    tags = ["developer", "admin"]
```

**Insert Multiple:**
```knirvql
INSERT INTO credentials
VALUES
    (username: "alice@example.com", email: "alice@example.com"),
    (username: "bob@example.com", email: "bob@example.com"),
    (username: "charlie@example.com", email: "charlie@example.com")
```

**Insert or Replace:**
```knirvql
UPSERT INTO credentials
SET username = "alice@example.com",
    email = "alice@example.com",
    updated_at = NOW()
```

**Insert and Return:**
```knirvql
INSERT INTO credentials
SET username = "alice@example.com"
RETURNING id, username, created_at
```

---

### 3.2 Update Operations

**Simple Update:**
```knirvql
UPDATE credentials
SET last_used = NOW()
WHERE username = "alice@example.com"
```

**Update with Expressions:**
```knirvql
-- Increment counter
UPDATE credentials
SET failed_attempts = failed_attempts + 1
WHERE username = "alice@example.com"

-- Conditional update
UPDATE credentials
SET status = "locked",
    locked_until = NOW() + INTERVAL 15 MINUTES
WHERE failed_attempts >= 5
```

**Update Nested Fields:**
```knirvql
-- Update object field
UPDATE users
SET profile.age = 31
WHERE username = "bob"

-- Update array element
UPDATE users
SET tags[0] = "senior-developer"
WHERE username = "bob"

-- Append to array
UPDATE users
SET tags = tags + ["team-lead"]
WHERE username = "bob"
```

**Bulk Update:**
```knirvql
UPDATE credentials
SET status = "expired"
WHERE expires_at < NOW()
  AND status = "active"
```

**Update and Return:**
```knirvql
UPDATE credentials
SET last_used = NOW()
WHERE username = "alice@example.com"
RETURNING last_used
```

---

### 3.3 Delete Operations

**Simple Delete (Soft Delete):**
```knirvql
DELETE FROM credentials
WHERE username = "alice@example.com"
```

**Permanent Delete:**
```knirvql
DELETE FROM credentials
WHERE username = "alice@example.com"
PERMANENT
```

**Conditional Delete:**
```knirvql
-- Delete expired sessions
DELETE FROM sessions
WHERE expires_at < NOW()

-- Delete old audit logs (with limit)
DELETE FROM audit_log
WHERE timestamp < NOW() - INTERVAL 90 DAYS
LIMIT 1000
```

**Delete and Return:**
```knirvql
DELETE FROM sessions
WHERE session_id = "550e8400-e29b-41d4-a716-446655440000"
RETURNING session_id, username, created_at
```

---

## 4. Query Language (DQL)

### 4.1 Basic Queries

**Select All:**
```knirvql
GET CREDENTIAL FROM credentials

GET * FROM credentials  -- Equivalent
```

**Select Specific Fields:**
```knirvql
GET username, email, status FROM credentials
```

**With Filters:**
```knirvql
GET CREDENTIAL FROM credentials
WHERE status = "active"

GET username FROM credentials
WHERE status = "active" AND email LIKE "%@example.com"
```

---

### 4.2 Comparison Operators

```knirvql
WHERE age = 30               -- Equal
WHERE age != 30              -- Not equal
WHERE age > 30               -- Greater than
WHERE age >= 30              -- Greater than or equal
WHERE age < 30               -- Less than
WHERE age <= 30              -- Less than or equal
WHERE email LIKE "%@gmail.com"      -- Pattern matching
WHERE email NOT LIKE "%spam%"       -- Negative pattern
WHERE username IN ("alice", "bob")  -- In list
WHERE status NOT IN ("deleted", "banned")
WHERE metadata.role IS NULL         -- Null check
WHERE metadata.role IS NOT NULL     -- Not null check
WHERE email MATCHES "^[a-z]+@"      -- Regex match
WHERE tags CONTAINS "admin"         -- Array contains
WHERE profile.city STARTS WITH "San"  -- String prefix
WHERE profile.city ENDS WITH "cisco"  -- String suffix
```

---

### 4.3 Logical Operators

```knirvql
-- AND
WHERE status = "active" AND age >= 18

-- OR
WHERE status = "active" OR status = "pending"

-- NOT
WHERE NOT (status = "deleted")

-- Complex expressions
WHERE (status = "active" OR status = "pending")
  AND age >= 18
  AND (metadata.role = "admin" OR metadata.role = "moderator")
```

---

### 4.4 Sorting and Limiting

**ORDER BY:**
```knirvql
GET CREDENTIAL FROM credentials
ORDER BY created_at DESC

GET CREDENTIAL FROM credentials
ORDER BY status ASC, created_at DESC

GET CREDENTIAL FROM credentials
ORDER BY metadata.priority DESC, username ASC
```

**LIMIT and OFFSET:**
```knirvql
-- Pagination
GET CREDENTIAL FROM credentials
LIMIT 10

GET CREDENTIAL FROM credentials
LIMIT 10 OFFSET 20

-- Top N
GET CREDENTIAL FROM credentials
ORDER BY last_used DESC
LIMIT 5
```

---

### 4.5 Aggregations

**COUNT:**
```knirvql
COUNT * FROM credentials

COUNT * FROM credentials
WHERE status = "active"

COUNT DISTINCT email FROM credentials
```

**SUM, AVG, MIN, MAX:**
```knirvql
SUM failed_attempts FROM credentials

AVG failed_attempts FROM credentials
WHERE status = "active"

MIN created_at FROM credentials

MAX last_used FROM credentials
```

**GROUP BY:**
```knirvql
COUNT * FROM credentials
GROUP BY status

AVG failed_attempts FROM credentials
GROUP BY status
ORDER BY COUNT(*) DESC

COUNT * FROM credentials
GROUP BY status, metadata.role
```

**HAVING:**
```knirvql
COUNT * FROM credentials
GROUP BY status
HAVING COUNT(*) > 100

AVG age FROM users
GROUP BY profile.city
HAVING AVG(age) > 30
```

---

### 4.6 Subqueries

**IN Subquery:**
```knirvql
GET CREDENTIAL FROM credentials
WHERE username IN (
    SELECT username FROM sessions
    WHERE created_at > NOW() - INTERVAL 1 HOUR
)
```

**EXISTS Subquery:**
```knirvql
GET CREDENTIAL FROM credentials AS c
WHERE EXISTS (
    SELECT * FROM sessions AS s
    WHERE s.username = c.username
      AND s.status = "active"
)
```

**Scalar Subquery:**
```knirvql
UPDATE credentials
SET failed_attempts = (
    SELECT COUNT(*) FROM audit_log
    WHERE event_type = "login_failed"
      AND username = credentials.username
)
WHERE username = "alice@example.com"
```

---

## 5. Vector Operations

### 5.1 Vector Similarity Search

**Cosine Similarity:**
```knirvql
GET MEMORY FROM memory
WHERE vector SIMILAR TO [0.1, 0.2, 0.3, ..., 0.768]
USING cosine
LIMIT 10
```

**Euclidean Distance:**
```knirvql
GET MEMORY FROM memory
WHERE vector SIMILAR TO @query_vector
USING euclidean
WITHIN 0.5
LIMIT 20
```

**Dot Product:**
```knirvql
GET MEMORY FROM memory
WHERE vector SIMILAR TO @query_vector
USING dotproduct
LIMIT 10
```

**With Filters (Hybrid Search):**
```knirvql
-- Vector search with metadata filters
GET MEMORY FROM memory
WHERE source = "web"
  AND created_at > NOW() - INTERVAL 7 DAYS
  AND vector SIMILAR TO @query_vector
USING cosine
LIMIT 10

-- Vector search with score threshold
GET MEMORY FROM memory
WHERE vector SIMILAR TO @query_vector
USING cosine
HAVING similarity_score > 0.8
LIMIT 10
```

**Multi-Vector Search:**
```knirvql
-- Search multiple vectors and merge results
GET MEMORY FROM memory
WHERE vector SIMILAR TO @query_vector1
   OR vector SIMILAR TO @query_vector2
USING cosine
LIMIT 20
```

---

### 5.2 Vector Operations

**Vector Arithmetic:**
```knirvql
-- Average vectors
UPDATE memory
SET vector = (vector1 + vector2 + vector3) / 3
WHERE id = "doc1"

-- Normalize vector
UPDATE memory
SET vector = vector / MAGNITUDE(vector)
WHERE id = "doc1"
```

**Vector Functions:**
```knirvql
-- Magnitude
SELECT MAGNITUDE(vector) FROM memory WHERE id = "doc1"

-- Distance between vectors
SELECT DISTANCE(vector1, vector2, "cosine") FROM memory

-- Dot product
SELECT DOT(vector1, vector2) FROM memory
```

---

## 6. Index Management

### 6.1 Index Usage Hints

**Force Index:**
```knirvql
GET CREDENTIAL FROM credentials
USE INDEX credentials:username
WHERE username = "alice@example.com"
```

**Ignore Index:**
```knirvql
GET CREDENTIAL FROM credentials
IGNORE INDEX credentials:status
WHERE status = "active"
```

**Index Selection:**
```knirvql
-- Let optimizer choose
GET CREDENTIAL FROM credentials
WHERE username = "alice@example.com"
  AND status = "active"
PREFER INDEX credentials:username
```

---

### 6.2 Explain Query Plan

**Show Query Plan:**
```knirvql
EXPLAIN GET CREDENTIAL FROM credentials
WHERE username = "alice@example.com"
  AND status = "active"
ORDER BY created_at DESC
LIMIT 10
```

**Output:**
```
Query Plan:
  1. Index Scan: credentials:username (username = "alice@example.com")
     Estimated rows: 1
  2. Filter: status = "active"
     Estimated rows: 1
  3. Sort: created_at DESC
     Method: In-memory
  4. Limit: 10

Estimated Cost: 5.2
Estimated Time: < 1ms
```

**Analyze Query:**
```knirvql
EXPLAIN ANALYZE GET CREDENTIAL FROM credentials
WHERE status = "active"
ORDER BY created_at DESC
LIMIT 100
```

**Output:**
```
Query Plan:
  1. Index Scan: credentials:status (status = "active")
     Estimated rows: 1000
     Actual rows: 1234
     Actual time: 2.3ms
  2. Sort: created_at DESC
     Method: External merge sort
     Actual time: 8.7ms
  3. Limit: 100
     Actual time: 0.1ms

Total time: 11.1ms
```

---

## 7. Transactions

### 7.1 Transaction Control

**Begin Transaction:**
```knirvql
BEGIN TRANSACTION

-- Or with isolation level
BEGIN TRANSACTION ISOLATION LEVEL SERIALIZABLE
```

**Commit:**
```knirvql
COMMIT
```

**Rollback:**
```knirvql
ROLLBACK

-- Rollback to savepoint
ROLLBACK TO SAVEPOINT sp1
```

**Savepoints:**
```knirvql
BEGIN TRANSACTION

INSERT INTO credentials SET username = "alice@example.com"
SAVEPOINT sp1

INSERT INTO credentials SET username = "bob@example.com"
SAVEPOINT sp2

ROLLBACK TO SAVEPOINT sp1

COMMIT
```

---

### 7.2 Transaction Example

```knirvql
BEGIN TRANSACTION

-- Deduct from sender
UPDATE accounts
SET balance = balance - 100
WHERE username = "alice"

-- Add to receiver
UPDATE accounts
SET balance = balance + 100
WHERE username = "bob"

-- Log transaction
INSERT INTO audit_log
SET event_type = "transfer",
    details = {"from": "alice", "to": "bob", "amount": 100},
    timestamp = NOW()

COMMIT
```

---

## 8. Advanced Features

### 8.1 Variables

**Declare and Use:**
```knirvql
-- Declare variable
DECLARE @username string = "alice@example.com"
DECLARE @limit integer = 10

-- Use variable
GET CREDENTIAL FROM credentials
WHERE username = @username
LIMIT @limit
```

**Bind Parameters (Prepared Statements):**
```knirvql
-- Prepare statement
PREPARE get_user AS
    GET CREDENTIAL FROM credentials
    WHERE username = $1
    LIMIT 1

-- Execute with parameter
EXECUTE get_user("alice@example.com")
```

---

### 8.2 Functions

**Built-in Functions:**

**String Functions:**
```knirvql
UPPER(username)          -- Convert to uppercase
LOWER(email)             -- Convert to lowercase
SUBSTRING(username, 0, 5)  -- Extract substring
LENGTH(username)         -- String length
TRIM(username)           -- Remove whitespace
CONCAT(first_name, " ", last_name)  -- Concatenate strings
REPLACE(email, "@", "_at_")  -- Replace substring
```

**Date/Time Functions:**
```knirvql
NOW()                    -- Current timestamp
INTERVAL 7 DAYS          -- Time interval
DATE_ADD(created_at, INTERVAL 30 DAYS)  -- Add interval
DATE_DIFF(NOW(), created_at, DAYS)  -- Difference in days
YEAR(created_at)         -- Extract year
MONTH(created_at)        -- Extract month
DAY(created_at)          -- Extract day
```

**Math Functions:**
```knirvql
ABS(value)               -- Absolute value
ROUND(value, 2)          -- Round to 2 decimals
FLOOR(value)             -- Round down
CEIL(value)              -- Round up
SQRT(value)              -- Square root
POW(value, 2)            -- Power
```

**JSON Functions:**
```knirvql
JSON_EXTRACT(metadata, "$.role")  -- Extract JSON value
JSON_SET(metadata, "$.role", "admin")  -- Set JSON value
JSON_LENGTH(tags)        -- Array length
JSON_CONTAINS(tags, "admin")  -- Check array contains
```

**Crypto Functions:**
```knirvql
SHA256(password)         -- SHA-256 hash
HMAC(message, key)       -- HMAC signature
RANDOM_BYTES(32)         -- Generate random bytes
UUID()                   -- Generate UUID
```

---

### 8.3 User-Defined Functions

**Create Function:**
```knirvql
CREATE FUNCTION is_admin(username string) RETURNS boolean AS {
    RETURN (
        SELECT metadata.role FROM credentials
        WHERE username = $username
        LIMIT 1
    ) = "admin"
}

-- Use function
GET CREDENTIAL FROM credentials
WHERE is_admin(username)
```

---

### 8.4 Stored Procedures

**Create Procedure:**
```knirvql
CREATE PROCEDURE lock_account(username string, duration integer) AS {
    UPDATE credentials
    SET status = "locked",
        locked_until = NOW() + INTERVAL duration MINUTES
    WHERE username = $username;

    INSERT INTO audit_log
    SET event_type = "account_locked",
        username = $username,
        details = {"duration": duration},
        timestamp = NOW();
}

-- Call procedure
CALL lock_account("alice@example.com", 15)
```

---

### 8.5 Views

**Create View:**
```knirvql
CREATE VIEW active_users AS
    GET username, email, created_at FROM credentials
    WHERE status = "active"

-- Query view
GET * FROM active_users
ORDER BY created_at DESC
LIMIT 10
```

**Materialized View:**
```knirvql
CREATE MATERIALIZED VIEW user_stats AS
    SELECT status, COUNT(*) as count
    FROM credentials
    GROUP BY status

-- Refresh materialized view
REFRESH MATERIALIZED VIEW user_stats
```

---

## 9. Complete Examples

### 9.1 ASIC-Shield Authentication Workflow

```knirvql
-- 1. Check if user exists and is active
DECLARE @username string = "alice@example.com"
DECLARE @credential object

SET @credential = (
    GET hash, salt, iterations, failed_attempts, status, locked_until
    FROM credentials
    WHERE username = @username
    LIMIT 1
)

-- 2. Check if account is locked
IF @credential.status = "locked" AND @credential.locked_until > NOW() THEN
    RAISE ERROR "Account is locked until " + @credential.locked_until
END IF

-- 3. Verify password (done in application with ASIC)
-- (KDF computation happens here)

-- 4a. On success: Update last_used, reset failed_attempts
IF @password_correct THEN
    UPDATE credentials
    SET last_used = NOW(),
        failed_attempts = 0,
        status = "active"
    WHERE username = @username;

    INSERT INTO audit_log
    SET event_type = "login_success",
        username = @username,
        ip_address = @client_ip,
        timestamp = NOW();

-- 4b. On failure: Increment failed_attempts, lock if needed
ELSE
    UPDATE credentials
    SET failed_attempts = failed_attempts + 1
    WHERE username = @username;

    IF @credential.failed_attempts + 1 >= 5 THEN
        UPDATE credentials
        SET status = "locked",
            locked_until = NOW() + INTERVAL 15 MINUTES
        WHERE username = @username;

        INSERT INTO audit_log
        SET event_type = "account_locked",
            username = @username,
            details = {"reason": "too_many_failed_attempts"},
            timestamp = NOW();
    END IF;

    INSERT INTO audit_log
    SET event_type = "login_failed",
        username = @username,
        ip_address = @client_ip,
        timestamp = NOW();
END IF
```

---

### 9.2 KNIRV_NETWORK Vector Memory Search

```knirvql
-- Semantic memory search with filters
GET MEMORY FROM memory
WHERE source IN ("web", "pdf")
  AND created_at > NOW() - INTERVAL 30 DAYS
  AND metadata.tags CONTAINS "important"
  AND vector SIMILAR TO @query_embedding
USING cosine
HAVING similarity_score > 0.75
ORDER BY similarity_score DESC, created_at DESC
LIMIT 20
```

---

## Appendix: KNIRVQL Grammar (EBNF)

```ebnf
statement ::= ddl_statement | dml_statement | dql_statement | utility_statement

ddl_statement ::= create_collection | alter_collection | drop_collection
                | create_index | drop_index
                | create_trigger | drop_trigger

dml_statement ::= insert_statement | update_statement | delete_statement

dql_statement ::= select_statement

insert_statement ::= "INSERT" "INTO" collection_name "SET" assignment_list
                   | "INSERT" "INTO" collection_name "VALUES" value_list

update_statement ::= "UPDATE" collection_name "SET" assignment_list where_clause?

delete_statement ::= "DELETE" "FROM" collection_name where_clause? "PERMANENT"?

select_statement ::= "GET" field_list "FROM" collection_name
                     where_clause?
                     order_clause?
                     limit_clause?

where_clause ::= "WHERE" condition

condition ::= comparison
            | condition "AND" condition
            | condition "OR" condition
            | "NOT" condition
            | "(" condition ")"

comparison ::= field operator value
             | field "SIMILAR" "TO" vector "USING" metric
             | field "IN" "(" value_list ")"
             | field "LIKE" pattern
             | field "IS" "NULL"
             | field "IS" "NOT" "NULL"

operator ::= "=" | "!=" | ">" | ">=" | "<" | "<="
```

---

**Document Version:** 2.0
**Last Updated:** December 24, 2024
**Status:** Production-Ready Specification
**Next Review:** After Phase 1 implementation
