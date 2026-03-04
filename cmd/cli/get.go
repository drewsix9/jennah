package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <job-id>",
	Short: "Get job details",
	Long:  "jennah get <job-id> [--output json]\n\nFetches and displays full details of a specific job by ID.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]
		outputFmt, _ := cmd.Flags().GetString("output")

		gw, err := newGatewayClient(cmd)
		if err != nil {
			return err
		}

		jobs, err := fetchJobs(gw)
		if err != nil {
			return fmt.Errorf("failed to fetch jobs: %w", err)
		}

		j := findJob(jobs, jobID)
		if j == nil {
			return fmt.Errorf("job %q not found", jobID)
		}

		if outputFmt == "json" {
			printJobsJSON([]Job{*j})
			return nil
		}

		pht, _ := time.LoadLocation("Asia/Manila")
		fmtTime := func(raw string) string {
			if raw == "" {
				return "-"
			}
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				return t.In(pht).Format("2006-01-02 15:04:05 PHT")
			}
			return raw
		}

		fmt.Println("Job Details")
		fmt.Println("───────────────────────────────────────")

		// Identity
		fmt.Printf("Job ID:          %s\n", j.JobID)
		if j.Name != "" {
			fmt.Printf("Name:            %s\n", j.Name)
		}
		fmt.Printf("Tenant:          %s\n", j.TenantID)

		// Status
		fmt.Println()
		fmt.Printf("Status:          %s\n", j.Status)
		if j.ErrorMessage != "" {
			fmt.Printf("Error:           %s\n", j.ErrorMessage)
		}
		if j.RetryCount.String() != "0" && j.RetryCount.String() != "" {
			fmt.Printf("Retries:         %s / %s\n", j.RetryCount, j.MaxRetries)
		}

		// Timestamps
		fmt.Println()
		fmt.Printf("Created:         %s\n", fmtTime(j.CreatedAt))
		fmt.Printf("Updated:         %s\n", fmtTime(j.UpdatedAt))
		if j.ScheduledAt != "" {
			fmt.Printf("Scheduled:       %s\n", fmtTime(j.ScheduledAt))
		}
		if j.StartedAt != "" {
			fmt.Printf("Started:         %s\n", fmtTime(j.StartedAt))
		}
		if j.CompletedAt != "" {
			fmt.Printf("Completed:       %s\n", fmtTime(j.CompletedAt))
		}

		// Container
		fmt.Println()
		fmt.Printf("Image:           %s\n", j.ImageURI)
		if len(j.Commands) > 0 {
			fmt.Printf("Commands:        %s\n", strings.Join(j.Commands, " "))
		}
		if j.EnvVarsJson != "" && j.EnvVarsJson != "{}" && j.EnvVarsJson != "null" {
			var envMap map[string]string
			if json.Unmarshal([]byte(j.EnvVarsJson), &envMap) == nil {
				fmt.Printf("Env Vars:\n")
				for k, v := range envMap {
					fmt.Printf("  %s=%s\n", k, v)
				}
			}
		}

		// Resources
		fmt.Println()
		if j.ResourceProfile != "" {
			fmt.Printf("Profile:         %s\n", j.ResourceProfile)
		}
		if j.MachineType != "" {
			fmt.Printf("Machine Type:    %s\n", j.MachineType)
		}
		if j.BootDiskSizeGb.String() != "0" && j.BootDiskSizeGb.String() != "" {
			fmt.Printf("Boot Disk:       %s GB\n", j.BootDiskSizeGb)
		}
		if j.UseSpotVms {
			fmt.Printf("Spot VMs:        yes\n")
		}
		if j.ServiceAccount != "" {
			fmt.Printf("Service Account: %s\n", j.ServiceAccount)
		}

		// GCP resource path
		if j.GcpBatchJobPath != "" {
			fmt.Println()
			fmt.Printf("GCP Job Path:    %s\n", j.GcpBatchJobPath)
		}

		return nil
	},
}

func init() {
	getCmd.Flags().String("output", "", "Output format: json")
}
