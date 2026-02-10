package main

import (
	"fmt"

	"jennah/internal/hashing"
)

func main() {
	workerIPs := []string{
		"10.128.0.1",
		"10.128.0.2",
		"10.128.0.3",
	}

	fmt.Println("--- Initializing Hash Ring ---")

	router := hashing.NewRouter(workerIPs)

	tenants := []string{
		"tenant-apple",
		"tenant-google",
		"tenant-facebook",
		"tenant-netflix",
		"tenant-amazon",
		"tenant-apple",
	}

	fmt.Println("\n--- Routing Results ---")
	for _, t := range tenants {
		assignedIP := router.GetWorkerIP(t)
		fmt.Printf("Tenant [%-15s] -> routed to -> %s\n", t, assignedIP)
	}
}
