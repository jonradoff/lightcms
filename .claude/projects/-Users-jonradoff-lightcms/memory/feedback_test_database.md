---
name: test-database-config
description: Test suite must use lightcms-test user and lightcms-test database only; drop collections after tests
type: feedback
---

When running coverage/integration tests, use the `lightcms-test` MongoDB user (password: P8ZoiX04QNyqYgsR) and the `lightcms-test` database only. After tests conclude, drop all collections in the test database.

**Why:** Prevent accidental use of production database during tests; keep test database clean.

**How to apply:** Set MONGODB_URI and DATABASE_NAME env vars when running tests. Use the lightcms-test user credentials. Always drop test collections after test runs complete.
