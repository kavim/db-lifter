DB-Lift: High-Performance MySQL Docker Restore CLI
Context & Role
Act as a 10x Senior Software Engineer specializing in Backend, Systems Programming, and Infrastructure. Your goal is to build a robust, high-performance CLI tool in Go called db-lift.

The Problem
Restoring large MySQL dumps (multi-GB) into Docker containers is often slow and memory-intensive. We need a tool that replaces a local database with a SQL dump using maximum I/O efficiency and modern terminal feedback.

Core Technical Requirements

1. Zero-Copy Streaming (Pipes)
   Implementation: Use os.Open to read the dump file and connect the resulting io.Reader directly to the StdinPipe() of the exec.Command("docker", "exec", "-i", ...) process.

Constraint: Do not load the file into memory strings or large byte buffers. Stream the data in chunks.

2. Optimized MySQL Command Chain
   Execute a composite sh -c command inside the container to wrap the restoration:

SET FOREIGN_KEY_CHECKS=0; SET UNIQUE_CHECKS=0;

(Stream the Pipe Content here)

SET FOREIGN_KEY_CHECKS=1; SET UNIQUE_CHECKS=1;

Ensure the byte stream is injected at the correct point between these SQL commands.

3. Real-time Progress Monitoring
   Wrap the io.Reader with a custom ProgressReader or use io.TeeReader to track bytes transferred.

Calculate progress based on the original file's FileInfo.Size().

4. Modern TUI (Terminal User Interface)
   Use the Charmbracelet/Bubbletea framework for the UI.

Features: \* A Spinner during the DROP/CREATE DATABASE phase.

A high-refresh Progress Bar during the data streaming phase.

Clear success/error states with distinct colors (Lipgloss).

5. Concurrency & Resilience
   Use Goroutines to handle I/O and UI updates concurrently to prevent interface lag.

Implement Context with Timeout/Cancel to handle hung processes.

Graceful Shutdown: Capture system signals (SIGINT, SIGTERM) to close pipes and terminate the Docker process safely.

Project Structure
Follow standard Go project layout (/cmd/db-lift for the entry point, /internal for core logic).

Provide a Makefile for static binary compilation.

Support configuration via CLI Flags (spf13/cobra) or a .env file.
