package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"bob/internal/agent"
	"bob/internal/audit"
	"bob/internal/coding"
	"bob/internal/computer"
	"bob/internal/config"
	"bob/internal/llm"
	"bob/internal/security"
	"bob/internal/server"
	"bob/internal/sessions"
	"bob/internal/tools/filesystem"
	"bob/internal/tools/registry"
	"bob/internal/tools/screenshot"
	"bob/internal/tools/terminal"
)

const Version = "0.1.0"

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	sysInfoFlag := flag.Bool("sysinfo", false, "Print detected hardware and model recommendation, then exit")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Bob Agent version %s\n", Version)
		return
	}

	if *sysInfoFlag {
		printSysInfo()
		return
	}

	// 1. Load Configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Printf("[WARN] Error loading config (%v), falling back to defaults", err)
		cfg = config.DefaultConfig()
	}

	// 2. Initialize Security
	redactor := security.NewRedactor(cfg.Server.APIToken)
	pathValidator := security.NewPathValidator(cfg.Filesystem.AllowedPaths)
	commandPolicy := security.NewCommandPolicy(
		cfg.Security.AllowedCommands,
		cfg.Security.ApprovalRequiredCommands,
		cfg.Security.BlockedCommands,
	)

	// 3. Initialize Audit Logger
	auditLogger, err := audit.NewLogger(cfg.Storage.AuditLogPath, redactor)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize audit logger: %v", err)
	}
	defer auditLogger.Close()

	// 4. Initialize Session Manager
	sessionMgr := sessions.NewManager(cfg.Storage.DataDir)

	// 5. Initialize Tool Registry
	toolReg := registry.NewRegistry()

	currentDir, _ := os.Getwd()
	termTool := terminal.NewTerminalTool(
		commandPolicy,
		pathValidator,
		currentDir,
		time.Duration(cfg.Agent.TimeoutSeconds)*time.Second,
		cfg.Filesystem.MaxOutputBytes,
	)
	_ = toolReg.Register(termTool)

	_ = toolReg.Register(filesystem.NewReadFileTool(pathValidator, cfg.Filesystem.MaxFileSizeRead))
	_ = toolReg.Register(filesystem.NewWriteFileTool(pathValidator))
	_ = toolReg.Register(filesystem.NewListDirectoryTool(pathValidator))
	_ = toolReg.Register(filesystem.NewSearchFilesTool(pathValidator))
	_ = toolReg.Register(filesystem.NewFindProjectTool(pathValidator))
	_ = toolReg.Register(screenshot.NewScreenshotTool(cfg.Storage.ScreenshotsDir))

	// 6. Initialize Computer Abstraction
	_ = computer.NewMacOSComputer(cfg.Storage.ScreenshotsDir)

	// 7. Initialize Coding Agent (AntiGravity integration placeholder)
	codingAgent := coding.NewAntiGravityCodingAgent(true)

	// 8. Initialize LLM Provider
	var llmClient llm.LLM
	switch cfg.LLM.Provider {
	case "mock":
		llmClient = llm.NewMockLLM()
	default:
		llmClient = llm.NewOllamaLLM(
			cfg.LLM.BaseURL,
			cfg.LLM.Model,
			time.Duration(cfg.LLM.TimeoutSec)*time.Second,
		)
	}

	// 9. Initialize Agent
	agentCore := agent.NewAgent(
		llmClient,
		toolReg,
		commandPolicy,
		auditLogger,
		sessionMgr,
		agent.AgentConfig{
			MaxSteps:       cfg.Agent.MaxSteps,
			MaxToolCalls:   cfg.Agent.MaxToolCalls,
			TimeoutSeconds: cfg.Agent.TimeoutSeconds,
		},
	)

	// Determine Web directory
	webDir := filepath.Join(currentDir, "web", "dist")
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		webDir = "" // Use embedded fallback UI
	}

	// 10. Initialize HTTP/WS Server
	srv := server.NewServer(cfg, agentCore, sessionMgr, auditLogger, codingAgent, webDir)

	// Detect system & Tailscale details for banner
	sysInfo := config.DetectSystemInfo()

	fmt.Println("=====================================================")
	fmt.Println("             🤖 BOB — Local AI Computer Agent        ")
	fmt.Printf("             Version: %s | Platform: %s/%s\n", Version, sysInfo.OS, sysInfo.Architecture)
	fmt.Println("=====================================================")
	fmt.Printf("• Apple Silicon: %s\n", sysInfo.CPUModel)
	fmt.Printf("• System RAM:    %s\n", sysInfo.RAMFormatted)
	fmt.Printf("• LLM Provider:  %s (%s)\n", cfg.LLM.Provider, cfg.LLM.Model)
	fmt.Printf("• Local URL:     http://localhost:%d\n", cfg.Server.Port)
	if sysInfo.TailscaleIP != "" {
		fmt.Printf("• Tailscale URL: http://%s:%d\n", sysInfo.TailscaleIP, cfg.Server.Port)
	} else {
		fmt.Println("• Tailscale:     (Tailscale daemon not connected or inactive)")
	}
	fmt.Printf("• Audit Log:     %s\n", cfg.Storage.AuditLogPath)
	fmt.Println("=====================================================")

	// Graceful shutdown handling
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("[FATAL] Server error: %v", err)
		}
	}()

	<-stopCh
	fmt.Println("\n[INFO] Shutting down Bob cleanly...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[WARN] Server shutdown error: %v", err)
	}

	fmt.Println("[INFO] Bob stopped.")
}

func printSysInfo() {
	info := config.DetectSystemInfo()
	fmt.Println("=== Mac System & AI Hardware Detection ===")
	fmt.Printf("OS:             macOS %s (%s)\n", info.MacOSVersion, info.Architecture)
	fmt.Printf("Apple Silicon:  %s\n", info.CPUModel)
	fmt.Printf("Memory (RAM):   %s\n", info.RAMFormatted)
	fmt.Printf("Disk Available: %s\n", info.AvailableDisk)
	fmt.Printf("Ollama Service: %v\n", info.OllamaRunning)
	fmt.Printf("Tailscale IP:   %s\n", info.TailscaleIP)
	fmt.Println("\nRecommended Local Models for this hardware:")
	for _, m := range info.RecommendedLLM {
		fmt.Printf(" - %s\n", m)
	}
}
