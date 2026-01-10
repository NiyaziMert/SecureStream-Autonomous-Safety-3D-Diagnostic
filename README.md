# Concurrent Log Streamer

This project was developed to demonstrate how to build a high-performance data processing and real-time monitoring system using advanced features of the Go language (Go Routines, Channels, WebSockets).

## Key Features
- **Backend (Go):** - Concurrent data generation and processing.
- Worker pool architecture with Go Routines.
- Type-safe channel management.
- Real-time data streaming via WebSocket.
- **Frontend (React):** - Reactive visualization of real-time data from the backend.

## Technologies
- **Language:** Go (Golang)
- **Concurrency:** Goroutines, Channels, Select, Mutex
- **API/Communication:** Gorilla WebSocket, Standard Net/HTTP
- **Frontend:** React, Vite, TailwindCSS

## Purpose
The purpose of the system is to process log and metric data generated intensively in the background without blocking the main thread and to display it to the user interface with the lowest possible latency.
