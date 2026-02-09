# Application overview

The S7-1500V Statistics & Visualization application is designed to collect, store, and visualize statistics from SIMATIC S7-1500V (Virtual PLC) installations using a modern monitoring stack with real-time dashboard capabilities.

- [Application overview](#application-overview)
  - [General Task](#general-task)
  - [Application Architecture](#application-architecture)
  - [Collector Service](#collector-service)
  - [Prometheus Service](#prometheus-service)
  - [Plutono Service](#plutono-service)

## General Task

The S7-1500V Statistics & Visualization application collects statistics from S7-1500V installations via API calls and provides comprehensive monitoring capabilities. The system uses a Go-based collector service to gather metrics from S7-1500V endpoints, stores them in a Prometheus time-series database, and visualizes the data through Plutono dashboards. The entire system runs in a containerized Docker environment for easy deployment and scalability.

![overview](graphics/dashboard.png)

## Application Architecture

The S7-1500V Statistics & Visualization application follows a microservice architecture pattern. The application consists of three independent components, each performing specialized tasks in the monitoring pipeline.

The system architecture includes:
- **Collector Service**: Gathers statistics via S7-1500V API endpoints
- **Prometheus**: Time-series database for metric storage and querying
- **Plutono**: Web-based visualization and dashboarding platform

## Collector Service

The "Collector Service" is implemented in Go and handles all communication with S7-1500V installations. The service:

- Connects to S7-1500V endpoints using configured credentials
- Collects system statistics and performance metrics via API calls
- Processes and formats the collected data for Prometheus consumption
- Exposes a `/metrics` endpoint in Prometheus format for scraping
- Supports multiple S7-1500V installations simultaneously
- Implements error handling and retry mechanisms for robust data collection

The collector service runs continuously and updates metrics at regular intervals, ensuring real-time monitoring capabilities.

## Prometheus Service

The "Prometheus Service" implements an open-source time-series database specifically designed for monitoring and alerting. Key features include:

- **Time-Series Storage**: Efficiently stores metrics with timestamps for historical analysis
- **Scraping Mechanism**: Automatically collects metrics from the collector service's `/metrics` endpoint
- **Query Language**: Provides PromQL for complex data queries and aggregations
- **Data Retention**: Configurable retention policies for managing storage requirements
- **High Availability**: Supports clustering and replication for production environments

Prometheus serves as the central data store for all S7-1500V metrics and provides the foundation for visualization and alerting.

## Plutono Service

The "Plutono Service" is a Grafana fork with a more permissive license, providing comprehensive visualization capabilities:

- **Dashboard Creation**: Pre-configured dashboards for S7-1500V monitoring
- **Multi-Instance Support**: Dropdown selection for monitoring different S7-1500V installations
- **Real-Time Visualization**: Live updating charts and graphs
- **Historical Analysis**: Time-range selection for historical data review
- **Alert Management**: Configurable alerts based on metric thresholds
- **User Management**: Authentication and authorization capabilities

The service provides an intuitive web interface accessible at `http://localhost:3000` for comprehensive S7-1500V monitoring and analysis.