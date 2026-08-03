// Package main provides a tool for generating audit encryption keys
package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		envVar    = flag.String("env", "AUDIT_ENCRYPTION_KEY", "Environment variable name for the key")
		showUsage = flag.Bool("h", false, "Show help message")
	)
	flag.Parse()

	if *showUsage {
		printUsage()
		return
	}

	// Generate 32-byte key for AES-256
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
		os.Exit(1)
	}

	// Encode to base64
	keyB64 := base64.StdEncoding.EncodeToString(key)

	// Output
	fmt.Println("# Audit Encryption Key Generated")
	fmt.Println("#")
	fmt.Println("# IMPORTANT: Store this key securely! It cannot be recovered if lost.")
	fmt.Println("#")
	fmt.Printf("# Configuration:\n")
	fmt.Printf("#   audit:\n")
	fmt.Printf("#     encryption:\n")
	fmt.Printf("#       enabled: true\n")
	fmt.Printf("#       algorithm: aes-256-gcm\n")
	fmt.Printf("#       key_env: %s\n", *envVar)
	fmt.Println("#")
	fmt.Printf("# Shell:\n")
	fmt.Printf("export %s="%s"\n", *envVar, keyB64)
	fmt.Println("#")
	fmt.Printf("# PowerShell:\n")
	fmt.Printf("$env:%s="%s"\n", *envVar, keyB64)
	fmt.Println("#")
	fmt.Printf("# Docker Compose (docker-compose.yml):\n")
	fmt.Printf("services:\n")
	fmt.Printf("  gateway:\n")
	fmt.Printf("    environment:\n")
	fmt.Printf("      - %s="%s"\n", *envVar, keyB64)
}

func printUsage() {
	fmt.Println("audit-keygen - Generate AES-256 encryption key for audit logs")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  audit-keygen [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -env <name>   Environment variable name (default: AUDIT_ENCRYPTION_KEY)")
	fmt.Println("  -h            Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Generate key with default env var name")
	fmt.Println("  audit-keygen")
	fmt.Println()
	fmt.Println("  # Generate key with custom env var name")
	fmt.Println("  audit-keygen -env MY_AUDIT_KEY")
}