package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	// context that cancels when Ctrl + C are pressed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capture OS signals
	sigChan := make(chan os.Signal, 16)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("Shutting down all services...")
		cancel()
	}()

	go runCommand(ctx, "MockBackend-9001", "go", []string{"run", "main.go", ":9001"}, "./mockbackend")

	go runCommand(ctx, "MockBackend-9002", "go", []string{"run", "main.go", ":9002"}, "./mockbackend")

	go runCommand(ctx, "Gateway-8080", "go", []string{"run", "main.go", "proxy.go", "router.go", "ratelimiter.go", "circuitbreaker.go"}, "./gateway")

	<-ctx.Done()
	fmt.Println("All services stopped successfully.")
}

func runCommand(ctx context.Context, name, command string, args []string, dir string) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Starting %s in %s...\n", name, dir)
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		log.Printf("%s crashed: %v", name, err)
	}
}
