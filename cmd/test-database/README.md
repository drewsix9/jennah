# Database Test Program

Simple program to test the Cloud Spanner database package.

## Prerequisites

You need to authenticate with Google Cloud:

```powershell
# Authenticate with your Google account
gcloud auth login

# Set application default credentials (required for the Go SDK)
gcloud auth application-default login
```

## Run the Test

```powershell
# From the project root
cd c:\Users\User\Jennah\jennah

# Run the test program
go run -mod=mod ./cmd/test-database/main.go
```

## What It Tests

1. ✅ Connect to Cloud Spanner
2. ✅ Create a tenant
3. ✅ Get tenant by ID
4. ✅ List all tenants
5. ✅ Create a job
6. ✅ Get job by ID
7. ✅ Update job status (PENDING → RUNNING)
8. ✅ List jobs for tenant
9. ✅ List jobs by status
10. ✅ Complete job (RUNNING → COMPLETED)
11. ✅ Delete job (cleanup)
12. ✅ Delete tenant (cleanup)

## Expected Output

```
🔌 Connecting to Cloud Spanner...
✅ Connected successfully!

--- Testing Tenant Operations ---
Creating tenant: [uuid]
✅ Tenant created
✅ Retrieved tenant: [uuid] - Test Company
✅ Total tenants: X

--- Testing Job Operations ---
Creating job: [uuid]
✅ Job created with status: PENDING
✅ Retrieved job: [uuid] - Status: PENDING

🔄 Updating job status to RUNNING...
✅ Status updated
✅ Current status: RUNNING
✅ Total jobs for tenant: 1
✅ Running jobs: 1

✅ Completing job...
✅ Job completed
✅ Final status: COMPLETED
✅ Completed at: 2026-02-10T12:34:56Z

--- Cleanup ---
✅ Test job deleted
✅ Test tenant deleted

🎉 All tests passed!
```

## Troubleshooting

### Error: "could not find default credentials"
```powershell
gcloud auth application-default login
```

### Error: "PERMISSION_DENIED"
Your account needs:
- `roles/spanner.databaseUser` or
- `roles/spanner.databaseAdmin`

Ask your admin to grant permissions.

### Error: "context deadline exceeded"
Check your internet connection and firewall settings.
