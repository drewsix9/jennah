# Frontend: Advanced Configuration for Create New Job

This document guides frontend development for the advanced configuration section in the **Create New Job** form.

---

## Current State

The existing "Create New Job" screen supports:

- Job name
- Container image URI
- Compute size (Small/Medium/Heavy/GPU presets)

---

## Recommended Form Sections (MVP)

### Priority 1: Configuration Section (High Priority - Core Features)

**Header**: "Configuration"

```
┌─────────────────────────────────────────────────────────┐
│ Configuration                                           │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Max Duration (Timeout)                                  │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Hours (Max 4)  │ Minutes    │ Seconds              │ │
│ │ [0         ]   │ [0      ]  │ [0                ] │ │
│ │ ℹ️ Set to 0 to use default timeout                 │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ Max Retries                                             │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ [3      ] (Default: 3, Max: 5)                      │ │
│ │ ℹ️ Recomm: 3. Max 5 for cost control               │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ Environment Variables                                   │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Key                  Value                          │ │
│ │ [DB_HOST          ] [mysql.example.com          ] │ │
│ │ [API_KEY          ] [••••••••••••••••••••••••  ] │ │
│ │ [REGION           ] [us-central1               ] │ │
│ │                                                  │ │
│ │ [+ Add variable]                                │ │
│ │ ⚠️ Sensitive values? Use GCP Secret Manager    │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Validation Rules**:

- Hours: 0-4 (integer)
- Minutes: 0-59 (integer)
- Seconds: 0-59 (integer)
- Max Retries: 1-5 (integer, default: 3)
- Environment Variables: Key must be alphanumeric + underscore, Value is string (no size limit)
- At least one timeout field must be > 0 OR timeout can be 0 (use default)

**Notes for Frontend**:

- When all timeout fields are 0, the worker uses default timeout from resource profile
- Show warning icon if timeout > 4 hours (GCP Batch max is longer, but our limit is configurable)
- Environment variables can be marked as "sensitive" to mask input

---

### Compute Resources: Choose Method (Mutually Exclusive)

**Header**: "Compute Resources"

```
┌─────────────────────────────────────────────────────────┐
│ Compute Resources                                       │
│ (Choose ONE configuration method)                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ ○ Quick Preset (Compute Size) [SELECTED BY DEFAULT]    │
│  └─ ┌─────────────────────────────────────────────────┐ │
│     │ ▼ Medium (4 vCPU, 16 GB) - Recommended        │ │
│     │  • Small (e2-micro)                            │ │
│     │  • Medium (e2-standard-4)                      │ │
│     │  • Heavy (n1-standard-16)                      │ │
│     │  • GPU (n1-standard-8 + GPU)                   │ │
│     └─────────────────────────────────────────────────┘ │
│                                                         │
│ ○ Custom Machine Type                                   │
│  └─ ┌─────────────────────────────────────────────────┐ │
│     │ ▼ Select machine type (disabled)               │ │
│     │  • e2-micro (2 vCPU, 1 GB) - Testing/light    │ │
│     │  • e2-standard-2 (2 vCPU, 8 GB)               │ │
│     │  • e2-standard-4 (4 vCPU, 16 GB)              │ │
│     │  • n1-standard-16 (16 vCPU, 60 GB)            │ │
│     │  • n1-standard-8 + GPU (8 vCPU, 30 GB + GPU)  │ │
│     └─────────────────────────────────────────────────┘ │
│                                                         │
│ ℹ️ Only ONE option can be active. Select the method   │
│    that works best for your workload.                  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Radio Button Behavior**:

- **Quick Preset** (default):
  - When selected: Enable Quick Preset dropdown, disable Custom Machine Type dropdown
  - Values: "small", "medium", "heavy", "gpu"
  - Non-technical users default to this

- **Custom Machine Type**:
  - When selected: Disable Quick Preset dropdown, enable Custom Machine Type dropdown
  - Values: "e2-micro", "e2-standard-2", "e2-standard-4", "n1-standard-16", "n1-standard-8+gpu"
  - Advanced users who need specific control

**Validation Rules**:

- Exactly ONE option must be selected (enforced by radio button)
- If "Quick Preset": must select from dropdown
- If "Custom Machine Type": must select from dropdown
- Only the selected option's value is sent to backend

**Notes for Frontend**:

- Both dropdowns should show machine specs (vCPU, Memory, cost estimate if available)
- Default to "Quick Preset" (easier for most users)
- Disable one dropdown when the other is selected
- Show visual indication of which is active (highlight, color change, etc.)

---

### Priority 2: Advanced Resources Section (Medium Priority - Optimization)

**Header**: "Advanced Resources"

```
┌─────────────────────────────────────────────────────────┐
│ Advanced Resources                                      │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Boot Disk Size (GB)                                     │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ [50    ] GB (Range: 10-100, Default: 50)          │ │
│ │ ℹ️ Allocated to container during execution        │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ☒ Use Spot VMs                                         │
│ └─ Save up to 60% on compute. ⚠️ VMs may be         │
│    interrupted with 25s notice. Only for fault-      │
│    tolerant workloads.                                │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Validation Rules**:

- Boot Disk Size: 10-100 GB (integer, default: 50)
- Use Spot VMs: Boolean toggle (default: false)

**Notes for Frontend**:

- Warn if Boot Disk Size < 20 GB ("may not have enough space for large images")
- Spot VMs checkbox: Add a link to GCP Spot documentation
- If use_spot_vms = true, show warning banner: "⚠️ Your job can be interrupted. Only use for fault-tolerant workloads."
- Disable Spot VMs if Machine Type includes GPU (not supported)

---

### Priority 3: Security & Logging Section (Low Priority - Optional)

**Header**: "Advanced Security & Logging"

```
┌─────────────────────────────────────────────────────────┐
│ Advanced Security & Logging                             │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Service Account (Optional)                              │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ ▼ Default Batch Service Account                   │ │
│ │  • Default Batch Service Account                  │ │
│ │  • Custom Service Account (if you have one)       │ │
│ └─────────────────────────────────────────────────────┘ │
│ ℹ️ Most jobs use the default. Requires admin access  │
│                                                         │
│ ☐ Stream logs to Cloud Logging                         │
│ └─ Enable to view detailed execution logs in GCP      │
│                                                         │
│ ℹ️ Jobs are always created in Cloud Batch.            │
│    View status and logs in GCP Console.                │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Validation Rules**:

- Service Account: Optional, string (email format if custom)
- Stream Logs: Boolean toggle (default: true)

**Notes for Frontend**:

- This section can be collapsed by default (low usage)
- Service Account dropdown should fetch from current GCP project (requires backend support)
- Add link to Cloud Logging documentation

---

## Form Section Ordering (Recommended Layout)

```
Create New Job
│
├─ [Basic Information]
│  ├─ Job Name *
│  └─ Container Image URI *
│
├─ [Compute Resources] ⬅️ MUTUALLY EXCLUSIVE - Choose ONE
│  │
│  └─ ○ Quick Preset (Compute Size) [SELECTED]
│      └─ [▼ Medium]
│            • Small (e2-micro)
│            • Medium (e2-standard-4) [selected]
│            • Heavy (n1-standard-16)
│            • GPU (n1-standard-8 + GPU)
│
│  └─ ○ Custom Machine Type
│      └─ [▼ e2-standard-4 (disabled until selected)]
│            • e2-micro (2 vCPU, 1 GB)
│            • e2-standard-2 (2 vCPU, 8 GB)
│            • e2-standard-4 (4 vCPU, 16 GB)
│            • n1-standard-16 (16 vCPU, 60 GB)
│            • n1-standard-8 + GPU (8 vCPU, 30 GB + GPU)
│
├─ [Configuration] ⬅️ Priority 1 (EXPANDED by default)
│  ├─ Max Duration (Timeout)
│  ├─ Max Retries
│  └─ Environment Variables
│
├─ [Advanced Resources] ⬅️ Priority 2 (COLLAPSED by default)
│  ├─ Boot Disk Size
│  └─ Use Spot VMs
│
├─ [Advanced Security & Logging] ⬅️ Priority 3 (COLLAPSED)
│  ├─ Service Account
│  └─ Stream logs to Cloud Logging
│
└─ [Action Buttons]
   ├─ [Cancel]
   └─ [Submit Job]
```

**Key Difference**: Compute resources are now **mutually exclusive**.

- User selects either "Quick Preset" OR "Custom Machine Type" (not both)
- Only one radio button can be active at a time
- Selecting one disables the other
- This eliminates confusion about which setting takes precedence

---

## Data Flow: Form → Proto → Backend

### Frontend Collects

```json
{
  "jobName": "my-etl-job",
  "imageUri": "gcr.io/my-project/etl:v1.0",
  "computeResourceMethod": "quick-preset",
  "computePreset": "medium",
  "customMachineType": null,
  "timeoutHours": 1,
  "timeoutMinutes": 30,
  "timeoutSeconds": 0,
  "maxRetries": 3,
  "envVars": {
    "DB_HOST": "mysql.example.com",
    "API_KEY": "secret-value",
    "REGION": "us-central1"
  },
  "bootDiskSizeGb": 50,
  "useSpotVms": false,
  "serviceAccount": null,
  "streamLogs": true
}
```

### Frontend Resolves & Sends (POST /jennah.v1.DeploymentService/SubmitJob)

The frontend **resolves** the mutually exclusive selection into a single `machine_type` value:

```typescript
// Frontend logic - only ONE of these paths is taken
let machineType;

if (form.computeResourceMethod === "quick-preset") {
  // Path 1: Quick Preset → Map to machine type
  machineType = mapPresetToMachineType(form.computePreset);
  // "medium" → "e2-standard-4"
} else {
  // Path 2: Custom Machine Type → Use directly
  machineType = form.customMachineType;
  // "e2-standard-4" → "e2-standard-4"
}
```

**Protobuf payload sent to backend:**

```protobuf
SubmitJobRequest {
  string image_uri = "gcr.io/my-project/etl:v1.0";
  map<string, string> env_vars = {
    "DB_HOST": "mysql.example.com",
    "API_KEY": "secret-value",
    "REGION": "us-central1"
  };
  int32 max_retries = 3;
  google.protobuf.Duration timeout = {
    seconds: 5400  // 1.5 hours in seconds
  };
  string machine_type = "e2-standard-4";  // ← Resolved single value
  int32 boot_disk_size_gb = 50;
  bool use_spot_vms = false;
  string service_account = "";  // Optional
}
```

### Backend Returns

```protobuf
SubmitJobResponse {
  string job_id = "550e8400-e29b-41d4-a716-446655440000";
  string gcp_batch_job_name = "projects/labs-169405/locations/asia-northeast1/jobs/jennah-550e8400";
  string status = "PENDING";
}
```

---

## Validation Rules Summary

| Field                   | Type     | Default | Validation                                  | Error Message                  |
| ----------------------- | -------- | ------- | ------------------------------------------- | ------------------------------ |
| **Timeout Hours**       | int      | 0       | 0-4                                         | "Max 4 hours"                  |
| **Timeout Minutes**     | int      | 0       | 0-59                                        | "Must be 0-59"                 |
| **Timeout Seconds**     | int      | 0       | 0-59                                        | "Must be 0-59"                 |
| **Max Retries**         | int      | 3       | 1-5                                         | "Must be 1-5"                  |
| **Env Var Key**         | string   | —       | alphanumeric + `_`                          | "Invalid key format"           |
| **Env Var Value**       | string   | —       | any string                                  | —                              |
| **Quick Preset**        | dropdown | medium  | must select if method="quick-preset"        | "Please select a preset"       |
| **Custom Machine Type** | dropdown | —       | must select if method="custom-machine-type" | "Please select a machine type" |
| **Boot Disk (GB)**      | int      | 50      | 10-100                                      | "Must be 10-100 GB"            |
| **Use Spot VMs**        | bool     | false   | —                                           | —                              |
| **Service Account**     | string   | empty   | email format (optional)                     | "Invalid email"                |

---

## Handling Compute Resources Selection (Mutually Exclusive Radio Buttons)

### Problem Solved

The mutually exclusive radio button approach **eliminates ambiguity** when users configure compute resources:

- ❌ **Before**: Users could select both "Medium" preset AND custom "n1-standard-16", causing confusion about which takes precedence
- ✅ **After**: Users must choose ONE method. Clear, unambiguous, and simple.

### Implementation Guide

#### Frontend State Management

```typescript
// Form state should track only the active method
interface ComputeResourcesForm {
  method: "quick-preset" | "custom-machine-type";
  quickPreset?: "small" | "medium" | "heavy" | "gpu";
  customMachineType?:
    | "e2-micro"
    | "e2-standard-2"
    | "e2-standard-4"
    | "n1-standard-16"
    | "n1-standard-8+gpu";
}

// Initialize with default
const defaultComputeResources: ComputeResourcesForm = {
  method: "quick-preset",
  quickPreset: "medium", // Default to Medium
};
```

#### Radio Button Selection Handler

```typescript
// When user clicks radio button
const handleComputeMethodChange = (
  method: "quick-preset" | "custom-machine-type",
) => {
  setForm((prev) => ({
    ...prev,
    computeResources: {
      method,
      // Keep the previously selected value for that method (optional)
      // Or clear it (safest approach shown below)
      quickPreset:
        method === "quick-preset"
          ? prev.computeResources.quickPreset
          : undefined,
      customMachineType:
        method === "custom-machine-type"
          ? prev.computeResources.customMachineType
          : undefined,
    },
  }));
};

// When user selects from Quick Preset dropdown
const handleQuickPresetChange = (preset: string) => {
  setForm((prev) => ({
    ...prev,
    computeResources: {
      ...prev.computeResources,
      quickPreset: preset,
    },
  }));
};

// When user selects from Custom Machine Type dropdown
const handleCustomMachineTypeChange = (machineType: string) => {
  setForm((prev) => ({
    ...prev,
    computeResources: {
      ...prev.computeResources,
      customMachineType: machineType,
    },
  }));
};
```

#### Resolving to Machine Type Before Submission

```typescript
// Helper function to resolve the selected radio button choice to a machine type
const resolveMachineType = (computeResources: ComputeResourcesForm): string => {
  if (computeResources.method === "quick-preset") {
    const presetMap: Record<string, string> = {
      small: "e2-micro",
      medium: "e2-standard-4",
      heavy: "n1-standard-16",
      gpu: "n1-standard-8+gpu",
    };
    return presetMap[computeResources.quickPreset!];
  } else {
    return computeResources.customMachineType!;
  }
};

// During form submission - only send ONE machine_type value
const submitJob = async (form: JobForm) => {
  // Validate before proceeding
  if (!validateComputeResources(form.computeResources)) {
    showError("Please select a compute resource configuration");
    return;
  }

  const machineType = resolveMachineType(form.computeResources);

  const payload = {
    image_uri: form.imageUri,
    machine_type: machineType, // ← Only this is sent (resolved single value)
    max_retries: form.maxRetries,
    timeout: {
      seconds: calculateTimeoutSeconds(form.timeout),
    },
    env_vars: form.envVars,
    boot_disk_size_gb: form.bootDiskSizeGb,
    use_spot_vms: form.useSpotVms,
    service_account: form.serviceAccount || "",
  };

  return await submitJobAPI(payload);
};
```

#### Validation Before Submit

```typescript
const validateComputeResources = (
  computeResources: ComputeResourcesForm,
): boolean => {
  if (computeResources.method === "quick-preset") {
    return !!computeResources.quickPreset; // Must have selected a preset
  } else if (computeResources.method === "custom-machine-type") {
    return !!computeResources.customMachineType; // Must have selected a machine type
  }
  return false; // Method not set
};
```

#### UI Behavior & Visual Indicators

```
Initial State:
  ◉ Quick Preset (selected, filled circle)
  └─ [▼ Medium] (enabled, text in black)

  ○ Custom Machine Type (not selected, empty circle)
  └─ [▼ Select...] (disabled, text in gray)

After clicking Custom Machine Type radio:
  ○ Quick Preset (not selected, empty circle)
  └─ [▼ Medium] (disabled, text in gray)

  ◉ Custom Machine Type (selected, filled circle)
  └─ [▼ Select...] (enabled, text in black)

After selecting "n1-standard-16" from Custom dropdown:
  ○ Quick Preset (not selected, empty circle)
  └─ [▼ Medium] (disabled, text in gray)

  ◉ Custom Machine Type (selected, filled circle)
  └─ [▼ n1-standard-16 (16 vCPU, 60 GB)] (enabled, text in black)
```

#### Dropdown State Management

```typescript
// Determine if each dropdown should be enabled
const isQuickPresetEnabled = form.computeResources.method === "quick-preset";
const isCustomMachineTypeEnabled = form.computeResources.method === "custom-machine-type";

// In JSX/React:
<select
  disabled={!isQuickPresetEnabled}
  value={form.computeResources.quickPreset || ""}
  onChange={(e) => handleQuickPresetChange(e.target.value)}
  style={{ color: isQuickPresetEnabled ? "black" : "gray" }}
>
  <option value="">Select preset...</option>
  <option value="small">Small (e2-micro)</option>
  <option value="medium">Medium (e2-standard-4)</option>
  <option value="heavy">Heavy (n1-standard-16)</option>
  <option value="gpu">GPU (n1-standard-8 + GPU)</option>
</select>

<select
  disabled={!isCustomMachineTypeEnabled}
  value={form.computeResources.customMachineType || ""}
  onChange={(e) => handleCustomMachineTypeChange(e.target.value)}
  style={{ color: isCustomMachineTypeEnabled ? "black" : "gray" }}
>
  <option value="">Select machine type...</option>
  <option value="e2-micro">e2-micro (2 vCPU, 1 GB)</option>
  <option value="e2-standard-2">e2-standard-2 (2 vCPU, 8 GB)</option>
  <option value="e2-standard-4">e2-standard-4 (4 vCPU, 16 GB)</option>
  <option value="n1-standard-16">n1-standard-16 (16 vCPU, 60 GB)</option>
  <option value="n1-standard-8+gpu">n1-standard-8 + GPU (8 vCPU, 30 GB + GPU)</option>
</select>
```

### Benefits of This Approach

✅ **No Ambiguity**: Only ONE source of truth for machine type  
✅ **Clear UX**: Users understand only one will be used  
✅ **Simple Logic**: Frontend clearly resolves which path is taken  
✅ **Easy Validation**: Validate only the active method  
✅ **Scalable**: Easy to add more compute options in future

---

## UI/UX Checklist

- [x] **Mutually Exclusive Selection**: Only ONE radio button can be active at a time
  - Selecting "Quick Preset" disables "Custom Machine Type" dropdown
  - Selecting "Custom Machine Type" disables "Quick Preset" dropdown
- [x] **Default State**: "Quick Preset" is selected by default (easier for non-technical users)
- [x] **Visual Feedback**: Show which option is currently active (highlight, color, boldface)
- [ ] Collapse Priority 2 & 3 sections by default (Configuration is expanded)
- [ ] Show field hints (ℹ️ icons) for non-obvious fields
- [ ] Disable Spot VM option if Machine Type is GPU
- [ ] Show warning if Boot Disk < 20 GB
- [ ] Validate timeout fields are numeric only
- [ ] Mask sensitive Environment Variable values (show as •••)
- [ ] Add "Copy to clipboard" button for job ID in success response
- [ ] Show estimated cost next to Machine Type selection
- [ ] Add tooltip explaining: "Quick Preset = Easy presets. Custom Machine Type = Full control. Choose ONE."

---

## Testing Scenarios

### Scenario 1: Minimal Configuration (Use Defaults)

- Job Name: "my-first-job"
- Image URI: "gcr.io/my-project/app:latest"
- Compute Resources: "Quick Preset" → "Medium" (default, pre-selected)
- All other settings: defaults
- Expected: Job submits successfully with e2-standard-4 machine type

### Scenario 2: Switch to Custom Machine Type

- Job Name: "heavy-processing-job"
- Image URI: "gcr.io/my-project/heavy:latest"
- Compute Resources: Click "Custom Machine Type" radio → Select "n1-standard-16"
- Quick Preset dropdown becomes disabled (grayed out)
- Expected: Job uses n1-standard-16, not any preset

### Scenario 3: Custom Timeout & Retries with Preset

- Compute Resources: "Quick Preset" → "Small"
- Max Duration: 2 hours, 30 minutes, 0 seconds
- Max Retries: 5
- Environment Variables: Add DB_HOST, DB_PASSWORD
- Expected: Job respects timeout and retry count; uses e2-micro

### Scenario 4: GPU Job with Custom Settings

- Compute Resources: "Custom Machine Type" → "n1-standard-8 + GPU"
- Boot Disk Size: 100 GB
- Use Spot VMs: ☐ (unchecked - Spot VMs should be disabled/unavailable for GPU)
- Expected: Job request includes GPU allocation

### Scenario 5: Cost Optimization with Spot VMs

- Compute Resources: "Custom Machine Type" → "n1-standard-16"
- Use Spot VMs: ☑ (checked)
- Expected: Warning shown: "⚠️ Your job can be interrupted. Only use for fault-tolerant workloads."

### Scenario 6: Switching Methods (Radio Button Toggle)

1. Start with "Quick Preset" → "Medium" selected
2. User realizes they need more control
3. Click "Custom Machine Type" radio button
4. Quick Preset dropdown becomes disabled
5. Select "e2-standard-2" from Custom Machine Type
6. Submit Job

- Expected: Job uses e2-standard-2 (the custom selection), not Medium

### Scenario 7: Complex Environment Configuration

- Compute Resources: "Quick Preset" → "Heavy"
- Max Duration: 4 hours
- Max Retries: 3
- Environment Variables:
  - DB_HOST: mysql.company.com
  - DB_USER: admin
  - API_KEY: [••••••] (sensitive, masked)
  - LOG_LEVEL: DEBUG
  - REGION: us-central1
- Expected: All 5 env vars passed to container; uses n1-standard-16

### Scenario 8: Validation Error - No Machine Type Selected

- (Theoretical) If radio button state is lost/invalid
- Click [Submit Job] without proper selection
- Expected: Error message: "Please select a compute resource configuration"

---

## Key Changes: Mutually Exclusive vs. Override Approach

### What Changed?

This document now **recommends Option 2: Mutually Exclusive Radio Buttons** instead of Option 1: Machine Type Overrides.

| Aspect                | Option 1 (Override) ❌           | Option 2 (Radio Buttons) ✅      |
| --------------------- | -------------------------------- | -------------------------------- |
| **When both are set** | Machine Type overrides Preset    | Only ONE can be active           |
| **User Experience**   | Confusing - which wins?          | Crystal clear - one or the other |
| **UI Complexity**     | Simple form layout               | Radio button group shows intent  |
| **Error Potential**   | User accidentally sets both      | User can only set one            |
| **Frontend Logic**    | Check if machineType !== default | Check which radio is selected    |
| **Validation**        | Ambiguous selection logic        | Explicit state machine           |
| **Best For**          | Power users only                 | All user types                   |

### Migration from Option 1 → Option 2

If you started with Option 1, here's what changed:

**Before (Option 1)**:

```json
{
  "computePreset": "medium",
  "machineType": "e2-standard-4" // This would win if set
  // Ambiguous: which one is being used?
}
```

**Now (Option 2)**:

```json
{
  "computeResourceMethod": "quick-preset", // ← Explicit choice
  "computePreset": "medium",
  "customMachineType": null // Not used when method=quick-preset
  // Clear: Quick Preset is active
}
```

### Why This Is Better

1. **No Ambiguity**: Frontend resolves BEFORE sending to API
2. **Clearer Intent**: Radio button shows exactly what user chose
3. **Simpler Backend**: Worker always gets single resolved `machine_type`
4. **Better Validation**: Easy to validate only active option
5. **Easier Testing**: Clear state paths (preset or custom, never both)

---

## Backend Integration Notes

### Proto File Location

- [proto/jennah.proto](proto/jennah.proto) - defines SubmitJobRequest/Response

### After Proto Changes

```bash
buf generate  # Regenerates gen/proto/ with new fields
```

### RPC Endpoint

```
POST http://gateway-ip:8080/jennah.v1.DeploymentService/SubmitJob
```

### Required Headers

```
Content-Type: application/json
X-OAuth-Email: user@example.com
X-OAuth-UserId: 123456
X-OAuth-Provider: google
```

---

## Future Enhancements

- [ ] Save job configurations as templates
- [ ] Pre-populate form from previous job
- [ ] Cost calculator: show estimated cost for selected resources
- [ ] Network configuration (VPC, firewall rules)
- [ ] Custom machine type builder (select vCPU/Memory separately)
- [ ] Workload identity federation for service account
- [ ] Secret Manager integration for sensitive env vars

---

## References

- [GCP Batch Job Configuration](docs/gcp-batch-sdk-guide.md)
- [Worker Implementation](cmd/worker/service.go)
- [Proto Definition](proto/jennah.proto)
- [Database Schema](database/schema.sql)
